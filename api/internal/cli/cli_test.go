package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"blog/api/internal/cli"
	"blog/api/internal/testkit"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.DiscardHandler))
	os.Exit(m.Run())
}

type harness struct {
	t      *testing.T
	env    map[string]string
	config string
	stdin  string
}

type result struct {
	code   int
	stdout string
	stderr string
}

func (r result) json(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(r.stdout), &m); err != nil {
		t.Fatalf("stdout is not a JSON object: %v\n%s", err, r.stdout)
	}
	return m
}

// problem 解析 stderr 中的 problem JSON。
func (r result) problem(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(r.stderr), &m); err != nil {
		t.Fatalf("stderr is not problem JSON: %v\n%s", err, r.stderr)
	}
	return m
}

func (h *harness) run(args ...string) result {
	var out, errb bytes.Buffer
	code := cli.Run(context.Background(), args, cli.Env{
		Stdin:     strings.NewReader(h.stdin),
		Stdout:    &out,
		Stderr:    &errb,
		Getenv:    func(k string) string { return h.env[k] },
		ConfigDir: h.config,
	})
	h.stdin = ""
	return result{code, out.String(), errb.String()}
}

func (h *harness) expect(code int, args ...string) result {
	h.t.Helper()
	r := h.run(args...)
	if r.code != code {
		h.t.Fatalf("blog %s: exit %d, want %d\nstdout: %s\nstderr: %s", strings.Join(args, " "), r.code, code, r.stdout, r.stderr)
	}
	return r
}

func writePNG(t *testing.T, path string, seed uint8) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for y := range 48 {
		for x := range 64 {
			img.Set(x, y, color.RGBA{uint8(x) ^ seed, uint8(y), seed, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var mediaLink = regexp.MustCompile(`/media/[0-9a-f]{64}`)

func TestPostWorkflow(t *testing.T) {
	env := testkit.New(t)
	srv := httptest.NewServer(env.App.Handler())
	t.Cleanup(srv.Close)
	writer := env.Token(t, "content:read", "content:write", "media:write")
	h := &harness{t: t, config: t.TempDir(), env: map[string]string{"BLOG_URL": srv.URL, "BLOG_TOKEN": writer}}

	h.expect(0, "tag", "create", "--name", "Go", "--slug", "go")
	h.expect(0, "series", "create", "--title", "从零搭建博客", "--slug", "blog-from-scratch")

	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "img", "topology.png"), 1)
	writePNG(t, filepath.Join(dir, "cover.png"), 2)
	draft := filepath.Join(dir, "draft.md")
	body := "## 背景\n\n![部署拓扑](./img/topology.png)\n\n```md\n![代码里的图片不处理](./missing.png)\n```\n\n正文，引用 `![行内代码](./missing.png)`。\n"
	os.WriteFile(draft, []byte("---\ntitle: 写时渲染\nslug: render-on-write\nsummary: 摘要\ntags: [go]\nseries: blog-from-scratch\ncover: ./cover.png\n---\n\n"+body), 0o644)

	dry := h.expect(0, "post", "push", draft, "--dry-run").json(t)
	if dry["method"] != "POST" || len(dry["local_images"].([]any)) != 2 || strings.Contains(readFile(t, draft), "id:") {
		t.Fatalf("dry run: %v", dry)
	}

	out := h.expect(0, "post", "push", draft).json(t)
	if out["created"] != true || len(out["uploaded"].([]any)) != 2 {
		t.Fatalf("push: %v", out)
	}
	pushed := readFile(t, draft)
	for _, want := range []string{"id: 1\n", "version: 1\n", "tags: [go]\n", "cover: /media/", "![部署拓扑](/media/", "(./missing.png)\n```", "`![行内代码](./missing.png)`"} {
		if !strings.Contains(pushed, want) {
			t.Fatalf("file after push lacks %q:\n%s", want, pushed)
		}
	}

	// 再次 push：图片已在媒体库中，不再上传；版本号前进。
	out = h.expect(0, "post", "push", draft).json(t)
	if out["created"] != false || len(out["uploaded"].([]any)) != 0 || !strings.Contains(readFile(t, draft), "version: 2\n") {
		t.Fatalf("second push: %v", out)
	}

	// pull 得到的文件与 push 写回的文件一致。
	pulled := filepath.Join(dir, "pulled.md")
	h.expect(0, "post", "pull", "render-on-write", "-o", pulled)
	if got, want := readFile(t, pulled), readFile(t, draft); got != want {
		t.Fatalf("pull differs from pushed file:\n--- pulled\n%s\n--- pushed\n%s", got, want)
	}

	// 旧版本号提交返回冲突（退出码 5）。
	os.WriteFile(pulled, []byte(strings.Replace(readFile(t, pulled), "version: 2", "version: 1", 1)), 0o644)
	if p := h.expect(5, "post", "push", pulled).problem(t); p["status"] != float64(412) {
		t.Fatalf("conflict problem: %v", p)
	}

	// 渲染错误：render 输出问题列表并以 6 退出，push 返回 422 且带行号。
	broken := filepath.Join(dir, "broken.md")
	os.WriteFile(broken, []byte("---\ntitle: 坏公式\nslug: broken\nsummary: \"\"\ntags: []\n---\n\n第一段\n\n$$\\bad$$\n![新图](./new.png)\n"), 0o644)
	writePNG(t, filepath.Join(dir, "new.png"), 3)
	rr := h.expect(6, "post", "render", broken)
	preview := rr.json(t)
	issues := preview["issues"].([]any)
	if len(issues) != 1 || issues[0].(map[string]any)["line"] != float64(3) || len(preview["local_images"].([]any)) != 1 {
		t.Fatalf("render preview: %v", preview)
	}
	p := h.expect(6, "post", "push", broken).problem(t)
	if fe := p["errors"].([]any)[0].(map[string]any); fe["code"] != "render_error" || fe["line"] != float64(3) {
		t.Fatalf("push render error: %v", p)
	}

	if list := h.expect(0, "post", "list", "--all").json(t); len(list["items"].([]any)) != 1 || list["next_cursor"] != nil {
		t.Fatalf("list: %v", list)
	}
	if got := h.expect(0, "post", "get", "render-on-write").json(t); got["series"] == nil || len(got["tags"].([]any)) != 1 {
		t.Fatalf("get: %v", got)
	}
	h.expect(4, "post", "get", "missing-slug")

	// 写入令牌没有发布与删除权限。
	if p := h.expect(3, "post", "publish", "render-on-write").problem(t); !strings.Contains(p["detail"].(string), "content:publish") {
		t.Fatalf("publish without scope: %v", p)
	}
	h.expect(2, "post", "delete", "render-on-write")

	h.env["BLOG_TOKEN"] = env.Token(t, "content:read", "content:publish", "media:write")
	if pub := h.expect(0, "post", "publish", "render-on-write").json(t); pub["state"] != "published" {
		t.Fatalf("publish: %v", pub)
	}
	if sched := h.expect(0, "post", "publish", "1", "--at", "2099-01-01T00:00:00Z").json(t); sched["state"] != "scheduled" {
		t.Fatalf("schedule: %v", sched)
	}
	h.expect(0, "post", "unpublish", "render-on-write")

	m := mediaLink.FindString(readFile(t, draft))
	h.expect(5, "media", "delete", strings.TrimPrefix(m, "/media/"), "--yes")
	h.expect(0, "post", "delete", "render-on-write", "--yes")
}

func TestPageWorkflowAndAuth(t *testing.T) {
	env := testkit.New(t)
	srv := httptest.NewServer(env.App.Handler())
	t.Cleanup(srv.Close)
	h := &harness{t: t, config: t.TempDir(), env: map[string]string{}}

	if p := h.expect(3, "page", "list").problem(t); p["type"] != "blog-cli:not-configured" {
		t.Fatalf("unconfigured: %v", p)
	}
	h.expect(2, "nope")
	h.expect(2, "page", "push")

	h.stdin = env.Token(t, "content:read", "content:write") + "\n"
	if login := h.expect(0, "auth", "login", "--url", srv.URL).json(t); login["profile"] != "default" {
		t.Fatalf("login: %v", login)
	}
	if st := h.expect(0, "auth", "status").json(t); st["me"].(map[string]any)["via"] != "token" {
		t.Fatalf("status: %v", st)
	}
	info, err := os.Stat(filepath.Join(h.config, "config.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("config file mode: %v %v", info, err)
	}

	dir := t.TempDir()
	about := filepath.Join(dir, "about.md")
	os.WriteFile(about, []byte("---\ntitle: 关于\nslug: about\ndescription: 本站简介\n---\n\n关于本站\n"), 0o644)
	h.expect(0, "page", "push", about)
	pulled := filepath.Join(dir, "pulled.md")
	h.expect(0, "page", "pull", "about", "-o", pulled)
	if readFile(t, pulled) != readFile(t, about) {
		t.Fatalf("page round trip:\n%s\n---\n%s", readFile(t, pulled), readFile(t, about))
	}
	os.WriteFile(about, []byte("---\ntitle: 关于\nslug: about\ndescripton: 拼错的字段\n---\n\n正文\n"), 0o644)
	if p := h.expect(6, "page", "push", about).problem(t); !strings.Contains(p["detail"].(string), "descripton") {
		t.Fatalf("unknown front matter field: %v", p)
	}

	var schema struct {
		Commands []struct {
			Name string `json:"name"`
		} `json:"commands"`
		ExitCodes []any `json:"exit_codes"`
	}
	if err := json.Unmarshal([]byte(h.expect(0, "schema").stdout), &schema); err != nil || len(schema.Commands) < 30 || len(schema.ExitCodes) != 8 {
		t.Fatalf("schema: %v %+v", err, schema)
	}

	h.expect(0, "auth", "logout")
	h.expect(3, "auth", "status")
}
