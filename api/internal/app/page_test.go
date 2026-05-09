package app_test

import (
	"fmt"
	"strings"
	"testing"
)

func pageInput(title, slug, body string, extra map[string]any) map[string]any {
	in := map[string]any{
		"title":       title,
		"slug":        slug,
		"description": "描述：" + title,
		"body_md":     body,
		"rendered": map[string]any{
			"html":             "<p>" + body + "</p>",
			"toc":              []map[string]any{},
			"plain_text":       body,
			"has_math":         false,
			"renderer_version": "2026.10.0",
		},
	}
	for k, v := range extra {
		in[k] = v
	}
	return in
}

func TestPageLifecycle(t *testing.T) {
	a, _ := newTestApp(t)
	c := &client{t: t, h: a.Handler()}
	c.login()

	r := c.do("POST", "/api/admin/pages", pageInput("关于", "posts", "正文", nil)).expect(422)
	if !strings.Contains(r.Body.String(), `"code":"reserved"`) {
		t.Fatalf("reserved slug should be rejected: %s", r.Body)
	}
	missing := strings.Repeat("b", 64)
	c.do("POST", "/api/admin/pages", pageInput("关于", "about", "![x](/media/"+missing+")", nil)).expect(422)
	c.do("POST", "/api/admin/pages", pageInput("关于", "about", "正文", map[string]any{"tag_ids": []int64{}})).expect(400)

	sha := c.upload("avatar.png", pngImage(t, 600, 400, 3), "头像").expect(201).json()["sha256"].(string)
	mediaID := num(c.do("GET", "/api/admin/media?sha256="+sha, nil).expect(200).json()["items"].([]any)[0].(map[string]any)["id"])

	body := "![头像](/media/" + sha + ")\n\n关于本站的独特介绍"
	r = c.do("POST", "/api/admin/pages", pageInput("关于", "about", body, nil)).expect(201)
	p := r.json()
	if p["state"] != "draft" || r.Header().Get("ETag") != `"v1"` || r.Header().Get("Location") != fmt.Sprintf("/api/admin/pages/%d", num(p["id"])) {
		t.Fatalf("create page: %v %v", p, r.Header())
	}
	pagePath := fmt.Sprintf("/api/admin/pages/%d", num(p["id"]))
	c.do("POST", "/api/admin/pages", pageInput("另一个", "about", "正文", nil)).expect(409)
	empty := c.do("POST", "/api/admin/pages", pageInput("空页面", "empty", "", map[string]any{"rendered": nil})).expect(201).json()

	c.do("GET", "/api/pages/about", nil).expect(404)
	c.do("PUT", pagePath+"/publication", nil).expect(428)
	c.do("PUT", fmt.Sprintf("/api/admin/pages/%d/publication", num(empty["id"])), nil, hdr{"If-Match": `"v1"`}).expect(422)
	c.do("PUT", pagePath+"/publication", map[string]string{"published_at": "2030-01-01T00:00:00Z"}, hdr{"If-Match": `"v1"`}).expect(400)
	r = c.do("PUT", pagePath+"/publication", nil, hdr{"If-Match": `"v1"`}).expect(200)
	if r.json()["state"] != "published" || r.Header().Get("ETag") != `"v2"` {
		t.Fatalf("publish page: %s %v", r.Body, r.Header())
	}
	if r = c.do("PUT", pagePath+"/publication", nil, hdr{"If-Match": `"v2"`}).expect(200); r.Header().Get("ETag") != `"v2"` {
		t.Fatalf("republish should be a no-op: %v", r.Header())
	}

	pub := c.do("GET", "/api/pages/about", nil).expect(200).json()
	if pub["title"] != "关于" || pub["description"] != "描述：关于" || !strings.Contains(pub["html"].(string), body) {
		t.Fatalf("public page: %v", pub)
	}
	for _, k := range []string{"published_at", "tags", "series", "adjacent", "cover"} {
		if _, ok := pub[k]; ok {
			t.Errorf("public page should not expose %q", k)
		}
	}

	if items := c.do("GET", "/api/posts", nil).expect(200).json()["items"].([]any); len(items) != 0 {
		t.Fatalf("page leaked into post list: %v", items)
	}
	if items := c.do("GET", "/api/search?q="+urlQuery("独特介绍"), nil).expect(200).json()["items"].([]any); len(items) != 0 {
		t.Fatalf("page leaked into search: %v", items)
	}
	if feed := c.do("GET", "/feed.xml", nil).expect(200).Body.String(); strings.Contains(feed, "about") {
		t.Fatalf("page leaked into feed: %s", feed)
	}
	if sm := c.do("GET", "/sitemap.xml", nil).expect(200).Body.String(); !strings.Contains(sm, "<loc>https://blog.example.com/about</loc>") {
		t.Fatalf("sitemap missing page: %s", sm)
	}

	mp := fmt.Sprintf("/api/admin/media/%d", mediaID)
	detail := c.do("GET", mp, nil).expect(200).json()
	refs := detail["referenced_by"].([]any)
	if num(detail["reference_count"]) != 1 || len(refs) != 1 || refs[0].(map[string]any)["type"] != "page" {
		t.Fatalf("media references: %v", detail)
	}
	if unused := c.do("GET", "/api/admin/media?unused=true", nil).expect(200).json()["items"].([]any); len(unused) != 0 {
		t.Fatalf("referenced media listed as unused: %v", unused)
	}
	c.do("DELETE", mp, nil).expect(409)

	c.do("PUT", pagePath, pageInput("关于我", "about-me", body, nil), hdr{"If-Match": `"v1"`}).expect(412)
	r = c.do("PUT", pagePath, pageInput("关于我", "about-me", body, nil), hdr{"If-Match": `"v2"`}).expect(200)
	if r.Header().Get("ETag") != `"v3"` {
		t.Fatalf("update page: %v", r.Header())
	}
	if loc := c.do("GET", "/api/pages/about", nil).expect(301).Header().Get("Location"); loc != "/api/pages/about-me" {
		t.Fatalf("redirect location %q", loc)
	}
	ap := c.do("GET", pagePath, nil).expect(200).json()
	if ap["slug"] != "about-me" || ap["body_md"] != body || ap["description"] != "描述：关于我" {
		t.Fatalf("admin page: %v", ap)
	}
	list := c.do("GET", "/api/admin/pages", nil).expect(200).json()["items"].([]any)
	if len(list) != 2 {
		t.Fatalf("admin pages: %v", list)
	}
	if stale := c.do("GET", "/api/admin/pages?renderer_version_not=2026.10.0", nil).expect(200).json()["items"].([]any); len(stale) != 1 {
		t.Fatalf("renderer_version_not should match only the page without rendered output: %v", stale)
	}

	c.do("DELETE", pagePath+"/publication", nil, hdr{"If-Match": `"v3"`}).expect(200)
	c.do("GET", "/api/pages/about-me", nil).expect(404)
	c.do("GET", "/api/pages/about", nil).expect(404)
	if sm := c.do("GET", "/sitemap.xml", nil).expect(200).Body.String(); strings.Contains(sm, "about-me") {
		t.Fatalf("unpublished page in sitemap: %s", sm)
	}

	c.do("DELETE", pagePath, nil).expect(428)
	c.do("DELETE", pagePath, nil, hdr{"If-Match": `"v4"`}).expect(204)
	c.do("GET", pagePath, nil).expect(404)
	c.do("DELETE", mp, nil, hdr{"If-Match": `"v1"`}).expect(204)
}
