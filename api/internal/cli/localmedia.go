package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Markdown 图片：![alt](dest "title")，dest 可以用尖括号包裹。
var imagePattern = regexp.MustCompile(`!\[([^\]]*)\]\(\s*(<[^>]*>|[^)\s]+)((?:\s+"[^"]*")?\s*)\)`)

var mediaRef = regexp.MustCompile(`^/media/([0-9a-f]{64})$`)

// localImage 是正文或封面引用的本地图片文件。
type localImage struct {
	Path   string `json:"path"` // 相对 Markdown 文件所在目录解析后的路径
	SHA256 string `json:"sha256"`
	ID     int64  `json:"id,omitempty"`
	alt    string
	lines  []int
}

// isLocal 判断图片地址是否指向本地文件：排除带协议的 URL、站内绝对路径与锚点。
func isLocal(dest string) bool {
	if dest == "" || strings.HasPrefix(dest, "/") || strings.HasPrefix(dest, "#") {
		return false
	}
	u, err := url.Parse(dest)
	return err == nil && u.Scheme == ""
}

// localImages 收集正文中引用的本地图片，跳过围栏代码块与行内代码，返回按出现顺序去重的列表。
func localImages(body, baseDir string) ([]*localImage, error) {
	var out []*localImage
	byPath := map[string]*localImage{}
	err := eachImage(body, func(line int, alt, dest string) {
		if !isLocal(dest) {
			return
		}
		path := filepath.Join(baseDir, filepath.FromSlash(dest))
		img, ok := byPath[path]
		if !ok {
			img = &localImage{Path: path, alt: alt}
			byPath[path] = img
			out = append(out, img)
		}
		img.lines = append(img.lines, line)
	})
	if err != nil {
		return nil, err
	}
	for _, img := range out {
		data, err := os.ReadFile(img.Path)
		if err != nil {
			return nil, invalidError("第 %d 行引用的图片无法读取：%v", img.lines[0], err)
		}
		img.SHA256 = sha256Hex(data)
	}
	return out, nil
}

// rewriteLocalImages 把本地图片地址替换为 /media/{sha256}，其余内容保持不变。
func rewriteLocalImages(body, baseDir string, images []*localImage) string {
	shaOf := map[string]string{}
	for _, img := range images {
		shaOf[img.Path] = img.SHA256
	}
	var b strings.Builder
	_ = eachLine(body, func(line int, text string, inCode bool) {
		if !inCode {
			text = replaceOutsideCode(text, func(seg string) string {
				return imagePattern.ReplaceAllStringFunc(seg, func(m string) string {
					sub := imagePattern.FindStringSubmatch(m)
					dest := strings.Trim(sub[2], "<>")
					sha, ok := shaOf[filepath.Join(baseDir, filepath.FromSlash(dest))]
					if !isLocal(dest) || !ok {
						return m
					}
					return "![" + sub[1] + "](/media/" + sha + sub[3] + ")"
				})
			})
		}
		b.WriteString(text)
	})
	return b.String()
}

// eachImage 对代码之外的每个图片引用调用 fn。
func eachImage(body string, fn func(line int, alt, dest string)) error {
	return eachLine(body, func(line int, text string, inCode bool) {
		if inCode {
			return
		}
		replaceOutsideCode(text, func(seg string) string {
			for _, m := range imagePattern.FindAllStringSubmatch(seg, -1) {
				fn(line, m[1], strings.Trim(m[2], "<>"))
			}
			return seg
		})
	})
}

// eachLine 逐行（含换行符）回调，并标出是否位于围栏代码块内。
func eachLine(body string, fn func(line int, text string, inCode bool)) error {
	var fenceMark string
	for i, text := range strings.SplitAfter(body, "\n") {
		if text == "" {
			continue
		}
		trimmed := strings.TrimLeft(text, " ")
		isFence := len(text)-len(trimmed) <= 3 && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~"))
		switch {
		case isFence && fenceMark == "":
			fenceMark = trimmed[:3]
			fn(i+1, text, true)
		case isFence && strings.HasPrefix(trimmed, fenceMark):
			fenceMark = ""
			fn(i+1, text, true)
		default:
			fn(i+1, text, fenceMark != "")
		}
	}
	return nil
}

// replaceOutsideCode 只对行内代码（反引号）之外的片段应用 fn。
func replaceOutsideCode(line string, fn func(string) string) string {
	parts := strings.Split(line, "`")
	for i := 0; i < len(parts); i += 2 {
		parts[i] = fn(parts[i])
	}
	return strings.Join(parts, "`")
}

type mediaItem struct {
	ID     int64  `json:"id"`
	SHA256 string `json:"sha256"`
}

// lookupMedia 按哈希批量查询媒体库，返回 sha256 → ID。
func (a *app) lookupMedia(ctx context.Context, shas []string) (map[string]int64, error) {
	found := map[string]int64{}
	for i := 0; i < len(shas); i += 100 {
		q := url.Values{}
		for _, s := range shas[i:min(i+100, len(shas))] {
			q.Add("sha256", s)
		}
		var page struct {
			Items []mediaItem `json:"items"`
		}
		if err := a.api.getJSON(ctx, "/api/admin/media", q, &page); err != nil {
			return nil, err
		}
		for _, m := range page.Items {
			found[m.SHA256] = m.ID
		}
	}
	return found, nil
}

// ensureUploaded 上传媒体库中还没有的本地图片，并为每张图片填上媒体 ID；返回本次实际上传的图片。
func (a *app) ensureUploaded(ctx context.Context, images []*localImage) ([]*localImage, error) {
	if len(images) == 0 {
		return nil, nil
	}
	shas := make([]string, 0, len(images))
	for _, img := range images {
		shas = append(shas, img.SHA256)
	}
	found, err := a.lookupMedia(ctx, shas)
	if err != nil {
		return nil, err
	}
	var uploaded []*localImage
	for _, img := range images {
		if id, ok := found[img.SHA256]; ok {
			img.ID = id
			continue
		}
		r, err := a.api.upload(ctx, img.Path, img.alt)
		if err != nil {
			return nil, err
		}
		var m mediaItem
		if err := r.decode(&m); err != nil {
			return nil, err
		}
		img.ID, found[img.SHA256] = m.ID, m.ID
		uploaded = append(uploaded, img)
	}
	return uploaded, nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
