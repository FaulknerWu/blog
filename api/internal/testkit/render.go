package testkit

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// FakeRenderer 按行模拟渲染服务的协议：## 标题生成目录，媒体图片展开为带 srcset 的 <img>，
// 含 \bad 的行报告公式错误，[文字](链接) 转为 <a>，其余行作为段落。
type FakeRenderer struct {
	*httptest.Server
	mu      sync.Mutex
	version string
	calls   int
}

var (
	fakeImage = regexp.MustCompile(`^!\[(.*?)\]\(/media/([0-9a-f]{64})\)$`)
	fakeLink  = regexp.MustCompile(`\[(.+?)\]\((\S+?)\)`)
)

func NewFakeRenderer(t testing.TB) *FakeRenderer {
	f := &FakeRenderer{version: "test.1"}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	return f
}

// SetVersion 模拟渲染服务升级到新的渲染器版本。
func (f *FakeRenderer) SetVersion(v string) {
	f.mu.Lock()
	f.version = v
	f.mu.Unlock()
}

// Calls 返回 /render 被调用的次数。
func (f *FakeRenderer) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type fakeVariant struct {
	Width int    `json:"width"`
	URL   string `json:"url"`
}

type fakeImageInfo struct {
	Src      string        `json:"src"`
	Width    int           `json:"width"`
	Height   int           `json:"height"`
	Variants []fakeVariant `json:"variants"`
}

func (f *FakeRenderer) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	version := f.version
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "renderer_version": version})
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != "/render" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	var in struct {
		Markdown string                   `json:"markdown"`
		Media    map[string]fakeImageInfo `json:"media"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var out, plain strings.Builder
	toc := []map[string]any{}
	issues := []map[string]any{}
	for i, line := range strings.Split(in.Markdown, "\n") {
		n := i + 1
		switch {
		case strings.TrimSpace(line) == "":
		case strings.HasPrefix(line, "## "):
			text := strings.TrimPrefix(line, "## ")
			toc = append(toc, map[string]any{"depth": 2, "id": text, "text": text})
			fmt.Fprintf(&out, `<h2 id="%s">%s</h2>`, html.EscapeString(text), html.EscapeString(text))
			plain.WriteString(text + "\n")
		case fakeImage.MatchString(line):
			m := fakeImage.FindStringSubmatch(line)
			img, ok := in.Media[m[2]]
			if !ok {
				issues = append(issues, map[string]any{"line": n, "message": "媒体库中没有图片", "severity": "error"})
				continue
			}
			var srcset []string
			for _, v := range img.Variants {
				srcset = append(srcset, fmt.Sprintf("%s %dw", v.URL, v.Width))
			}
			fmt.Fprintf(&out, `<p><img src="%s" srcset="%s" alt="%s" width="%d" height="%d"></p>`,
				img.Src, strings.Join(srcset, ", "), html.EscapeString(m[1]), img.Width, img.Height)
		case strings.Contains(line, `\bad`):
			issues = append(issues, map[string]any{"line": n, "message": "公式无法编译：未知命令 \\bad", "severity": "error"})
		default:
			body := fakeLink.ReplaceAllString(html.EscapeString(line), `<a href="$2">$1</a>`)
			out.WriteString("<p>" + body + "</p>")
			plain.WriteString(fakeLink.ReplaceAllString(line, "$1") + "\n")
		}
	}
	json.NewEncoder(w).Encode(map[string]any{
		"html":             out.String(),
		"toc":              toc,
		"plain_text":       strings.TrimSpace(plain.String()),
		"has_math":         strings.Contains(in.Markdown, "$"),
		"renderer_version": version,
		"issues":           issues,
	})
}
