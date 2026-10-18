package content

import (
	"context"
	"fmt"
	"log/slog"

	"blog/api/internal/store"
)

type RerenderReport struct {
	RendererVersion string `json:"renderer_version"`
	Posts           int    `json:"posts"`
	Pages           int    `json:"pages"`
	Failed          int    `json:"failed"`
}

// staleItem 是由旧版渲染器生成、待重新渲染的文章或页面。
type staleItem struct {
	kind    string
	id      int64
	body    string
	version int32
}

// Rerender 用当前渲染器重新渲染所有过期的文章与页面。原文不变，渲染结果变化会递增版本号，
// 让公开页面的 ETag 随之失效。渲染出错的条目保持原样并计入 Failed，留待作者在编辑器中修正。
func (s *Service) Rerender(ctx context.Context) (RerenderReport, error) {
	current, err := s.renderer.Version(ctx)
	if err != nil {
		return RerenderReport{}, err
	}
	report := RerenderReport{RendererVersion: current}

	posts, err := s.store.ListStalePosts(ctx, current)
	if err != nil {
		return report, err
	}
	pages, err := s.store.ListStalePages(ctx, current)
	if err != nil {
		return report, err
	}
	items := make([]staleItem, 0, len(posts)+len(pages))
	for _, p := range posts {
		items = append(items, staleItem{"post", p.ID, p.BodyMd, p.Version})
	}
	for _, p := range pages {
		items = append(items, staleItem{"page", p.ID, p.BodyMd, p.Version})
	}

	for _, it := range items {
		updated, err := s.rerenderOne(ctx, it)
		if err != nil {
			if ctx.Err() != nil {
				return report, ctx.Err()
			}
			slog.WarnContext(ctx, "rerender failed", "kind", it.kind, "id", it.id, "err", err)
			report.Failed++
			continue
		}
		switch {
		case !updated:
		case it.kind == "post":
			report.Posts++
		default:
			report.Pages++
		}
	}
	return report, nil
}

// rerenderOne 渲染并按版本号条件更新；期间被编辑过（版本已变）时放弃，编辑保存时已经用新渲染器渲染过。
func (s *Service) rerenderOne(ctx context.Context, it staleItem) (bool, error) {
	res, _, err := s.render(ctx, it.body)
	if err != nil {
		return false, err
	}
	for _, is := range res.Issues {
		if is.Severity == "error" {
			return false, &renderError{line: is.Line, message: is.Message}
		}
	}
	b := newBody(it.body, res, nil)
	var n int64
	if it.kind == "post" {
		n, err = s.store.UpdatePostRendered(ctx, store.UpdatePostRenderedParams{
			ID: it.id, Version: it.version, BodyHtml: b.HTML, Toc: b.TOC, PlainText: b.PlainText,
			WordCount: b.WordCount, HasMath: b.HasMath, RendererVersion: b.RendererVersion,
		})
	} else {
		n, err = s.store.UpdatePageRendered(ctx, store.UpdatePageRenderedParams{
			ID: it.id, Version: it.version, BodyHtml: b.HTML, Toc: b.TOC, PlainText: b.PlainText,
			HasMath: b.HasMath, RendererVersion: b.RendererVersion,
		})
	}
	return n > 0, err
}

type renderError struct {
	line    int
	message string
}

func (e *renderError) Error() string { return fmt.Sprintf("line %d: %s", e.line, e.message) }
