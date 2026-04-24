package auth

import (
	"context"
	"net/http"
	"sync"
	"time"

	"blog/api/internal/httpx"
	"blog/api/internal/store"
)

// __Host- 前缀要求 cookie 必须 Secure、Path=/ 且不带 Domain，防止被子域覆盖。
const cookieName = "__Host-session"

type principalKey struct{}

func FromContext(ctx context.Context) *Principal {
	p, _ := ctx.Value(principalKey{}).(*Principal)
	return p
}

func setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   max(int(time.Until(expires).Round(time.Second).Seconds()), 1),
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

// Require 是鉴权中间件：校验会话 cookie 并把 Principal 注入 context；会话续期时同步刷新 cookie 有效期。
func (h *Handler) Require(next http.Handler) http.Handler {
	return httpx.Handler(func(w http.ResponseWriter, r *http.Request) error {
		var token string
		if c, err := r.Cookie(cookieName); err == nil {
			token = c.Value
		}
		p, err := h.svc.Authenticate(r.Context(), token)
		if err != nil {
			if token != "" {
				clearSessionCookie(w)
			}
			return err
		}
		if p.Refreshed {
			setSessionCookie(w, token, p.ExpiresAt)
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
		return nil
	})
}

type Handler struct {
	svc     *Service
	limiter *limiter
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc, limiter: newLimiter(10, 30*time.Second)}
}

type userJSON struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

type sessionJSON struct {
	ID         int64      `json:"id"`
	Current    bool       `json:"current"`
	UserAgent  string     `json:"user_agent"`
	IP         *string    `json:"ip"`
	CreatedAt  httpx.Time `json:"created_at"`
	LastSeenAt httpx.Time `json:"last_seen_at"`
	ExpiresAt  httpx.Time `json:"expires_at"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) error {
	ip := httpx.ClientIP(r)
	if ok, retry := h.limiter.allow(ip.String()); !ok {
		return httpx.RateLimited(retry)
	}
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	var v httpx.Validator
	if in.Username == "" {
		v.Add("#/username", "required", "不能为空")
	}
	if in.Password == "" {
		v.Add("#/password", "required", "不能为空")
	} else if len(in.Password) > 4*maxPasswordLen { // 按 UTF-8 最大字节数粗略限制，避免对超长输入跑 argon2
		v.Add("#/password", "too_long", "密码过长")
	}
	if err := v.Err(); err != nil {
		return err
	}
	token, user, sess, err := h.svc.Login(r.Context(), in.Username, in.Password, r.UserAgent(), ip)
	if err != nil {
		return err
	}
	setSessionCookie(w, token, sess.ExpiresAt)
	return httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"user": userJSON{ID: user.ID, Username: user.Username, DisplayName: user.DisplayName},
		"session": map[string]any{
			"id":         sess.ID,
			"expires_at": httpx.NewTime(sess.ExpiresAt),
		},
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.Logout(r.Context(), FromContext(r.Context())); err != nil {
		return err
	}
	clearSessionCookie(w)
	return httpx.NoContent(w)
}

func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) error {
	p := FromContext(r.Context())
	rows, err := h.svc.ListSessions(r.Context(), p.UserID)
	if err != nil {
		return err
	}
	items := make([]sessionJSON, 0, len(rows))
	for _, s := range rows {
		items = append(items, toSessionJSON(s, p.SessionID))
	}
	return httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func toSessionJSON(s store.Session, currentID int64) sessionJSON {
	out := sessionJSON{
		ID:         s.ID,
		Current:    s.ID == currentID,
		UserAgent:  s.UserAgent,
		CreatedAt:  httpx.NewTime(s.CreatedAt),
		LastSeenAt: httpx.NewTime(s.LastSeenAt),
		ExpiresAt:  httpx.NewTime(s.ExpiresAt),
	}
	if s.Ip != nil {
		ip := s.Ip.String()
		out.IP = &ip
	}
	return out
}

func (h *Handler) RevokeSession(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	p := FromContext(r.Context())
	if err := h.svc.RevokeSession(r.Context(), p.UserID, id); err != nil {
		return err
	}
	if id == p.SessionID {
		clearSessionCookie(w)
	}
	return httpx.NoContent(w)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) error {
	p := FromContext(r.Context())
	return httpx.WriteJSON(w, http.StatusOK, userJSON{ID: p.UserID, Username: p.Username, DisplayName: p.DisplayName})
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.svc.ChangePassword(r.Context(), FromContext(r.Context()), in.CurrentPassword, in.NewPassword); err != nil {
		return err
	}
	return httpx.NoContent(w)
}

// limiter 是按 key（客户端 IP）划分的令牌桶限流器：桶容量为 burst，每 interval 补充一个令牌。
type limiter struct {
	mu       sync.Mutex
	burst    float64
	interval time.Duration
	buckets  map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newLimiter(burst int, interval time.Duration) *limiter {
	return &limiter{burst: float64(burst), interval: interval, buckets: map[string]*bucket{}}
}

func (l *limiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.buckets) > 10000 {
		l.prune(now)
	}
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	b.tokens = min(l.burst, b.tokens+float64(now.Sub(b.last))/float64(l.interval))
	b.last = now
	if b.tokens < 1 {
		return false, time.Duration((1 - b.tokens) * float64(l.interval))
	}
	b.tokens--
	return true, 0
}

// prune 删除已回满的桶，它们与新建的桶等价，删除不影响限流结果。
func (l *limiter) prune(now time.Time) {
	full := time.Duration(l.burst * float64(l.interval))
	for k, b := range l.buckets {
		if now.Sub(b.last) >= full {
			delete(l.buckets, k)
		}
	}
}
