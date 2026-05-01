package content

import (
	"context"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"blog/api/internal/httpx"
	"blog/api/internal/media"
	"blog/api/internal/store"
)

const MaxRequestBody = 8 << 20

const (
	maxMarkdown        = 1 << 20
	maxHTML            = 6 << 20
	maxTOCItems        = 200
	maxRendererVersion = 64
)

// Rendered 是前端渲染 Markdown 后随正文一起提交的结果。服务端不做渲染，只校验大小后原样存储。
type Rendered struct {
	HTML            string    `json:"html"`
	TOC             []TOCItem `json:"toc"`
	PlainText       string    `json:"plain_text"`
	HasMath         bool      `json:"has_math"`
	RendererVersion string    `json:"renderer_version"`
}

type TOCItem struct {
	Depth int    `json:"depth"`
	ID    string `json:"id"`
	Text  string `json:"text"`
}

type Body struct {
	Markdown  string
	Rendered  Rendered
	TOC       json.RawMessage
	WordCount int32
	MediaRefs []string
}

// Validate 校验正文与渲染结果并组装 Body，校验错误累加到 v 中；v 有错误时返回值不应使用。
func Validate(v *httpx.Validator, markdown string, r *Rendered) Body {
	if len(markdown) > maxMarkdown {
		v.Add("#/body_md", "too_long", "正文不能超过 1 MB")
	}
	if markdown != "" && r == nil {
		v.Add("#/rendered", "required", "正文非空时必须提交渲染结果")
	}
	b := Body{Markdown: markdown, TOC: json.RawMessage("[]"), MediaRefs: media.ExtractRefs(markdown)}
	if r == nil {
		return b
	}
	if len(r.HTML) > maxHTML {
		v.Add("#/rendered/html", "too_long", "渲染 HTML 不能超过 6 MB")
	}
	if len(r.TOC) > maxTOCItems {
		v.Add("#/rendered/toc", "too_many", fmt.Sprintf("目录不能超过 %d 项", maxTOCItems))
	}
	for i, item := range r.TOC {
		if item.Depth < 2 || item.Depth > 6 {
			v.Add(fmt.Sprintf("#/rendered/toc/%d/depth", i), "out_of_range", "取值范围为 2–6")
		}
	}
	if utf8.RuneCountInString(r.RendererVersion) > maxRendererVersion {
		v.Add("#/rendered/renderer_version", "too_long", fmt.Sprintf("长度不能超过 %d 个字符", maxRendererVersion))
	}
	b.Rendered = *r
	if r.TOC != nil {
		b.TOC, _ = json.Marshal(r.TOC)
	}
	b.WordCount = int32(countWords(r.PlainText))
	return b
}

// ResolveMedia 把正文引用的图片 sha256 解析为媒体 ID，不存在的引用记为校验错误。
func ResolveMedia(ctx context.Context, q *store.Queries, v *httpx.Validator, refs []string) ([]int64, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	rows, err := q.ListMediaIDsBySHA256(ctx, refs)
	if err != nil {
		return nil, err
	}
	have := make(map[string]int64, len(rows))
	for _, r := range rows {
		have[r.Sha256] = r.ID
	}
	ids := make([]int64, 0, len(refs))
	for _, sha := range refs {
		if id, ok := have[sha]; ok {
			ids = append(ids, id)
		} else {
			v.Add("#/body_md", "not_found", fmt.Sprintf("引用的图片 /media/%s 不存在", sha))
		}
	}
	return ids, nil
}

func DecodeTOC(raw json.RawMessage) []TOCItem {
	out := []TOCItem{}
	_ = json.Unmarshal(raw, &out)
	return out
}
