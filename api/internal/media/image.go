package media

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"blog/api/internal/store"
)

type Variant struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Format string `json:"format"`
	Size   int64  `json:"size"`
}

type VariantJSON struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Format string `json:"format"`
	URL    string `json:"url"`
}

type Image struct {
	Sha256   string        `json:"sha256"`
	Width    int32         `json:"width"`
	Height   int32         `json:"height"`
	Alt      string        `json:"alt"`
	Src      string        `json:"src"`
	Variants []VariantJSON `json:"variants"`
}

const DefaultMaxWidth = 1600

var (
	sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	refPattern    = regexp.MustCompile(`/media/([0-9a-f]{64})\b`)
)

func srcURL(sha string) string { return "/media/" + sha }

func variantURL(sha string, v Variant) string {
	return fmt.Sprintf("/media/%s/%d.%s", sha, v.Width, v.Format)
}

func decodeVariants(raw json.RawMessage) []Variant {
	var vs []Variant
	_ = json.Unmarshal(raw, &vs)
	return vs
}

func variantsJSON(sha string, vs []Variant) []VariantJSON {
	out := make([]VariantJSON, 0, len(vs))
	for _, v := range vs {
		out = append(out, VariantJSON{Width: v.Width, Height: v.Height, Format: v.Format, URL: variantURL(sha, v)})
	}
	return out
}

// defaultVariant 选出 /media/{sha256} 默认返回的变体：不超过 DefaultMaxWidth 的最大一档，都超过时取最小的。
func defaultVariant(vs []Variant) (Variant, bool) {
	var best, smallest Variant
	var found bool
	for i, v := range vs {
		if i == 0 || v.Width < smallest.Width {
			smallest = v
		}
		if v.Width <= DefaultMaxWidth && v.Width > best.Width {
			best, found = v, true
		}
	}
	if !found {
		return smallest, len(vs) > 0
	}
	return best, true
}

func newImage(sha string, width, height int32, alt string, variants json.RawMessage) Image {
	return Image{
		Sha256:   sha,
		Width:    width,
		Height:   height,
		Alt:      alt,
		Src:      srcURL(sha),
		Variants: variantsJSON(sha, decodeVariants(variants)),
	}
}

func LoadImages(ctx context.Context, q *store.Queries, ids []int64) (map[int64]Image, error) {
	out := make(map[int64]Image, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := q.ListImages(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = newImage(r.Sha256, r.Width, r.Height, r.Alt, r.Variants)
	}
	return out, nil
}

// LoadImagesBySHA256 按哈希查询图片，返回以 sha256 为键的图片信息及其中存在的媒体 ID（按 shas 顺序）。
func LoadImagesBySHA256(ctx context.Context, q *store.Queries, shas []string) (map[string]Image, []int64, error) {
	images := make(map[string]Image, len(shas))
	if len(shas) == 0 {
		return images, nil, nil
	}
	rows, err := q.ListImagesBySHA256(ctx, shas)
	if err != nil {
		return nil, nil, err
	}
	idOf := make(map[string]int64, len(rows))
	for _, r := range rows {
		images[r.Sha256] = newImage(r.Sha256, r.Width, r.Height, r.Alt, r.Variants)
		idOf[r.Sha256] = r.ID
	}
	ids := make([]int64, 0, len(rows))
	for _, sha := range shas {
		if id, ok := idOf[sha]; ok {
			ids = append(ids, id)
		}
	}
	return images, ids, nil
}

// ExtractRefs 按出现顺序提取正文中去重后的 /media/{sha256} 引用。
// 引用关系写入 post_media/page_media，用于阻止删除仍被引用的图片。
func ExtractRefs(body string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range refPattern.FindAllStringSubmatch(body, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}
