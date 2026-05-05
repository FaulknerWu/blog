package feed

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

var urlAttrs = map[string]bool{"href": true, "src": true, "poster": true, "srcset": true}

// absolutize 把 HTML 中以 / 开头的站内 URL 属性改写为绝对地址，因为 feed 阅读器无法解析站内相对路径。
// 未改动的标签按原始字节输出，尽量保持原 HTML 不变。
func absolutize(src, base string) string {
	z := html.NewTokenizer(strings.NewReader(src))
	var out bytes.Buffer
	out.Grow(len(src) + 256)
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			out.Write(z.Raw())
			continue
		}
		raw := append([]byte(nil), z.Raw()...)
		tok := z.Token()
		changed := false
		for i, a := range tok.Attr {
			if a.Namespace != "" || !urlAttrs[a.Key] {
				continue
			}
			var v string
			if a.Key == "srcset" {
				v = absolutizeSrcset(a.Val, base)
			} else {
				v = absolutizeURL(a.Val, base)
			}
			if v != a.Val {
				tok.Attr[i].Val = v
				changed = true
			}
		}
		if changed {
			out.WriteString(tok.String())
		} else {
			out.Write(raw)
		}
	}
	return out.String()
}

func absolutizeURL(u, base string) string {
	if strings.HasPrefix(u, "/") && !strings.HasPrefix(u, "//") {
		return base + u
	}
	return u
}

func absolutizeSrcset(v, base string) string {
	parts := strings.Split(v, ",")
	for i, p := range parts {
		trimmed := strings.TrimSpace(p)
		url, desc, _ := strings.Cut(trimmed, " ")
		abs := absolutizeURL(url, base)
		if abs == url {
			continue
		}
		if desc != "" {
			abs += " " + desc
		}
		if i > 0 {
			abs = " " + abs
		}
		parts[i] = abs
	}
	return strings.Join(parts, ",")
}
