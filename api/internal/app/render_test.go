package app_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"blog/api/internal/content"
	"blog/api/internal/render"
	"blog/api/internal/testkit"
)

func TestRenderPreviewAndRerender(t *testing.T) {
	env := testkit.New(t)
	c := &client{t: t, h: env.App.Handler()}
	c.login()

	if v := c.do("GET", "/api/admin/render", nil).expect(200).json()["renderer_version"]; v != "test.1" {
		t.Fatalf("renderer version = %v", v)
	}
	pv := c.do("POST", "/api/admin/render", map[string]string{"body_md": "## 小节\n\n正文 text\n\n$$\\bad$$"}).expect(200).json()
	issues := pv["issues"].([]any)
	if len(issues) != 1 || num(issues[0].(map[string]any)["line"]) != 5 || num(pv["word_count"]) != 5 ||
		len(pv["toc"].([]any)) != 1 || pv["has_math"] != true {
		t.Fatalf("preview: %v", pv)
	}
	c.do("POST", "/api/admin/render", `{"body_md":"x","rendered":{}}`).expect(400)

	p := c.do("POST", "/api/admin/posts", postInput("旧文章", "old-post", "旧渲染器的正文", nil)).expect(201).json()
	postPath := fmt.Sprintf("/api/admin/posts/%d", num(p["id"]))
	broken := c.do("POST", "/api/admin/pages", map[string]any{"title": "页面", "slug": "broken", "description": "", "body_md": "正文"}).expect(201).json()
	c.do("POST", "/api/admin/posts", postInput("空文章", "empty", "", nil)).expect(201)

	// 升级渲染器后，有正文的内容都变为过期；空正文没有渲染结果，不算过期。
	env.Render.SetVersion("test.2")
	c.do("POST", "/api/admin/render", map[string]string{"body_md": "刷新版本缓存"}).expect(200)
	if stale := c.do("GET", "/api/admin/posts?stale=true", nil).expect(200).json()["items"].([]any); len(stale) != 1 {
		t.Fatalf("stale posts: %v", stale)
	}
	if stale := c.do("GET", "/api/admin/pages?stale=true", nil).expect(200).json()["items"].([]any); len(stale) != 1 {
		t.Fatalf("stale pages: %v", stale)
	}
	// 让页面在新渲染器下出错：直接改库里的原文，模拟渲染规则变严格。
	if _, err := env.Store.Pool.Exec(context.Background(), `UPDATE pages SET body_md = '$$\bad$$' WHERE id = $1`, num(broken["id"])); err != nil {
		t.Fatal(err)
	}

	report, err := content.NewService(env.Store, render.NewClient(env.Render.URL)).Rerender(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.RendererVersion != "test.2" || report.Posts != 1 || report.Pages != 0 || report.Failed != 1 {
		t.Fatalf("rerender report: %+v", report)
	}
	got := c.do("GET", postPath, nil).expect(200)
	if got.Header().Get("ETag") != `"v2"` || got.json()["renderer_version"] != "test.2" {
		t.Fatalf("rerendered post: %v %s", got.Header(), got.Body)
	}
	if stale := c.do("GET", "/api/admin/posts?stale=true", nil).expect(200).json()["items"].([]any); len(stale) != 0 {
		t.Fatalf("posts still stale: %v", stale)
	}
}

func TestRenderServiceUnavailable(t *testing.T) {
	env := testkit.New(t)
	env.Render.Close()
	c := &client{t: t, h: env.App.Handler()}
	c.login()
	r := c.do("POST", "/api/admin/posts", postInput("标题", "slug", "正文", nil)).expect(503)
	if r.problemType() != "service-unavailable" {
		t.Fatalf("problem: %s", r.Body)
	}
	// 空正文不需要渲染服务。
	c.do("POST", "/api/admin/posts", postInput("草稿", "draft", "", nil)).expect(201)
	c.do("GET", "/api/admin/posts", nil).expect(200)
	c.do("GET", "/api/admin/posts?stale=true", nil).expect(503)
	if strings.Contains(c.do("GET", "/healthz", nil).Body.String(), "unavailable") {
		t.Fatal("API health should not depend on the render service")
	}
}
