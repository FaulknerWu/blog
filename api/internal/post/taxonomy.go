package post

import (
	"fmt"
	"net/http"
	"strings"

	"blog/api/internal/httpx"
	"blog/api/internal/store"
)

type adminTag struct {
	ID                 int64      `json:"id"`
	Slug               string     `json:"slug"`
	Name               string     `json:"name"`
	PostCount          int32      `json:"post_count"`
	PublishedPostCount int32      `json:"published_post_count"`
	Version            int32      `json:"version"`
	CreatedAt          httpx.Time `json:"created_at"`
	UpdatedAt          httpx.Time `json:"updated_at"`
}

func toAdminTag(t store.GetAdminTagRow) adminTag {
	return adminTag{
		ID: t.ID, Slug: t.Slug, Name: t.Name,
		PostCount: t.PostCount, PublishedPostCount: t.PublishedPostCount,
		Version: t.Version, CreatedAt: httpx.NewTime(t.CreatedAt), UpdatedAt: httpx.NewTime(t.UpdatedAt),
	}
}

type tagInput struct {
	Name *string `json:"name"`
	Slug *string `json:"slug"`
}

func validateTag(name, slug string) error {
	var v httpx.Validator
	v.Length("#/name", name, 1, 50)
	v.Slug("#/slug", slug, 50)
	return v.Err()
}

func mapTagError(err error) error {
	switch store.ConstraintViolation(err) {
	case "tags_slug_key":
		return httpx.Conflict("slug 已被其他标签使用",
			httpx.FieldError{Pointer: "#/slug", Code: "duplicate", Detail: "slug 已被其他标签使用"})
	case "tags_name_lower_key":
		return httpx.Conflict("标签名称已存在",
			httpx.FieldError{Pointer: "#/name", Code: "duplicate", Detail: "标签名称已存在（不区分大小写）"})
	}
	return err
}

func (h *Handler) respondTag(w http.ResponseWriter, r *http.Request, id int64, status int) error {
	t, err := h.store.GetAdminTag(r.Context(), id)
	if store.IsNotFound(err) {
		return httpx.NotFound("标签不存在")
	}
	if err != nil {
		return err
	}
	httpx.SetETag(w, t.Version)
	return httpx.WriteJSON(w, status, toAdminTag(t))
}

func (h *Handler) AdminTags(w http.ResponseWriter, r *http.Request) error {
	rows, err := h.store.ListAdminTags(r.Context())
	if err != nil {
		return err
	}
	items := make([]adminTag, 0, len(rows))
	for _, t := range rows {
		items = append(items, toAdminTag(store.GetAdminTagRow(t)))
	}
	return httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) CreateTag(w http.ResponseWriter, r *http.Request) error {
	var in tagInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	name, slug := strings.TrimSpace(deref(in.Name)), deref(in.Slug)
	if err := validateTag(name, slug); err != nil {
		return err
	}
	id, err := h.store.CreateTag(r.Context(), store.CreateTagParams{Name: name, Slug: slug})
	if err != nil {
		return mapTagError(err)
	}
	w.Header().Set("Location", fmt.Sprintf("/api/admin/tags/%d", id))
	return h.respondTag(w, r, id, http.StatusCreated)
}

func (h *Handler) UpdateTag(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in tagInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	err = h.store.InTx(ctx, func(q *store.Queries) error {
		cur, err := q.LockTag(ctx, id)
		if store.IsNotFound(err) {
			return httpx.NotFound("标签不存在")
		}
		if err != nil {
			return err
		}
		if err := httpx.ParseIfMatch(r).Check(cur.Version, cur.UpdatedAt); err != nil {
			return err
		}
		if in.Name == nil && in.Slug == nil {
			return nil
		}
		name, slug := cur.Name, cur.Slug
		if in.Name != nil {
			name = strings.TrimSpace(*in.Name)
		}
		if in.Slug != nil {
			slug = *in.Slug
		}
		if err := validateTag(name, slug); err != nil {
			return err
		}
		return q.UpdateTag(ctx, store.UpdateTagParams{ID: id, Name: name, Slug: slug})
	})
	if err != nil {
		return mapTagError(err)
	}
	return h.respondTag(w, r, id, http.StatusOK)
}

func (h *Handler) DeleteTag(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	ctx := r.Context()
	err = h.store.InTx(ctx, func(q *store.Queries) error {
		cur, err := q.LockTag(ctx, id)
		if store.IsNotFound(err) {
			return httpx.NotFound("标签不存在")
		}
		if err != nil {
			return err
		}
		if err := httpx.ParseIfMatch(r).Check(cur.Version, cur.UpdatedAt); err != nil {
			return err
		}
		return q.DeleteTag(ctx, id)
	})
	if err != nil {
		return err
	}
	return httpx.NoContent(w)
}

type adminSeriesJSON struct {
	ID                 int64      `json:"id"`
	Slug               string     `json:"slug"`
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	PostCount          int32      `json:"post_count"`
	PublishedPostCount int32      `json:"published_post_count"`
	Version            int32      `json:"version"`
	CreatedAt          httpx.Time `json:"created_at"`
	UpdatedAt          httpx.Time `json:"updated_at"`
}

type adminSeriesPost struct {
	ID       int64  `json:"id"`
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	State    string `json:"state"`
	Position int32  `json:"position"`
}

type adminSeriesDetail struct {
	adminSeriesJSON
	Posts []adminSeriesPost `json:"posts"`
}

func toAdminSeries(s store.GetAdminSeriesRow) adminSeriesJSON {
	return adminSeriesJSON{
		ID: s.ID, Slug: s.Slug, Title: s.Title, Description: s.Description,
		PostCount: s.PostCount, PublishedPostCount: s.PublishedPostCount,
		Version: s.Version, CreatedAt: httpx.NewTime(s.CreatedAt), UpdatedAt: httpx.NewTime(s.UpdatedAt),
	}
}

type seriesInput struct {
	Title       *string `json:"title"`
	Slug        *string `json:"slug"`
	Description *string `json:"description"`
}

func validateSeries(title, slug, description string) error {
	var v httpx.Validator
	v.Length("#/title", title, 1, 200)
	v.Slug("#/slug", slug, 100)
	v.Length("#/description", description, 0, 2000)
	return v.Err()
}

func mapSeriesError(err error) error {
	if store.ConstraintViolation(err) == "series_slug_key" {
		return httpx.Conflict("slug 已被其他专栏使用",
			httpx.FieldError{Pointer: "#/slug", Code: "duplicate", Detail: "slug 已被其他专栏使用"})
	}
	return err
}

func (h *Handler) respondSeries(w http.ResponseWriter, r *http.Request, id int64, status int, withPosts bool) error {
	ctx := r.Context()
	s, err := h.store.GetAdminSeries(ctx, id)
	if store.IsNotFound(err) {
		return httpx.NotFound("专栏不存在")
	}
	if err != nil {
		return err
	}
	httpx.SetETag(w, s.Version)
	if !withPosts {
		return httpx.WriteJSON(w, status, toAdminSeries(s))
	}
	rows, err := h.store.ListAdminSeriesPosts(ctx, id)
	if err != nil {
		return err
	}
	posts := make([]adminSeriesPost, 0, len(rows))
	for _, p := range rows {
		posts = append(posts, adminSeriesPost{ID: p.ID, Slug: p.Slug, Title: p.Title, State: p.State, Position: p.Position})
	}
	return httpx.WriteJSON(w, status, adminSeriesDetail{adminSeriesJSON: toAdminSeries(s), Posts: posts})
}

func (h *Handler) AdminSeriesList(w http.ResponseWriter, r *http.Request) error {
	rows, err := h.store.ListAdminSeries(r.Context())
	if err != nil {
		return err
	}
	items := make([]adminSeriesJSON, 0, len(rows))
	for _, s := range rows {
		items = append(items, toAdminSeries(store.GetAdminSeriesRow(s)))
	}
	return httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) CreateSeries(w http.ResponseWriter, r *http.Request) error {
	var in seriesInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	title, slug, desc := strings.TrimSpace(deref(in.Title)), deref(in.Slug), deref(in.Description)
	if err := validateSeries(title, slug, desc); err != nil {
		return err
	}
	id, err := h.store.CreateSeries(r.Context(), store.CreateSeriesParams{Title: title, Slug: slug, Description: desc})
	if err != nil {
		return mapSeriesError(err)
	}
	w.Header().Set("Location", fmt.Sprintf("/api/admin/series/%d", id))
	return h.respondSeries(w, r, id, http.StatusCreated, false)
}

func (h *Handler) AdminSeries(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	return h.respondSeries(w, r, id, http.StatusOK, true)
}

func (h *Handler) UpdateSeries(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in seriesInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	err = h.store.InTx(ctx, func(q *store.Queries) error {
		cur, err := q.LockSeries(ctx, id)
		if store.IsNotFound(err) {
			return httpx.NotFound("专栏不存在")
		}
		if err != nil {
			return err
		}
		if err := httpx.ParseIfMatch(r).Check(cur.Version, cur.UpdatedAt); err != nil {
			return err
		}
		if in.Title == nil && in.Slug == nil && in.Description == nil {
			return nil
		}
		title, slug, desc := cur.Title, cur.Slug, cur.Description
		if in.Title != nil {
			title = strings.TrimSpace(*in.Title)
		}
		if in.Slug != nil {
			slug = *in.Slug
		}
		if in.Description != nil {
			desc = *in.Description
		}
		if err := validateSeries(title, slug, desc); err != nil {
			return err
		}
		return q.UpdateSeries(ctx, store.UpdateSeriesParams{ID: id, Title: title, Slug: slug, Description: desc})
	})
	if err != nil {
		return mapSeriesError(err)
	}
	return h.respondSeries(w, r, id, http.StatusOK, false)
}

func (h *Handler) DeleteSeries(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	ctx := r.Context()
	err = h.store.InTx(ctx, func(q *store.Queries) error {
		cur, err := q.LockSeries(ctx, id)
		if store.IsNotFound(err) {
			return httpx.NotFound("专栏不存在")
		}
		if err != nil {
			return err
		}
		if err := httpx.ParseIfMatch(r).Check(cur.Version, cur.UpdatedAt); err != nil {
			return err
		}
		if err := q.DetachSeriesPosts(ctx, id); err != nil {
			return err
		}
		return q.DeleteSeries(ctx, id)
	})
	if err != nil {
		return err
	}
	return httpx.NoContent(w)
}

func (h *Handler) ReorderSeries(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		PostIDs []int64 `json:"post_ids"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	err = h.store.InTx(ctx, func(q *store.Queries) error {
		if _, err := q.LockSeries(ctx, id); store.IsNotFound(err) {
			return httpx.NotFound("专栏不存在")
		} else if err != nil {
			return err
		}
		members, err := q.ListAdminSeriesPosts(ctx, id)
		if err != nil {
			return err
		}
		current := make(map[int64]bool, len(members))
		for _, m := range members {
			current[m.ID] = true
		}
		// post_ids 必须恰好是当前成员的一个排列。
		seen := make(map[int64]bool, len(in.PostIDs))
		for _, pid := range in.PostIDs {
			if !current[pid] || seen[pid] {
				seen = nil
				break
			}
			seen[pid] = true
		}
		if seen == nil || len(seen) != len(current) {
			return httpx.Validation(httpx.FieldError{Pointer: "#/post_ids", Code: "mismatch",
				Detail: "必须恰好包含专栏当前的全部文章且不能重复"})
		}
		// posts_series_position_key 是 DEFERRABLE INITIALLY DEFERRED 约束，
		// 逐行更新时出现的临时重复序号只在提交时检查，不会报错。
		for i, pid := range in.PostIDs {
			if err := q.SetSeriesPosition(ctx, store.SetSeriesPositionParams{
				ID: pid, SeriesID: id, Position: int32(i + 1),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return h.respondSeries(w, r, id, http.StatusOK, true)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
