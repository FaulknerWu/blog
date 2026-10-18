package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"blog/api/internal/auth"
	"blog/api/internal/content"
	"blog/api/internal/feed"
	"blog/api/internal/httpx"
	"blog/api/internal/media"
	"blog/api/internal/page"
	"blog/api/internal/post"
	"blog/api/internal/render"
	"blog/api/internal/settings"
	"blog/api/internal/store"
)

const defaultMaxBody = 1 << 20

type App struct {
	cfg     Config
	store   *store.Store
	auth    *auth.Service
	content *content.Service
	handler http.Handler
	routes  []string // 已注册的路由模式，如 "GET /api/posts"，供契约测试与 openapi.yaml 比对
}

func New(cfg Config, s *store.Store) (*App, error) {
	files, err := media.NewStorage(cfg.MediaDir)
	if err != nil {
		return nil, err
	}
	authSvc := auth.NewService(s)
	contentSvc := content.NewService(s, render.NewClient(cfg.RenderURL))
	a := &App{cfg: cfg, store: s, auth: authSvc, content: contentSvc}
	a.handler = a.mount(
		auth.NewHandler(authSvc, cfg.SiteURL),
		content.NewHandler(contentSvc),
		post.NewHandler(post.NewService(s, contentSvc), s),
		page.NewHandler(page.NewService(s, contentSvc), s),
		media.NewHandler(media.NewService(s, files, cfg.ImageWorkers)),
		feed.NewHandler(s, feed.Config{SiteURL: cfg.SiteURL}),
		settings.NewHandler(s),
	)
	return a, nil
}

func (a *App) Handler() http.Handler { return a.handler }

// Routes 返回已注册的全部路由模式（"METHOD /path"）。
func (a *App) Routes() []string { return slices.Clone(a.routes) }

func (a *App) mount(authH *auth.Handler, contentH *content.Handler, postH *post.Handler, pageH *page.Handler, mediaH *media.Handler, feedH *feed.Handler, settingsH *settings.Handler) http.Handler {
	mux := http.NewServeMux()

	type route struct {
		pattern string
		h       httpx.Handler
		maxBody int64
	}
	// register 为一组路由统一包裹中间件，由内到外依次为：wrap（如鉴权）→ 请求体上限 → Cache-Control。
	register := func(cache string, wrap func(http.Handler) http.Handler, routes ...route) {
		for _, rt := range routes {
			var h http.Handler = rt.h
			if wrap != nil {
				h = wrap(h)
			}
			h = httpx.MaxBody(orDefault(rt.maxBody, defaultMaxBody), h)
			if cache != "" {
				h = httpx.CacheControl(cache, h)
			}
			mux.Handle(rt.pattern, h)
			a.routes = append(a.routes, rt.pattern)
		}
	}

	register("no-cache", nil,
		route{pattern: "GET /api/posts", h: postH.PublicList},
		route{pattern: "GET /api/posts/{slug}", h: postH.PublicGet},
		route{pattern: "GET /api/pages/{slug}", h: pageH.PublicGet},
		route{pattern: "GET /api/tags", h: postH.PublicTags},
		route{pattern: "GET /api/tags/{slug}", h: postH.PublicTag},
		route{pattern: "GET /api/series", h: postH.PublicSeriesList},
		route{pattern: "GET /api/series/{slug}", h: postH.PublicSeries},
		route{pattern: "GET /api/search", h: postH.Search},
		route{pattern: "GET /api/settings", h: settingsH.Public},
	)

	// 认证：登录自行做跨站校验；账户、会话与令牌管理只接受浏览器会话。
	register("no-store", nil,
		route{pattern: "POST /api/auth/sessions", h: authH.Login},
	)
	register("no-store", authH.Require(""),
		route{pattern: "GET /api/auth/me", h: authH.Me},
	)
	register("no-store", authH.RequireSession,
		route{pattern: "DELETE /api/auth/sessions/current", h: authH.Logout},
		route{pattern: "GET /api/auth/sessions", h: authH.ListSessions},
		route{pattern: "DELETE /api/auth/sessions/{id}", h: authH.RevokeSession},
		route{pattern: "PUT /api/auth/password", h: authH.ChangePassword},
		route{pattern: "GET /api/auth/tokens", h: authH.ListTokens},
		route{pattern: "POST /api/auth/tokens", h: authH.CreateToken},
		route{pattern: "DELETE /api/auth/tokens/{id}", h: authH.RevokeToken},
	)

	// 管理接口按 scope 分组；浏览器会话拥有全部 scope。
	register("no-store", authH.Require(auth.ScopeContentRead),
		route{pattern: "GET /api/admin/render", h: contentH.Renderer},
		route{pattern: "GET /api/admin/posts", h: postH.AdminList},
		route{pattern: "GET /api/admin/posts/{id}", h: postH.Get},
		route{pattern: "GET /api/admin/pages", h: pageH.AdminList},
		route{pattern: "GET /api/admin/pages/{id}", h: pageH.Get},
		route{pattern: "GET /api/admin/tags", h: postH.AdminTags},
		route{pattern: "GET /api/admin/series", h: postH.AdminSeriesList},
		route{pattern: "GET /api/admin/series/{id}", h: postH.AdminSeries},
		route{pattern: "GET /api/admin/media", h: mediaH.List},
		route{pattern: "GET /api/admin/media/{id}", h: mediaH.Get},
		route{pattern: "GET /api/admin/settings", h: settingsH.Get},
	)
	register("no-store", authH.Require(auth.ScopeContentWrite),
		route{pattern: "POST /api/admin/render", h: contentH.Preview, maxBody: content.MaxRequestBody},
		route{pattern: "POST /api/admin/posts", h: postH.Create, maxBody: content.MaxRequestBody},
		route{pattern: "PUT /api/admin/posts/{id}", h: postH.Update, maxBody: content.MaxRequestBody},
		route{pattern: "POST /api/admin/pages", h: pageH.Create, maxBody: content.MaxRequestBody},
		route{pattern: "PUT /api/admin/pages/{id}", h: pageH.Update, maxBody: content.MaxRequestBody},
		route{pattern: "POST /api/admin/tags", h: postH.CreateTag},
		route{pattern: "PATCH /api/admin/tags/{id}", h: postH.UpdateTag},
		route{pattern: "DELETE /api/admin/tags/{id}", h: postH.DeleteTag},
		route{pattern: "POST /api/admin/series", h: postH.CreateSeries},
		route{pattern: "PATCH /api/admin/series/{id}", h: postH.UpdateSeries},
		route{pattern: "DELETE /api/admin/series/{id}", h: postH.DeleteSeries},
		route{pattern: "PUT /api/admin/series/{id}/posts", h: postH.ReorderSeries},
	)
	register("no-store", authH.Require(auth.ScopeContentPublish),
		route{pattern: "DELETE /api/admin/posts/{id}", h: postH.Delete},
		route{pattern: "PUT /api/admin/posts/{id}/publication", h: postH.Publish},
		route{pattern: "DELETE /api/admin/posts/{id}/publication", h: postH.Unpublish},
		route{pattern: "DELETE /api/admin/pages/{id}", h: pageH.Delete},
		route{pattern: "PUT /api/admin/pages/{id}/publication", h: pageH.Publish},
		route{pattern: "DELETE /api/admin/pages/{id}/publication", h: pageH.Unpublish},
	)
	register("no-store", authH.Require(auth.ScopeMediaWrite),
		route{pattern: "POST /api/admin/media", h: mediaH.Upload, maxBody: media.MaxUploadBody},
		route{pattern: "PATCH /api/admin/media/{id}", h: mediaH.Update},
		route{pattern: "DELETE /api/admin/media/{id}", h: mediaH.Delete},
	)
	register("no-store", authH.Require(auth.ScopeSettingsWrite),
		route{pattern: "PATCH /api/admin/settings", h: settingsH.Update},
	)

	register("", nil,
		route{pattern: "GET /feed.xml", h: feedH.Atom},
		route{pattern: "GET /rss.xml", h: feedH.RSS},
		route{pattern: "GET /sitemap.xml", h: feedH.Sitemap},
		route{pattern: "GET /robots.txt", h: feedH.Robots},
		route{pattern: "GET /media/{sha256}", h: mediaH.ServeDefault},
		route{pattern: "GET /media/{sha256}/{file}", h: mediaH.ServeVariant},
	)
	register("no-store", nil, route{pattern: "GET /healthz", h: a.healthz})

	// 全局中间件自下而上包裹，请求按相反顺序经过：先分配请求 ID，最后才进入路由。
	var h http.Handler = httpx.Problems(mux)
	h = httpx.Recover(h)
	h = httpx.AccessLog(h)
	h = httpx.RealIP(a.cfg.TrustProxy, h)
	h = httpx.WithProblemBase(a.cfg.SiteURL+"/problems/", h)
	h = httpx.WithRequestID(h)
	return h
}

func orDefault(v, def int64) int64 {
	if v > 0 {
		return v
	}
	return def
}

func (a *App) healthz(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.store.Ping(ctx); err != nil {
		slog.WarnContext(ctx, "health check failed", "err", err)
		return httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
	}
	return httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              a.cfg.ListenAddr,
		Handler:           a.handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    64 << 10,
	}

	go a.sessionJanitor(ctx)
	go a.rerenderStale(ctx)

	errc := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", a.cfg.ListenAddr)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// rerenderStale 在启动后用当前渲染器重新渲染过期内容。渲染服务可能晚于 API 就绪，失败时退避重试。
func (a *App) rerenderStale(ctx context.Context) {
	delay := 2 * time.Second
	for {
		report, err := a.content.Rerender(ctx)
		if err == nil {
			if report.Posts+report.Pages+report.Failed > 0 {
				slog.Info("rerendered stale content", "renderer_version", report.RendererVersion,
					"posts", report.Posts, "pages", report.Pages, "failed", report.Failed)
			}
			return
		}
		if ctx.Err() != nil {
			return
		}
		slog.Warn("rerender stale content", "err", err, "retry_in", delay)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay = min(delay*2, 5*time.Minute)
	}
}

// sessionJanitor 启动时及之后每小时清理一次过期会话，直到 ctx 取消。
func (a *App) sessionJanitor(ctx context.Context) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		n, err := a.auth.CleanupExpired(ctx)
		if err != nil && ctx.Err() == nil {
			slog.Error("cleanup expired sessions", "err", err)
		} else if n > 0 {
			slog.Info("cleaned up expired sessions", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
