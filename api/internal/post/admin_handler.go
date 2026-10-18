package post

import (
	"fmt"
	"net/http"
	"time"

	"blog/api/internal/httpx"
	"blog/api/internal/store"
)

type Handler struct {
	svc   *Service
	store *store.Store
}

func NewHandler(svc *Service, s *store.Store) *Handler { return &Handler{svc: svc, store: s} }

var validStates = map[string]bool{"draft": true, "scheduled": true, "published": true}

func (h *Handler) AdminList(w http.ResponseWriter, r *http.Request) error {
	limit, err := httpx.QueryLimit(r, 50, 100)
	if err != nil {
		return err
	}
	cursor, err := httpx.ParseCursor(r)
	if err != nil {
		return err
	}
	tagID, err := httpx.QueryInt64(r, "tag_id")
	if err != nil {
		return err
	}
	state := httpx.QueryString(r, "state")
	if state != nil && !validStates[*state] {
		return httpx.Validation(httpx.FieldError{Parameter: "state", Code: "invalid_value",
			Detail: "取值为 draft、scheduled 或 published"})
	}
	stale, err := httpx.QueryBool(r, "stale")
	if err != nil {
		return err
	}
	staleAgainst, err := h.svc.content.StaleAgainst(r.Context(), stale)
	if err != nil {
		return err
	}
	var titlePattern *string
	if q := httpx.QueryString(r, "q"); q != nil {
		p := httpx.LikePattern(*q)
		titlePattern = &p
	}
	ts, id := cursor.Params()
	rows, err := h.store.ListAdminPosts(r.Context(), store.ListAdminPostsParams{
		State:        state,
		TitlePattern: titlePattern,
		TagID:        tagID,
		Slug:         httpx.QueryString(r, "slug"),
		StaleAgainst: staleAgainst,
		CursorTs:     ts,
		CursorID:     id,
		Lim:          limit + 1,
	})
	if err != nil {
		return err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	tags, err := loadTags(r.Context(), h.store.Queries, ids)
	if err != nil {
		return err
	}
	page := httpx.NewPage(rows, limit,
		func(row store.ListAdminPostsRow) adminPostSummary {
			return toAdminSummary(store.GetAdminPostSummaryRow(row), tags.admin(row.ID))
		},
		func(row store.ListAdminPostsRow) httpx.Cursor { return httpx.Cursor{TS: row.UpdatedAt, ID: row.ID} })
	return httpx.WriteJSON(w, http.StatusOK, page)
}

func (h *Handler) respondSummary(w http.ResponseWriter, r *http.Request, id int64, status int) error {
	s, err := h.svc.Summary(r.Context(), id)
	if err != nil {
		return err
	}
	httpx.SetETag(w, s.Version)
	return httpx.WriteJSON(w, status, s)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) error {
	var in PostInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	id, err := h.svc.Create(r.Context(), in)
	if err != nil {
		return err
	}
	w.Header().Set("Location", fmt.Sprintf("/api/admin/posts/%d", id))
	return h.respondSummary(w, r, id, http.StatusCreated)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	p, err := h.svc.Get(r.Context(), id)
	if err != nil {
		return err
	}
	httpx.SetETag(w, p.Version)
	return httpx.WriteJSON(w, http.StatusOK, p)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	ifMatch := httpx.ParseIfMatch(r)
	if err := ifMatch.Require(); err != nil {
		return err
	}
	var in PostInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.svc.Update(r.Context(), id, ifMatch, in); err != nil {
		return err
	}
	return h.respondSummary(w, r, id, http.StatusOK)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	if err := h.svc.Delete(r.Context(), id, httpx.ParseIfMatch(r)); err != nil {
		return err
	}
	return httpx.NoContent(w)
}

func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	ifMatch := httpx.ParseIfMatch(r)
	if err := ifMatch.Require(); err != nil {
		return err
	}
	var in struct {
		PublishedAt *time.Time `json:"published_at"`
	}
	if err := httpx.DecodeOptionalJSON(r, &in); err != nil {
		return err
	}
	if err := h.svc.Publish(r.Context(), id, ifMatch, in.PublishedAt); err != nil {
		return err
	}
	return h.respondSummary(w, r, id, http.StatusOK)
}

func (h *Handler) Unpublish(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	if err := h.svc.Unpublish(r.Context(), id, httpx.ParseIfMatch(r)); err != nil {
		return err
	}
	return h.respondSummary(w, r, id, http.StatusOK)
}
