package post

import (
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"blog/api/internal/content"
	"blog/api/internal/httpx"
	"blog/api/internal/media"
	"blog/api/internal/store"
)

func (h *Handler) PublicList(w http.ResponseWriter, r *http.Request) error {
	limit, err := httpx.QueryLimit(r, 20, 50)
	if err != nil {
		return err
	}
	cursor, err := httpx.ParseCursor(r)
	if err != nil {
		return err
	}
	ts, id := cursor.Params()
	ctx := r.Context()
	rows, err := h.store.ListPublishedPosts(ctx, store.ListPublishedPostsParams{
		TagSlug: httpx.QueryString(r, "tag"), CursorTs: ts, CursorID: id, Lim: limit + 1,
	})
	if err != nil {
		return err
	}
	var ids, covers []int64
	for _, row := range rows {
		ids = append(ids, row.ID)
		covers = append(covers, derefIDs(row.CoverMediaID)...)
	}
	tags, err := loadTags(ctx, h.store.Queries, ids)
	if err != nil {
		return err
	}
	images, err := media.LoadImages(ctx, h.store.Queries, covers)
	if err != nil {
		return err
	}
	page := httpx.NewPage(rows, limit,
		func(row store.ListPublishedPostsRow) postSummary {
			s := postSummary{
				ID:          row.ID,
				Slug:        row.Slug,
				Title:       row.Title,
				Summary:     row.Summary,
				WordCount:   row.WordCount,
				PublishedAt: httpx.NewTime(row.PublishedAt),
				UpdatedAt:   httpx.NewTime(row.UpdatedAt),
				Tags:        tags.public(row.ID),
				Cover:       imagePtr(images, row.CoverMediaID),
			}
			if row.SeriesSlug != nil && row.SeriesTitle != nil {
				s.Series = &seriesRef{Slug: *row.SeriesSlug, Title: *row.SeriesTitle}
			}
			return s
		},
		func(row store.ListPublishedPostsRow) httpx.Cursor {
			return httpx.Cursor{TS: row.PublishedAt, ID: row.ID}
		})
	return httpx.WritePublicJSON(w, r, page)
}

func (h *Handler) PublicGet(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	slug := r.PathValue("slug")
	row, err := h.store.GetPublishedPostBySlug(ctx, slug)
	// 旧 slug 命中历史记录时 301 到当前 slug。
	if store.IsNotFound(err) {
		current, aerr := h.store.ResolvePublishedSlugAlias(ctx, slug)
		if store.IsNotFound(aerr) {
			return httpx.NotFound("文章不存在")
		}
		if aerr != nil {
			return aerr
		}
		w.Header().Set("Location", "/api/posts/"+url.PathEscape(current))
		w.WriteHeader(http.StatusMovedPermanently)
		return nil
	}
	if err != nil {
		return err
	}

	tags, err := loadTags(ctx, h.store.Queries, []int64{row.ID})
	if err != nil {
		return err
	}
	images, err := media.LoadImages(ctx, h.store.Queries, derefIDs(row.CoverMediaID))
	if err != nil {
		return err
	}
	out := postDetail{
		ID:          row.ID,
		Slug:        row.Slug,
		Title:       row.Title,
		Summary:     row.Summary,
		HTML:        row.BodyHtml,
		TOC:         content.DecodeTOC(row.Toc),
		HasMath:     row.HasMath,
		WordCount:   row.WordCount,
		PublishedAt: httpx.NewTime(row.PublishedAt),
		UpdatedAt:   httpx.NewTime(row.UpdatedAt),
		Tags:        tags.public(row.ID),
		Cover:       imagePtr(images, row.CoverMediaID),
	}
	if row.SeriesID != nil && row.SeriesSlug != nil && row.SeriesTitle != nil {
		members, err := h.store.ListPublishedSeriesPosts(ctx, *row.SeriesID)
		if err != nil {
			return err
		}
		nav := &seriesNav{Slug: *row.SeriesSlug, Title: *row.SeriesTitle, Total: len(members)}
		for i, m := range members {
			if m.ID != row.ID {
				continue
			}
			nav.Index = i + 1
			if i > 0 {
				nav.Prev = &postRef{Slug: members[i-1].Slug, Title: members[i-1].Title}
			}
			if i+1 < len(members) {
				nav.Next = &postRef{Slug: members[i+1].Slug, Title: members[i+1].Title}
			}
		}
		out.Series = nav
	}

	newer, err := h.store.GetNewerPublishedPost(ctx, store.GetNewerPublishedPostParams{PublishedAt: row.PublishedAt, ID: row.ID})
	if err == nil {
		out.Adjacent.Newer = &postRef{Slug: newer.Slug, Title: newer.Title}
	} else if !store.IsNotFound(err) {
		return err
	}
	older, err := h.store.GetOlderPublishedPost(ctx, store.GetOlderPublishedPostParams{PublishedAt: row.PublishedAt, ID: row.ID})
	if err == nil {
		out.Adjacent.Older = &postRef{Slug: older.Slug, Title: older.Title}
	} else if !store.IsNotFound(err) {
		return err
	}
	return httpx.WritePublicJSON(w, r, out)
}

type publicTag struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	PostCount int32  `json:"post_count"`
}

func (h *Handler) PublicTags(w http.ResponseWriter, r *http.Request) error {
	rows, err := h.store.ListPublicTags(r.Context())
	if err != nil {
		return err
	}
	items := make([]publicTag, 0, len(rows))
	for _, t := range rows {
		items = append(items, publicTag(t))
	}
	return httpx.WritePublicJSON(w, r, map[string]any{"items": items})
}

func (h *Handler) PublicTag(w http.ResponseWriter, r *http.Request) error {
	t, err := h.store.GetPublicTag(r.Context(), r.PathValue("slug"))
	if store.IsNotFound(err) {
		return httpx.NotFound("标签不存在")
	}
	if err != nil {
		return err
	}
	return httpx.WritePublicJSON(w, r, publicTag(t))
}

type publicSeries struct {
	Slug            string     `json:"slug"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	PostCount       int32      `json:"post_count"`
	LastPublishedAt httpx.Time `json:"last_published_at"`
}

func (h *Handler) PublicSeriesList(w http.ResponseWriter, r *http.Request) error {
	rows, err := h.store.ListPublicSeries(r.Context())
	if err != nil {
		return err
	}
	items := make([]publicSeries, 0, len(rows))
	for _, s := range rows {
		items = append(items, publicSeries{
			Slug: s.Slug, Title: s.Title, Description: s.Description,
			PostCount: s.PostCount, LastPublishedAt: httpx.NewTime(s.LastPublishedAt),
		})
	}
	return httpx.WritePublicJSON(w, r, map[string]any{"items": items})
}

type publicSeriesPost struct {
	Index       int        `json:"index"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	PublishedAt httpx.Time `json:"published_at"`
}

func (h *Handler) PublicSeries(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	s, err := h.store.GetSeriesBySlug(ctx, r.PathValue("slug"))
	if store.IsNotFound(err) {
		return httpx.NotFound("专栏不存在")
	}
	if err != nil {
		return err
	}
	rows, err := h.store.ListPublishedSeriesPosts(ctx, s.ID)
	if err != nil {
		return err
	}
	// 没有已发布文章的专栏对外不可见。
	if len(rows) == 0 {
		return httpx.NotFound("专栏不存在")
	}
	posts := make([]publicSeriesPost, 0, len(rows))
	for i, p := range rows {
		posts = append(posts, publicSeriesPost{Index: i + 1, Slug: p.Slug, Title: p.Title, PublishedAt: httpx.NewTime(p.PublishedAt)})
	}
	return httpx.WritePublicJSON(w, r, map[string]any{
		"slug": s.Slug, "title": s.Title, "description": s.Description, "posts": posts,
	})
}

type searchResult struct {
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Summary     string     `json:"summary"`
	Snippet     string     `json:"snippet"`
	PublishedAt httpx.Time `json:"published_at"`
}

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) error {
	limit, err := httpx.QueryLimit(r, 20, 50)
	if err != nil {
		return err
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if n := utf8.RuneCountInString(q); n < 1 || n > 64 {
		code, detail := "required", "不能为空"
		if n > 64 {
			code, detail = "too_long", "长度不能超过 64 个字符"
		}
		return httpx.Validation(httpx.FieldError{Parameter: "q", Code: code, Detail: detail})
	}
	rows, err := h.store.SearchPublishedPosts(r.Context(), store.SearchPublishedPostsParams{
		Q: q, Pattern: httpx.LikePattern(q), Lim: limit,
	})
	if err != nil {
		return err
	}
	items := make([]searchResult, 0, len(rows))
	for _, row := range rows {
		items = append(items, searchResult{
			Slug:        row.Slug,
			Title:       row.Title,
			Summary:     row.Summary,
			Snippet:     snippet(row),
			PublishedAt: httpx.NewTime(row.PublishedAt),
		})
	}
	return httpx.WritePublicJSON(w, r, map[string]any{"items": items})
}

// snippet 压缩空白并在片段未到达正文开头/结尾时补省略号。SnippetStart 是从 1 开始的字符位置。
func snippet(row store.SearchPublishedPostsRow) string {
	s := strings.Join(strings.Fields(row.Snippet), " ")
	if s == "" {
		return ""
	}
	if row.SnippetStart > 1 {
		s = "…" + s
	}
	if int(row.SnippetStart)-1+utf8.RuneCountInString(row.Snippet) < int(row.PlainLength) {
		s += "…"
	}
	return s
}
