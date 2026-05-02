package post

import (
	"context"
	"time"

	"blog/api/internal/content"
	"blog/api/internal/httpx"
	"blog/api/internal/media"
	"blog/api/internal/store"
)

type tagRef struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type postRef struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

type seriesRef struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

type postSummary struct {
	ID          int64        `json:"id"`
	Slug        string       `json:"slug"`
	Title       string       `json:"title"`
	Summary     string       `json:"summary"`
	WordCount   int32        `json:"word_count"`
	PublishedAt httpx.Time   `json:"published_at"`
	UpdatedAt   httpx.Time   `json:"updated_at"`
	Tags        []tagRef     `json:"tags"`
	Series      *seriesRef   `json:"series"`
	Cover       *media.Image `json:"cover"`
}

type seriesNav struct {
	Slug  string   `json:"slug"`
	Title string   `json:"title"`
	Index int      `json:"index"`
	Total int      `json:"total"`
	Prev  *postRef `json:"prev"`
	Next  *postRef `json:"next"`
}

type adjacent struct {
	Newer *postRef `json:"newer"`
	Older *postRef `json:"older"`
}

type postDetail struct {
	ID          int64             `json:"id"`
	Slug        string            `json:"slug"`
	Title       string            `json:"title"`
	Summary     string            `json:"summary"`
	HTML        string            `json:"html"`
	TOC         []content.TOCItem `json:"toc"`
	HasMath     bool              `json:"has_math"`
	WordCount   int32             `json:"word_count"`
	PublishedAt httpx.Time        `json:"published_at"`
	UpdatedAt   httpx.Time        `json:"updated_at"`
	Tags        []tagRef          `json:"tags"`
	Cover       *media.Image      `json:"cover"`
	Series      *seriesNav        `json:"series"`
	Adjacent    adjacent          `json:"adjacent"`
}

type adminTagRef struct {
	ID   int64  `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type adminSeriesRef struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Position int32  `json:"position"`
}

type adminPostSummary struct {
	ID              int64           `json:"id"`
	Slug            string          `json:"slug"`
	Title           string          `json:"title"`
	State           string          `json:"state"`
	PublishedAt     *httpx.Time     `json:"published_at"`
	RendererVersion string          `json:"renderer_version"`
	Tags            []adminTagRef   `json:"tags"`
	Series          *adminSeriesRef `json:"series"`
	Version         int32           `json:"version"`
	CreatedAt       httpx.Time      `json:"created_at"`
	UpdatedAt       httpx.Time      `json:"updated_at"`
}

type adminPost struct {
	adminPostSummary
	Summary string       `json:"summary"`
	BodyMD  string       `json:"body_md"`
	Cover   *media.Image `json:"cover"`
}

func adminSeries(id *int64, title *string, pos *int32) *adminSeriesRef {
	if id == nil || title == nil || pos == nil {
		return nil
	}
	return &adminSeriesRef{ID: *id, Title: *title, Position: *pos}
}

func toAdminSummary(r store.GetAdminPostSummaryRow, tags []adminTagRef) adminPostSummary {
	if tags == nil {
		tags = []adminTagRef{}
	}
	return adminPostSummary{
		ID:              r.ID,
		Slug:            r.Slug,
		Title:           r.Title,
		State:           r.State,
		PublishedAt:     httpx.NewTimePtr(r.PublishedAt),
		RendererVersion: r.RendererVersion,
		Tags:            tags,
		Series:          adminSeries(r.SeriesID, r.SeriesTitle, r.SeriesPosition),
		Version:         r.Version,
		CreatedAt:       httpx.NewTime(r.CreatedAt),
		UpdatedAt:       httpx.NewTime(r.UpdatedAt),
	}
}

type postTags map[int64][]store.ListTagsForPostsRow

func loadTags(ctx context.Context, q *store.Queries, ids []int64) (postTags, error) {
	out := postTags{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := q.ListTagsForPosts(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.PostID] = append(out[r.PostID], r)
	}
	return out, nil
}

func (t postTags) public(id int64) []tagRef {
	out := make([]tagRef, 0, len(t[id]))
	for _, r := range t[id] {
		out = append(out, tagRef{Slug: r.Slug, Name: r.Name})
	}
	return out
}

func (t postTags) admin(id int64) []adminTagRef {
	out := make([]adminTagRef, 0, len(t[id]))
	for _, r := range t[id] {
		out = append(out, adminTagRef{ID: r.ID, Slug: r.Slug, Name: r.Name})
	}
	return out
}

func imagePtr(images map[int64]media.Image, id *int64) *media.Image {
	if id == nil {
		return nil
	}
	if img, ok := images[*id]; ok {
		return &img
	}
	return nil
}

func derefIDs(ids ...*int64) []int64 {
	var out []int64
	for _, id := range ids {
		if id != nil {
			out = append(out, *id)
		}
	}
	return out
}

// truncateMicro 对齐 PostgreSQL 的微秒精度，避免写入后读回的时间不相等。
func truncateMicro(t time.Time) time.Time { return t.UTC().Truncate(time.Microsecond) }
