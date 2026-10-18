package page

import (
	"fmt"
	"net/http"
	"net/url"

	"blog/api/internal/content"
	"blog/api/internal/httpx"
	"blog/api/internal/store"
)

type Handler struct {
	svc   *Service
	store *store.Store
}

func NewHandler(svc *Service, s *store.Store) *Handler { return &Handler{svc: svc, store: s} }

type adminPageSummary struct {
	ID              int64      `json:"id"`
	Slug            string     `json:"slug"`
	Title           string     `json:"title"`
	State           string     `json:"state"`
	RendererVersion string     `json:"renderer_version"`
	Version         int32      `json:"version"`
	CreatedAt       httpx.Time `json:"created_at"`
	UpdatedAt       httpx.Time `json:"updated_at"`
}

type adminPage struct {
	adminPageSummary
	Description string `json:"description"`
	BodyMD      string `json:"body_md"`
}

func toAdminSummary(r store.ListAdminPagesRow) adminPageSummary {
	return adminPageSummary{
		ID:              r.ID,
		Slug:            r.Slug,
		Title:           r.Title,
		State:           r.Status,
		RendererVersion: r.RendererVersion,
		Version:         r.Version,
		CreatedAt:       httpx.NewTime(r.CreatedAt),
		UpdatedAt:       httpx.NewTime(r.UpdatedAt),
	}
}

func toAdminPage(r store.GetAdminPageRow) adminPage {
	return adminPage{
		adminPageSummary: toAdminSummary(store.ListAdminPagesRow{
			ID: r.ID, Slug: r.Slug, Title: r.Title, Status: r.Status, RendererVersion: r.RendererVersion,
			Version: r.Version, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}),
		Description: r.Description,
		BodyMD:      r.BodyMd,
	}
}

func (h *Handler) AdminList(w http.ResponseWriter, r *http.Request) error {
	stale, err := httpx.QueryBool(r, "stale")
	if err != nil {
		return err
	}
	staleAgainst, err := h.svc.content.StaleAgainst(r.Context(), stale)
	if err != nil {
		return err
	}
	rows, err := h.store.ListAdminPages(r.Context(), store.ListAdminPagesParams{
		Slug:         httpx.QueryString(r, "slug"),
		StaleAgainst: staleAgainst,
	})
	if err != nil {
		return err
	}
	items := make([]adminPageSummary, 0, len(rows))
	for _, row := range rows {
		items = append(items, toAdminSummary(row))
	}
	return httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) respondSummary(w http.ResponseWriter, r *http.Request, id int64, status int) error {
	p, err := h.svc.Get(r.Context(), id)
	if err != nil {
		return err
	}
	httpx.SetETag(w, p.Version)
	return httpx.WriteJSON(w, status, toAdminPage(p).adminPageSummary)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) error {
	var in Input
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	id, err := h.svc.Create(r.Context(), in)
	if err != nil {
		return err
	}
	w.Header().Set("Location", fmt.Sprintf("/api/admin/pages/%d", id))
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
	return httpx.WriteJSON(w, http.StatusOK, toAdminPage(p))
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
	var in Input
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
	if err := httpx.DecodeOptionalJSON(r, &struct{}{}); err != nil {
		return err
	}
	if err := h.svc.Publish(r.Context(), id, ifMatch); err != nil {
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

type publicPage struct {
	Slug        string            `json:"slug"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	HTML        string            `json:"html"`
	TOC         []content.TOCItem `json:"toc"`
	HasMath     bool              `json:"has_math"`
	UpdatedAt   httpx.Time        `json:"updated_at"`
}

func (h *Handler) PublicGet(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	slug := r.PathValue("slug")
	row, err := h.store.GetPublishedPageBySlug(ctx, slug)
	if store.IsNotFound(err) {
		current, aerr := h.store.ResolvePublishedPageSlugAlias(ctx, slug)
		if store.IsNotFound(aerr) {
			return httpx.NotFound("页面不存在")
		}
		if aerr != nil {
			return aerr
		}
		w.Header().Set("Location", "/api/pages/"+url.PathEscape(current))
		w.WriteHeader(http.StatusMovedPermanently)
		return nil
	}
	if err != nil {
		return err
	}
	return httpx.WritePublicJSON(w, r, publicPage{
		Slug:        row.Slug,
		Title:       row.Title,
		Description: row.Description,
		HTML:        row.BodyHtml,
		TOC:         content.DecodeTOC(row.Toc),
		HasMath:     row.HasMath,
		UpdatedAt:   httpx.NewTime(row.UpdatedAt),
	})
}
