package post

import (
	"context"
	"fmt"
	"slices"
	"time"

	"blog/api/internal/content"
	"blog/api/internal/httpx"
	"blog/api/internal/media"
	"blog/api/internal/store"
)

const maxTags = 10

type PostInput struct {
	Title        string  `json:"title"`
	Slug         string  `json:"slug"`
	Summary      string  `json:"summary"`
	BodyMD       string  `json:"body_md"`
	TagIDs       []int64 `json:"tag_ids"`
	SeriesID     *int64  `json:"series_id"`
	CoverMediaID *int64  `json:"cover_media_id"`
}

type postContent struct {
	in     PostInput
	body   content.Body
	tagIDs []int64
}

type Service struct {
	store   *store.Store
	content *content.Service
}

func NewService(s *store.Store, c *content.Service) *Service { return &Service{store: s, content: c} }

// prepare 校验字段并渲染正文，字段错误与渲染错误一并返回。渲染在事务之外进行，避免长时间持有行锁。
func (s *Service) prepare(ctx context.Context, in PostInput) (*postContent, error) {
	var v httpx.Validator
	v.Length("#/title", in.Title, 1, 200)
	v.Slug("#/slug", in.Slug, 100)
	v.Length("#/summary", in.Summary, 0, 500)
	body, err := s.content.Prepare(ctx, &v, in.BodyMD)
	if err != nil {
		return nil, err
	}
	tagIDs := slices.Compact(slices.Sorted(slices.Values(in.TagIDs)))
	if len(tagIDs) > maxTags {
		v.Add("#/tag_ids", "too_many", fmt.Sprintf("标签不能超过 %d 个", maxTags))
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	if tagIDs == nil {
		tagIDs = []int64{}
	}
	return &postContent{in: in, body: body, tagIDs: tagIDs}, nil
}

// resolveRefs 在事务内校验标签、专栏与封面是否存在。正文图片已在渲染时校验。
// joiningSeries 为 true 时锁定目标专栏行，使同一专栏的 NextSeriesPosition 串行执行。
func resolveRefs(ctx context.Context, q *store.Queries, c *postContent, joiningSeries bool) error {
	var v httpx.Validator
	if len(c.tagIDs) > 0 {
		found, err := q.ListTagIDs(ctx, c.tagIDs)
		if err != nil {
			return err
		}
		for i, id := range c.in.TagIDs {
			if !slices.Contains(found, id) {
				v.Add(fmt.Sprintf("#/tag_ids/%d", i), "not_found", "标签不存在")
			}
		}
	}
	if c.in.SeriesID != nil && joiningSeries {
		if _, err := q.LockSeries(ctx, *c.in.SeriesID); store.IsNotFound(err) {
			v.Add("#/series_id", "not_found", "专栏不存在")
		} else if err != nil {
			return err
		}
	}
	if c.in.CoverMediaID != nil {
		ok, err := q.MediaExists(ctx, *c.in.CoverMediaID)
		if err != nil {
			return err
		}
		if !ok {
			v.Add("#/cover_media_id", "not_found", "封面图片不存在")
		}
	}
	return v.Err()
}

func writeAssociations(ctx context.Context, q *store.Queries, postID int64, c *postContent, oldSlug string) error {
	if err := q.DeletePostTags(ctx, postID); err != nil {
		return err
	}
	if len(c.tagIDs) > 0 {
		if err := q.InsertPostTags(ctx, store.InsertPostTagsParams{PostID: postID, TagIds: c.tagIDs}); err != nil {
			return err
		}
	}
	if err := q.DeletePostMedia(ctx, postID); err != nil {
		return err
	}
	if len(c.body.MediaIDs) > 0 {
		if err := q.InsertPostMedia(ctx, store.InsertPostMediaParams{PostID: postID, MediaIds: c.body.MediaIDs}); err != nil {
			return err
		}
	}
	// slug 变更时记录旧 slug 供 301 跳转；新 slug 若曾是历史别名则先删除，避免别名与正式 slug 冲突。
	if oldSlug != c.in.Slug {
		if err := q.DeleteSlugHistory(ctx, c.in.Slug); err != nil {
			return err
		}
		if oldSlug != "" {
			return q.UpsertSlugHistory(ctx, store.UpsertSlugHistoryParams{OldSlug: oldSlug, PostID: postID})
		}
	}
	return nil
}

func mapWriteError(err error) error {
	switch store.ConstraintViolation(err) {
	case "":
		return err
	case "posts_slug_key":
		return httpx.Conflict("slug 已被其他文章使用",
			httpx.FieldError{Pointer: "#/slug", Code: "duplicate", Detail: "slug 已被其他文章使用"})
	case "post_tags_tag_id_fkey":
		return httpx.Validation(httpx.FieldError{Pointer: "#/tag_ids", Code: "not_found", Detail: "标签不存在"})
	case "posts_series_id_fkey":
		return httpx.Validation(httpx.FieldError{Pointer: "#/series_id", Code: "not_found", Detail: "专栏不存在"})
	case "posts_cover_media_id_fkey":
		return httpx.Validation(httpx.FieldError{Pointer: "#/cover_media_id", Code: "not_found", Detail: "封面图片不存在"})
	case "post_media_media_id_fkey":
		return httpx.Validation(httpx.FieldError{Pointer: "#/body_md", Code: "not_found", Detail: "引用的图片不存在"})
	case "posts_series_position_key":
		return httpx.Conflict("专栏序号冲突，请重试")
	default:
		return err
	}
}

func (s *Service) Create(ctx context.Context, in PostInput) (int64, error) {
	c, err := s.prepare(ctx, in)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.store.InTx(ctx, func(q *store.Queries) error {
		if err := resolveRefs(ctx, q, c, true); err != nil {
			return err
		}
		var pos *int32
		if in.SeriesID != nil {
			p, err := q.NextSeriesPosition(ctx, *in.SeriesID)
			if err != nil {
				return err
			}
			pos = &p
		}
		b := c.body
		id, err = q.InsertPost(ctx, store.InsertPostParams{
			Slug:            in.Slug,
			Title:           in.Title,
			Summary:         in.Summary,
			BodyMd:          in.BodyMD,
			BodyHtml:        b.HTML,
			Toc:             b.TOC,
			PlainText:       b.PlainText,
			WordCount:       b.WordCount,
			HasMath:         b.HasMath,
			RendererVersion: b.RendererVersion,
			CoverMediaID:    in.CoverMediaID,
			SeriesID:        in.SeriesID,
			SeriesPosition:  pos,
		})
		if err != nil {
			return err
		}
		return writeAssociations(ctx, q, id, c, "")
	})
	return id, mapWriteError(err)
}

func lockPost(ctx context.Context, q *store.Queries, id int64, ifMatch httpx.IfMatch) (store.LockPostRow, error) {
	if err := ifMatch.Require(); err != nil {
		return store.LockPostRow{}, err
	}
	cur, err := q.LockPost(ctx, id)
	if store.IsNotFound(err) {
		return cur, httpx.NotFound("文章不存在")
	}
	if err != nil {
		return cur, err
	}
	return cur, ifMatch.Check(cur.Version, cur.UpdatedAt)
}

func (s *Service) Update(ctx context.Context, id int64, ifMatch httpx.IfMatch, in PostInput) error {
	if err := ifMatch.Require(); err != nil {
		return err
	}
	c, err := s.prepare(ctx, in)
	if err != nil {
		return err
	}
	err = s.store.InTx(ctx, func(q *store.Queries) error {
		cur, err := lockPost(ctx, q, id, ifMatch)
		if err != nil {
			return err
		}
		joining := in.SeriesID != nil && (cur.SeriesID == nil || *cur.SeriesID != *in.SeriesID)
		if err := resolveRefs(ctx, q, c, joining); err != nil {
			return err
		}
		// 留在原专栏保持序号，移出专栏清空序号，加入新专栏追加到末尾。
		pos := cur.SeriesPosition
		switch {
		case in.SeriesID == nil:
			pos = nil
		case joining:
			p, err := q.NextSeriesPosition(ctx, *in.SeriesID)
			if err != nil {
				return err
			}
			pos = &p
		}
		b := c.body
		if err := q.UpdatePostContent(ctx, store.UpdatePostContentParams{
			ID:              id,
			Slug:            in.Slug,
			Title:           in.Title,
			Summary:         in.Summary,
			BodyMd:          in.BodyMD,
			BodyHtml:        b.HTML,
			Toc:             b.TOC,
			PlainText:       b.PlainText,
			WordCount:       b.WordCount,
			HasMath:         b.HasMath,
			RendererVersion: b.RendererVersion,
			CoverMediaID:    in.CoverMediaID,
			SeriesID:        in.SeriesID,
			SeriesPosition:  pos,
		}); err != nil {
			return err
		}
		return writeAssociations(ctx, q, id, c, cur.Slug)
	})
	return mapWriteError(err)
}

func (s *Service) Delete(ctx context.Context, id int64, ifMatch httpx.IfMatch) error {
	return s.store.InTx(ctx, func(q *store.Queries) error {
		if _, err := lockPost(ctx, q, id, ifMatch); err != nil {
			return err
		}
		return q.DeletePost(ctx, id)
	})
}

// Publish 发布文章。未指定 at 时沿用首次发布时间（重新发布不改变日期），从未发布过则取当前时间。
// published_at 晚于当前时间即为定时发布，状态 scheduled 由查询时推导。
func (s *Service) Publish(ctx context.Context, id int64, ifMatch httpx.IfMatch, at *time.Time) error {
	return s.store.InTx(ctx, func(q *store.Queries) error {
		cur, err := lockPost(ctx, q, id, ifMatch)
		if err != nil {
			return err
		}
		if !cur.HasBody {
			return httpx.Validation(httpx.FieldError{Pointer: "#/body_md", Code: "required", Detail: "正文为空，无法发布"})
		}
		var publishedAt time.Time
		switch {
		case at != nil:
			publishedAt = *at
		case cur.PublishedAt != nil:
			publishedAt = *cur.PublishedAt
		default:
			publishedAt = time.Now()
		}
		return q.PublishPost(ctx, store.PublishPostParams{ID: id, PublishedAt: truncateMicro(publishedAt)})
	})
}

func (s *Service) Unpublish(ctx context.Context, id int64, ifMatch httpx.IfMatch) error {
	return s.store.InTx(ctx, func(q *store.Queries) error {
		cur, err := lockPost(ctx, q, id, ifMatch)
		if err != nil {
			return err
		}
		if cur.Status == "draft" {
			return nil
		}
		return q.UnpublishPost(ctx, id)
	})
}

func (s *Service) Summary(ctx context.Context, id int64) (adminPostSummary, error) {
	row, err := s.store.GetAdminPostSummary(ctx, id)
	if store.IsNotFound(err) {
		return adminPostSummary{}, httpx.NotFound("文章不存在")
	}
	if err != nil {
		return adminPostSummary{}, err
	}
	tags, err := loadTags(ctx, s.store.Queries, []int64{id})
	if err != nil {
		return adminPostSummary{}, err
	}
	return toAdminSummary(row, tags.admin(id)), nil
}

func (s *Service) Get(ctx context.Context, id int64) (adminPost, error) {
	row, err := s.store.GetAdminPost(ctx, id)
	if store.IsNotFound(err) {
		return adminPost{}, httpx.NotFound("文章不存在")
	}
	if err != nil {
		return adminPost{}, err
	}
	tags, err := loadTags(ctx, s.store.Queries, []int64{id})
	if err != nil {
		return adminPost{}, err
	}
	images, err := media.LoadImages(ctx, s.store.Queries, derefIDs(row.CoverMediaID))
	if err != nil {
		return adminPost{}, err
	}
	return adminPost{
		adminPostSummary: toAdminSummary(store.GetAdminPostSummaryRow{
			ID: row.ID, Slug: row.Slug, Title: row.Title, State: row.State, PublishedAt: row.PublishedAt,
			RendererVersion: row.RendererVersion, SeriesID: row.SeriesID, SeriesPosition: row.SeriesPosition,
			SeriesTitle: row.SeriesTitle, Version: row.Version, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}, tags.admin(id)),
		Summary: row.Summary,
		BodyMD:  row.BodyMd,
		Cover:   imagePtr(images, row.CoverMediaID),
	}, nil
}
