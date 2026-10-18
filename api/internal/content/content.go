// Package content 是文章与页面共用的写时渲染正文：保存时调用渲染服务，把原文与渲染结果一并持久化。
package content

import (
	"context"
	"encoding/json"

	"blog/api/internal/httpx"
	"blog/api/internal/media"
	"blog/api/internal/render"
	"blog/api/internal/store"
)

// MaxRequestBody 是文章与页面保存、预览请求的上限：正文 1 MB，JSON 转义最坏情况下膨胀数倍。
const MaxRequestBody = 4 << 20

const maxMarkdown = 1 << 20

type Body struct {
	Markdown        string
	HTML            string
	TOC             json.RawMessage
	PlainText       string
	WordCount       int32
	HasMath         bool
	RendererVersion string
	MediaIDs        []int64
}

type Service struct {
	store    *store.Store
	renderer *render.Client
}

func NewService(s *store.Store, r *render.Client) *Service {
	return &Service{store: s, renderer: r}
}

// Prepare 校验并渲染正文，渲染出的错误按行号累加到 v 中；v 有错误时返回值不应使用。
// 空正文不调用渲染服务，渲染器版本留空。
func (s *Service) Prepare(ctx context.Context, v *httpx.Validator, markdown string) (Body, error) {
	b := Body{Markdown: markdown, TOC: json.RawMessage("[]")}
	if len(markdown) > maxMarkdown {
		v.Add("#/body_md", "too_long", "正文不能超过 1 MB")
		return b, nil
	}
	if markdown == "" {
		return b, nil
	}
	res, ids, err := s.render(ctx, markdown)
	if err != nil {
		return b, err
	}
	for _, is := range res.Issues {
		if is.Severity == "error" {
			v.AddError(httpx.FieldError{Pointer: "#/body_md", Code: "render_error", Detail: is.Message, Line: is.Line})
		}
	}
	return newBody(markdown, res, ids), nil
}

// Preview 渲染正文供编辑器预览，警告与错误都随结果返回，不做持久化。
type Preview struct {
	HTML            string          `json:"html"`
	TOC             json.RawMessage `json:"toc"`
	HasMath         bool            `json:"has_math"`
	WordCount       int             `json:"word_count"`
	RendererVersion string          `json:"renderer_version"`
	Issues          []render.Issue  `json:"issues"`
}

func (s *Service) Preview(ctx context.Context, markdown string) (Preview, error) {
	var v httpx.Validator
	if len(markdown) > maxMarkdown {
		v.Add("#/body_md", "too_long", "正文不能超过 1 MB")
		return Preview{}, v.Err()
	}
	res, _, err := s.render(ctx, markdown)
	if err != nil {
		return Preview{}, err
	}
	return Preview{
		HTML:            res.HTML,
		TOC:             tocOrEmpty(res.TOC),
		HasMath:         res.HasMath,
		WordCount:       countWords(res.PlainText),
		RendererVersion: res.RendererVersion,
		Issues:          nonNil(res.Issues),
	}, nil
}

// RendererVersion 返回渲染服务当前的渲染器版本。
func (s *Service) RendererVersion(ctx context.Context) (string, error) {
	return s.renderer.Version(ctx)
}

// StaleAgainst 把列表的 stale 过滤条件转换为查询参数：stale 为 true 时返回当前渲染器版本，
// 查询据此筛出由其他版本渲染的内容。
func (s *Service) StaleAgainst(ctx context.Context, stale bool) (*string, error) {
	if !stale {
		return nil, nil
	}
	v, err := s.renderer.Version(ctx)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// render 查出正文引用的图片交给渲染服务，并返回其中存在于媒体库的图片 ID，用于记录引用关系。
// 不存在的图片由渲染器报告为带行号的错误。
func (s *Service) render(ctx context.Context, markdown string) (render.Result, []int64, error) {
	refs := media.ExtractRefs(markdown)
	images, ids, err := media.LoadImagesBySHA256(ctx, s.store.Queries, refs)
	if err != nil {
		return render.Result{}, nil, err
	}
	res, err := s.renderer.Render(ctx, markdown, images)
	return res, ids, err
}

func newBody(markdown string, res render.Result, ids []int64) Body {
	return Body{
		Markdown:        markdown,
		HTML:            res.HTML,
		TOC:             tocOrEmpty(res.TOC),
		PlainText:       res.PlainText,
		WordCount:       int32(countWords(res.PlainText)),
		HasMath:         res.HasMath,
		RendererVersion: res.RendererVersion,
		MediaIDs:        ids,
	}
}

func tocOrEmpty(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage("[]")
	}
	return raw
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

type TOCItem struct {
	Depth int    `json:"depth"`
	ID    string `json:"id"`
	Text  string `json:"text"`
}

func DecodeTOC(raw json.RawMessage) []TOCItem {
	out := []TOCItem{}
	_ = json.Unmarshal(raw, &out)
	return out
}
