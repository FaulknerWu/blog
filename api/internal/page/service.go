package page

import (
	"context"

	"blog/api/internal/content"
	"blog/api/internal/httpx"
	"blog/api/internal/store"
)

// 页面挂在站点根路径（/{slug}），需避开前端路由与后端占用的一级路径。
var reservedSlugs = map[string]bool{
	"admin": true, "api": true, "archive": true, "archives": true, "assets": true,
	"feed": true, "healthz": true, "login": true, "logout": true, "media": true,
	"page": true, "pages": true, "posts": true, "problems": true, "rss": true,
	"search": true, "series": true, "sitemap": true, "static": true, "tags": true,
}

type Input struct {
	Title       string            `json:"title"`
	Slug        string            `json:"slug"`
	Description string            `json:"description"`
	BodyMD      string            `json:"body_md"`
	Rendered    *content.Rendered `json:"rendered"`
}

type pageContent struct {
	in   Input
	body content.Body
}

type Service struct {
	store *store.Store
}

func NewService(s *store.Store) *Service { return &Service{store: s} }

func validate(in Input) (*pageContent, error) {
	var v httpx.Validator
	v.Length("#/title", in.Title, 1, 200)
	if v.Slug("#/slug", in.Slug, 100) && reservedSlugs[in.Slug] {
		v.Add("#/slug", "reserved", "该 slug 为站点保留路径")
	}
	v.Length("#/description", in.Description, 0, 500)
	body := content.Validate(&v, in.BodyMD, in.Rendered)
	if err := v.Err(); err != nil {
		return nil, err
	}
	return &pageContent{in: in, body: body}, nil
}

func resolveMedia(ctx context.Context, q *store.Queries, c *pageContent) ([]int64, error) {
	var v httpx.Validator
	ids, err := content.ResolveMedia(ctx, q, &v, c.body.MediaRefs)
	if err != nil {
		return nil, err
	}
	return ids, v.Err()
}

func writeAssociations(ctx context.Context, q *store.Queries, pageID int64, c *pageContent, mediaIDs []int64, oldSlug string) error {
	if err := q.DeletePageMedia(ctx, pageID); err != nil {
		return err
	}
	if len(mediaIDs) > 0 {
		if err := q.InsertPageMedia(ctx, store.InsertPageMediaParams{PageID: pageID, MediaIds: mediaIDs}); err != nil {
			return err
		}
	}
	if oldSlug != c.in.Slug {
		if err := q.DeletePageSlugHistory(ctx, c.in.Slug); err != nil {
			return err
		}
		if oldSlug != "" {
			return q.UpsertPageSlugHistory(ctx, store.UpsertPageSlugHistoryParams{OldSlug: oldSlug, PageID: pageID})
		}
	}
	return nil
}

func mapWriteError(err error) error {
	switch store.ConstraintViolation(err) {
	case "pages_slug_key":
		return httpx.Conflict("slug 已被其他页面使用",
			httpx.FieldError{Pointer: "#/slug", Code: "duplicate", Detail: "slug 已被其他页面使用"})
	case "page_media_media_id_fkey":
		return httpx.Validation(httpx.FieldError{Pointer: "#/body_md", Code: "not_found", Detail: "引用的图片不存在"})
	}
	return err
}

func (s *Service) Create(ctx context.Context, in Input) (int64, error) {
	c, err := validate(in)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.store.InTx(ctx, func(q *store.Queries) error {
		mediaIDs, err := resolveMedia(ctx, q, c)
		if err != nil {
			return err
		}
		r := c.body.Rendered
		id, err = q.InsertPage(ctx, store.InsertPageParams{
			Slug:            in.Slug,
			Title:           in.Title,
			Description:     in.Description,
			BodyMd:          in.BodyMD,
			BodyHtml:        r.HTML,
			Toc:             c.body.TOC,
			PlainText:       r.PlainText,
			HasMath:         r.HasMath,
			RendererVersion: r.RendererVersion,
		})
		if err != nil {
			return err
		}
		return writeAssociations(ctx, q, id, c, mediaIDs, "")
	})
	return id, mapWriteError(err)
}

func lockPage(ctx context.Context, q *store.Queries, id int64, ifMatch httpx.IfMatch) (store.LockPageRow, error) {
	if err := ifMatch.Require(); err != nil {
		return store.LockPageRow{}, err
	}
	cur, err := q.LockPage(ctx, id)
	if store.IsNotFound(err) {
		return cur, httpx.NotFound("页面不存在")
	}
	if err != nil {
		return cur, err
	}
	return cur, ifMatch.Check(cur.Version, cur.UpdatedAt)
}

func (s *Service) Update(ctx context.Context, id int64, ifMatch httpx.IfMatch, in Input) error {
	if err := ifMatch.Require(); err != nil {
		return err
	}
	c, err := validate(in)
	if err != nil {
		return err
	}
	err = s.store.InTx(ctx, func(q *store.Queries) error {
		cur, err := lockPage(ctx, q, id, ifMatch)
		if err != nil {
			return err
		}
		mediaIDs, err := resolveMedia(ctx, q, c)
		if err != nil {
			return err
		}
		r := c.body.Rendered
		if err := q.UpdatePageContent(ctx, store.UpdatePageContentParams{
			ID:              id,
			Slug:            in.Slug,
			Title:           in.Title,
			Description:     in.Description,
			BodyMd:          in.BodyMD,
			BodyHtml:        r.HTML,
			Toc:             c.body.TOC,
			PlainText:       r.PlainText,
			HasMath:         r.HasMath,
			RendererVersion: r.RendererVersion,
		}); err != nil {
			return err
		}
		return writeAssociations(ctx, q, id, c, mediaIDs, cur.Slug)
	})
	return mapWriteError(err)
}

func (s *Service) Delete(ctx context.Context, id int64, ifMatch httpx.IfMatch) error {
	return s.store.InTx(ctx, func(q *store.Queries) error {
		if _, err := lockPage(ctx, q, id, ifMatch); err != nil {
			return err
		}
		return q.DeletePage(ctx, id)
	})
}

func (s *Service) Publish(ctx context.Context, id int64, ifMatch httpx.IfMatch) error {
	return s.store.InTx(ctx, func(q *store.Queries) error {
		cur, err := lockPage(ctx, q, id, ifMatch)
		if err != nil {
			return err
		}
		if !cur.HasBody {
			return httpx.Validation(httpx.FieldError{Pointer: "#/body_md", Code: "required", Detail: "正文为空，无法发布"})
		}
		if cur.Status == "published" {
			return nil
		}
		return q.SetPageStatus(ctx, store.SetPageStatusParams{ID: id, Status: "published"})
	})
}

func (s *Service) Unpublish(ctx context.Context, id int64, ifMatch httpx.IfMatch) error {
	return s.store.InTx(ctx, func(q *store.Queries) error {
		cur, err := lockPage(ctx, q, id, ifMatch)
		if err != nil {
			return err
		}
		if cur.Status == "draft" {
			return nil
		}
		return q.SetPageStatus(ctx, store.SetPageStatusParams{ID: id, Status: "draft"})
	})
}

func (s *Service) Get(ctx context.Context, id int64) (store.GetAdminPageRow, error) {
	row, err := s.store.GetAdminPage(ctx, id)
	if store.IsNotFound(err) {
		return row, httpx.NotFound("页面不存在")
	}
	return row, err
}
