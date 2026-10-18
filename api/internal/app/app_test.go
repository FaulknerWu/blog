package app_test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strings"
	"testing"
	"time"
)

func pngImage(t *testing.T, w, h int, seed uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x) ^ seed, uint8(y), seed, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// postInput 构造文章保存请求；非空正文前加一个二级标题，使渲染结果带一项目录。
func postInput(title, slug, body string, extra map[string]any) map[string]any {
	if body != "" {
		body = "## 背景\n\n" + body
	}
	in := map[string]any{
		"title":          title,
		"slug":           slug,
		"summary":        "摘要：" + title,
		"body_md":        body,
		"tag_ids":        []int64{},
		"series_id":      nil,
		"cover_media_id": nil,
	}
	for k, v := range extra {
		in[k] = v
	}
	return in
}

func num(v any) int64 { return int64(v.(float64)) }

func TestHealthAndProblems(t *testing.T) {
	a, _ := newTestApp(t)
	c := &client{t: t, h: a.Handler()}

	if got := c.do("GET", "/healthz", nil).expect(200).json()["status"]; got != "ok" {
		t.Fatalf("healthz status = %v", got)
	}
	r := c.do("GET", "/api/nope", nil).expect(404)
	if r.problemType() != "not-found" || r.json()["request_id"] == "" {
		t.Fatalf("unexpected problem: %s", r.Body)
	}
	r = c.do("PATCH", "/api/posts", "{}").expect(405)
	if r.problemType() != "method-not-allowed" || r.Header().Get("Allow") == "" {
		t.Fatalf("unexpected 405: %v %s", r.Header(), r.Body)
	}
	if c.do("GET", "/api/posts?cursor=bm90LWpzb24", nil).expect(400).problemType() != "malformed-request" {
		t.Fatal("bad cursor should be malformed-request")
	}
	if c.do("GET", "/api/posts?limit=51", nil).expect(422).problemType() != "validation-error" {
		t.Fatal("limit over max should be validation-error")
	}
	if c.do("GET", "/api/admin/posts", nil).expect(401).problemType() != "unauthenticated" {
		t.Fatal("admin without session should be 401")
	}
	c.do("POST", "/api/auth/sessions", map[string]string{"username": testUser, "password": testPassword},
		hdr{"Sec-Fetch-Site": "cross-site"}).expect(403)
	c.do("POST", "/api/auth/sessions", map[string]string{"username": testUser, "password": testPassword},
		hdr{"Origin": "https://evil.example.com"}).expect(403)
}

func TestAuthSessions(t *testing.T) {
	a, _ := newTestApp(t)
	h := a.Handler()
	c := &client{t: t, h: h}

	r := c.do("POST", "/api/auth/sessions", map[string]string{"username": testUser, "password": "wrong-password"}).expect(401)
	if r.problemType() != "unauthenticated" {
		t.Fatal("wrong password should be unauthenticated")
	}
	c.do("POST", "/api/auth/sessions", `{"username":"author","password":"x","extra":1}`).expect(400)

	r = c.do("POST", "/api/auth/sessions", map[string]string{"username": testUser, "password": testPassword}).expect(201)
	setCookie := r.Header().Get("Set-Cookie")
	for _, want := range []string{"__Host-session=", "Path=/", "Secure", "HttpOnly", "SameSite=Strict", "Max-Age=2592000"} {
		if !strings.Contains(setCookie, want) {
			t.Errorf("Set-Cookie %q missing %q", setCookie, want)
		}
	}
	body := r.json()
	if body["user"].(map[string]any)["username"] != testUser {
		t.Fatalf("login body: %v", body)
	}
	if r.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("auth Cache-Control = %q", r.Header().Get("Cache-Control"))
	}

	me := c.do("GET", "/api/auth/me", nil).expect(200).json()
	if me["display_name"] != "Author" {
		t.Fatalf("me = %v", me)
	}

	other := &client{t: t, h: h}
	other.login()
	list := c.do("GET", "/api/auth/sessions", nil).expect(200).json()["items"].([]any)
	if len(list) != 2 {
		t.Fatalf("sessions = %d, want 2", len(list))
	}
	var currentCount int
	for _, it := range list {
		if it.(map[string]any)["current"] == true {
			currentCount++
		}
	}
	if currentCount != 1 {
		t.Fatalf("expected exactly one current session: %v", list)
	}

	c.do("PUT", "/api/auth/password", map[string]string{"current_password": "nope", "new_password": "a-long-enough-password"}).expect(422)
	c.do("PUT", "/api/auth/password", map[string]string{"current_password": testPassword, "new_password": "short"}).expect(422)
	c.do("PUT", "/api/auth/password", map[string]string{"current_password": testPassword, "new_password": "a-long-enough-password"}).expect(204)
	other.do("GET", "/api/auth/me", nil).expect(401)
	c.do("GET", "/api/auth/me", nil).expect(200)

	c.do("DELETE", "/api/auth/sessions/999999", nil).expect(404)
	c.do("DELETE", "/api/auth/sessions/current", nil).expect(204)
	c.do("GET", "/api/auth/me", nil).expect(401)
}

func TestPostLifecycle(t *testing.T) {
	a, _ := newTestApp(t)
	c := &client{t: t, h: a.Handler()}
	c.login()

	tag := c.do("POST", "/api/admin/tags", map[string]string{"name": "PostgreSQL", "slug": "postgresql"}).expect(201).json()
	tagID := num(tag["id"])
	if r := c.do("POST", "/api/admin/tags", map[string]string{"name": "postgresql", "slug": "pg"}).expect(409); r.problemType() != "conflict" {
		t.Fatal("case-insensitive duplicate tag name should conflict")
	}
	series := c.do("POST", "/api/admin/series", map[string]string{"title": "从零搭建博客", "slug": "blog-from-scratch"}).expect(201).json()
	seriesID := num(series["id"])

	bad := postInput("", "Bad Slug", "正文", map[string]any{"tag_ids": []int64{tagID, 9999}})
	r := c.do("POST", "/api/admin/posts", bad).expect(422)
	errs := r.json()["errors"].([]any)
	pointers := map[string]bool{}
	for _, e := range errs {
		pointers[e.(map[string]any)["pointer"].(string)] = true
	}
	if !pointers["#/title"] || !pointers["#/slug"] {
		t.Fatalf("expected title and slug errors, got %v", errs)
	}
	r = c.do("POST", "/api/admin/posts", postInput("标题", "ok-slug", "正文", map[string]any{"tag_ids": []int64{tagID, 9999}})).expect(422)
	if !strings.Contains(r.Body.String(), "#/tag_ids/1") {
		t.Fatalf("expected #/tag_ids/1 not_found: %s", r.Body)
	}
	clientRendered := postInput("标题", "ok-slug", "正文", map[string]any{"rendered": map[string]any{"html": "<script>"}})
	c.do("POST", "/api/admin/posts", clientRendered).expect(400)
	r = c.do("POST", "/api/admin/posts", postInput("标题", "ok-slug", "第一段\n\n$$\\bad$$", nil)).expect(422)
	if fe := r.json()["errors"].([]any)[0].(map[string]any); fe["pointer"] != "#/body_md" || fe["code"] != "render_error" || num(fe["line"]) != 5 {
		t.Fatalf("render error should point at the Markdown line: %s", r.Body)
	}
	c.do("POST", "/api/admin/posts", `{"title":"x","slug":"x","unknown":true}`).expect(400)
	c.do("POST", "/api/admin/posts", "{}", hdr{"Content-Type": "text/plain"}).expect(415)

	r = c.do("POST", "/api/admin/posts", postInput("写时渲染：把成本留在编辑器里", "render-on-write",
		"博客属于典型的一次写入多次读取场景，三元组索引用于中文检索。",
		map[string]any{"tag_ids": []int64{tagID}, "series_id": seriesID})).expect(201)
	created := r.json()
	postID := num(created["id"])
	if r.Header().Get("ETag") != `"v1"` || r.Header().Get("Location") != fmt.Sprintf("/api/admin/posts/%d", postID) {
		t.Fatalf("create headers: %v", r.Header())
	}
	if created["state"] != "draft" || created["published_at"] != nil {
		t.Fatalf("new post should be draft: %v", created)
	}
	if s := created["series"].(map[string]any); num(s["position"]) != 1 || num(s["id"]) != seriesID {
		t.Fatalf("series = %v", s)
	}
	c.do("POST", "/api/admin/posts", postInput("重复", "render-on-write", "x", nil)).expect(409)

	c.do("GET", "/api/posts/render-on-write", nil).expect(404)

	path := fmt.Sprintf("/api/admin/posts/%d", postID)
	got := c.do("GET", path, nil).expect(200)
	if got.Header().Get("ETag") != `"v1"` || !strings.Contains(got.json()["body_md"].(string), "三元组") {
		t.Fatalf("admin get: %v %s", got.Header(), got.Body)
	}
	update := postInput("写时渲染：把成本留在编辑器里", "render-on-write-v2", "改写后的正文，包含 pg_trgm 检索说明。",
		map[string]any{"tag_ids": []int64{tagID}, "series_id": seriesID})
	if c.do("PUT", path, update).expect(428).problemType() != "precondition-required" {
		t.Fatal("PUT without If-Match should be 428")
	}
	r = c.do("PUT", path, update, hdr{"If-Match": `"v7"`}).expect(412)
	if pf := r.json(); pf["current_etag"] != `"v1"` || pf["updated_at"] == nil {
		t.Fatalf("412 body: %v", pf)
	}
	r = c.do("PUT", path, update, hdr{"If-Match": `"v1"`}).expect(200)
	if r.Header().Get("ETag") != `"v2"` || num(r.json()["series"].(map[string]any)["position"]) != 1 {
		t.Fatalf("update: %v %s", r.Header(), r.Body)
	}

	empty := c.do("POST", "/api/admin/posts", postInput("空文章", "empty", "", nil)).expect(201).json()
	c.do("PUT", fmt.Sprintf("/api/admin/posts/%d/publication", num(empty["id"])), nil, hdr{"If-Match": `"v1"`}).expect(422)

	r = c.do("PUT", path+"/publication", nil, hdr{"If-Match": `"v2"`}).expect(200)
	if pub := r.json(); pub["state"] != "published" || pub["published_at"] == nil {
		t.Fatalf("publish: %v", pub)
	}
	firstPublishedAt := r.json()["published_at"].(string)

	detail := c.do("GET", "/api/posts/render-on-write-v2", nil).expect(200)
	if detail.Header().Get("Cache-Control") != "no-cache" || detail.Header().Get("ETag") == "" {
		t.Fatalf("public headers: %v", detail.Header())
	}
	d := detail.json()
	if d["html"] == "" || len(d["toc"].([]any)) != 1 || len(d["tags"].([]any)) != 1 {
		t.Fatalf("detail: %v", d)
	}
	nav := d["series"].(map[string]any)
	if num(nav["index"]) != 1 || num(nav["total"]) != 1 || nav["prev"] != nil {
		t.Fatalf("series nav: %v", nav)
	}
	c.do("GET", "/api/posts/render-on-write-v2", nil, hdr{"If-None-Match": detail.Header().Get("ETag")}).expect(304)

	r = c.do("GET", "/api/posts/render-on-write", nil).expect(301)
	if r.Header().Get("Location") != "/api/posts/render-on-write-v2" {
		t.Fatalf("Location = %q", r.Header().Get("Location"))
	}

	second := c.do("POST", "/api/admin/posts", postInput("用 pg_trgm 做中文检索", "pg-trgm-search", "中文没有天然的空格分词。",
		map[string]any{"series_id": seriesID})).expect(201).json()
	secondPath := fmt.Sprintf("/api/admin/posts/%d", num(second["id"]))
	future := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	r = c.do("PUT", secondPath+"/publication", map[string]any{"published_at": future}, hdr{"If-Match": `"v1"`}).expect(200)
	if r.json()["state"] != "scheduled" {
		t.Fatalf("expected scheduled: %s", r.Body)
	}
	c.do("GET", "/api/posts/pg-trgm-search", nil).expect(404)
	list := c.do("GET", "/api/admin/posts?state=scheduled", nil).expect(200).json()
	if items := list["items"].([]any); len(items) != 1 || num(items[0].(map[string]any)["id"]) != num(second["id"]) {
		t.Fatalf("scheduled filter: %v", list)
	}

	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
	c.do("PUT", secondPath+"/publication", map[string]any{"published_at": past}, hdr{"If-Match": `"v2"`}).expect(200)
	d = c.do("GET", "/api/posts/render-on-write-v2", nil).expect(200).json()
	if adj := d["adjacent"].(map[string]any); adj["newer"] != nil || adj["older"].(map[string]any)["slug"] != "pg-trgm-search" {
		t.Fatalf("adjacent: %v", adj)
	}
	if nav := d["series"].(map[string]any); num(nav["total"]) != 2 || nav["next"] == nil {
		t.Fatalf("series nav after second: %v", nav)
	}

	page1 := c.do("GET", "/api/posts?limit=1", nil).expect(200).json()
	if len(page1["items"].([]any)) != 1 || page1["next_cursor"] == nil {
		t.Fatalf("page1: %v", page1)
	}
	page2 := c.do("GET", "/api/posts?limit=1&cursor="+page1["next_cursor"].(string), nil).expect(200).json()
	if items := page2["items"].([]any); len(items) != 1 || items[0].(map[string]any)["slug"] != "pg-trgm-search" || page2["next_cursor"] != nil {
		t.Fatalf("page2: %v", page2)
	}
	tagged := c.do("GET", "/api/posts?tag=postgresql", nil).expect(200).json()["items"].([]any)
	if len(tagged) != 1 {
		t.Fatalf("tag filter: %v", tagged)
	}

	res := c.do("GET", "/api/search?q="+urlQuery("三元组"), nil).expect(200).json()["items"].([]any)
	if len(res) != 0 {
		t.Fatalf("old body text should no longer match: %v", res)
	}
	res = c.do("GET", "/api/search?q="+urlQuery("空格分词"), nil).expect(200).json()["items"].([]any)
	if len(res) != 1 || !strings.Contains(res[0].(map[string]any)["snippet"].(string), "空格分词") {
		t.Fatalf("search: %v", res)
	}
	res = c.do("GET", "/api/search?q=PG_TRGM", nil).expect(200).json()["items"].([]any)
	if len(res) != 2 || res[0].(map[string]any)["slug"] != "pg-trgm-search" {
		t.Fatalf("title match should rank first: %v", res)
	}
	c.do("GET", "/api/search?q=%20%20", nil).expect(422)

	tags := c.do("GET", "/api/tags", nil).expect(200).json()["items"].([]any)
	if len(tags) != 1 || num(tags[0].(map[string]any)["post_count"]) != 1 {
		t.Fatalf("public tags: %v", tags)
	}
	c.do("GET", "/api/tags/postgresql", nil).expect(200)
	c.do("GET", "/api/tags/missing", nil).expect(404)
	sl := c.do("GET", "/api/series", nil).expect(200).json()["items"].([]any)
	if len(sl) != 1 || num(sl[0].(map[string]any)["post_count"]) != 2 {
		t.Fatalf("public series: %v", sl)
	}
	sd := c.do("GET", "/api/series/blog-from-scratch", nil).expect(200).json()
	if posts := sd["posts"].([]any); len(posts) != 2 || posts[0].(map[string]any)["slug"] != "render-on-write-v2" {
		t.Fatalf("series detail: %v", sd)
	}

	seriesPath := fmt.Sprintf("/api/admin/series/%d", seriesID)
	c.do("PUT", seriesPath+"/posts", map[string]any{"post_ids": []int64{postID}}).expect(422)
	c.do("PUT", seriesPath+"/posts", map[string]any{"post_ids": []int64{postID, postID}}).expect(422)
	r = c.do("PUT", seriesPath+"/posts", map[string]any{"post_ids": []int64{num(second["id"]), postID}}).expect(200)
	if posts := r.json()["posts"].([]any); num(posts[0].(map[string]any)["id"]) != num(second["id"]) {
		t.Fatalf("reorder: %s", r.Body)
	}
	sd = c.do("GET", "/api/series/blog-from-scratch", nil).expect(200).json()
	if sd["posts"].([]any)[0].(map[string]any)["slug"] != "pg-trgm-search" {
		t.Fatalf("public order after reorder: %v", sd)
	}

	cur := c.do("GET", path, nil).expect(200)
	r = c.do("DELETE", path+"/publication", nil, hdr{"If-Match": cur.Header().Get("ETag")}).expect(200)
	if un := r.json(); un["state"] != "draft" || un["published_at"] != firstPublishedAt {
		t.Fatalf("unpublish: %v (want published_at %s)", un, firstPublishedAt)
	}
	c.do("GET", "/api/posts/render-on-write-v2", nil).expect(404)
	r = c.do("PUT", path+"/publication", "", hdr{"If-Match": r.Header().Get("ETag"), "Content-Type": ""}).expect(200)
	if r.json()["published_at"] != firstPublishedAt {
		t.Fatalf("republish should keep original time: %s", r.Body)
	}

	c.do("DELETE", seriesPath, nil, hdr{"If-Match": `"v99"`}).expect(412)
	c.do("DELETE", seriesPath, nil).expect(204)
	if s := c.do("GET", path, nil).expect(200).json()["series"]; s != nil {
		t.Fatalf("post should leave deleted series: %v", s)
	}

	c.do("DELETE", fmt.Sprintf("/api/admin/tags/%d", tagID), nil).expect(204)
	c.do("DELETE", path, nil).expect(428)
	cur = c.do("GET", path, nil).expect(200)
	c.do("DELETE", path, nil, hdr{"If-Match": cur.Header().Get("ETag")}).expect(204)
	c.do("GET", path, nil).expect(404)
	c.do("GET", "/api/posts/render-on-write", nil).expect(404)
}

func TestTagAndSeriesAdmin(t *testing.T) {
	a, _ := newTestApp(t)
	c := &client{t: t, h: a.Handler()}
	c.login()

	tag := c.do("POST", "/api/admin/tags", map[string]string{"name": "架构", "slug": "architecture"}).expect(201)
	id := num(tag.json()["id"])
	path := fmt.Sprintf("/api/admin/tags/%d", id)
	c.do("PATCH", path, map[string]string{"name": "系统架构"}, hdr{"If-Match": `"v2"`}).expect(412)
	r := c.do("PATCH", path, map[string]string{"name": "系统架构"}, hdr{"If-Match": `"v1"`}).expect(200)
	if r.json()["name"] != "系统架构" || r.json()["slug"] != "architecture" || r.Header().Get("ETag") != `"v2"` {
		t.Fatalf("patch tag: %s", r.Body)
	}
	c.do("PATCH", path, map[string]string{"slug": "Not Valid"}).expect(422)
	items := c.do("GET", "/api/admin/tags", nil).expect(200).json()["items"].([]any)
	if len(items) != 1 || num(items[0].(map[string]any)["post_count"]) != 0 {
		t.Fatalf("admin tags: %v", items)
	}

	s := c.do("POST", "/api/admin/series", map[string]any{"title": "专栏", "slug": "col", "description": "简介"}).expect(201).json()
	sp := fmt.Sprintf("/api/admin/series/%d", num(s["id"]))
	c.do("POST", "/api/admin/series", map[string]any{"title": "另一个", "slug": "col"}).expect(409)
	r = c.do("PATCH", sp, map[string]any{"description": "新的简介"}).expect(200)
	if r.json()["description"] != "新的简介" || r.json()["title"] != "专栏" {
		t.Fatalf("patch series: %s", r.Body)
	}
	if posts := c.do("GET", sp, nil).expect(200).json()["posts"].([]any); len(posts) != 0 {
		t.Fatalf("empty series posts: %v", posts)
	}
	c.do("GET", "/api/series/col", nil).expect(404)
}

func TestMediaAndFeeds(t *testing.T) {
	a, _ := newTestApp(t)
	c := &client{t: t, h: a.Handler()}
	c.login()

	img := pngImage(t, 1200, 800, 7)
	r := c.upload("topology.png", img, "部署拓扑示意图").expect(201)
	m := r.json()
	sha := m["sha256"].(string)
	mediaID := num(m["id"])
	if num(m["width"]) != 1200 || num(m["height"]) != 800 || m["original_filename"] != "topology.png" || m["alt"] != "部署拓扑示意图" {
		t.Fatalf("media: %v", m)
	}
	variants := m["variants"].([]any)
	if len(variants) != 3 {
		t.Fatalf("variants: %v", variants)
	}
	if v := variants[0].(map[string]any); v["format"] != "webp" || v["url"] != fmt.Sprintf("/media/%s/480.webp", sha) {
		t.Fatalf("variant: %v", v)
	}
	if again := c.upload("copy.png", img, "").expect(200).json(); num(again["id"]) != mediaID {
		t.Fatalf("dedupe returned different media: %v", again)
	}
	c.upload("note.txt", []byte("hello, not an image"), "").expect(415)

	res := c.do("GET", "/media/"+sha, nil).expect(200)
	if res.Header().Get("Content-Type") != "image/webp" || !strings.Contains(res.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("media headers: %v", res.Header())
	}
	if !bytes.HasPrefix(res.Body.Bytes(), []byte("RIFF")) {
		t.Fatal("served media is not WebP")
	}
	c.do("GET", "/media/"+sha+"/480.webp", nil).expect(200)
	c.do("GET", "/media/"+sha+"/481.webp", nil).expect(404)
	c.do("GET", "/media/"+strings.Repeat("0", 64), nil).expect(404)

	byHash := c.do("GET", "/api/admin/media?sha256="+sha+"&sha256="+strings.Repeat("a", 64), nil).expect(200).json()
	if len(byHash["items"].([]any)) != 1 {
		t.Fatalf("by hash: %v", byHash)
	}
	if unused := c.do("GET", "/api/admin/media?unused=true", nil).expect(200).json()["items"].([]any); len(unused) != 1 {
		t.Fatalf("unused: %v", unused)
	}

	missing := strings.Repeat("b", 64)
	c.do("POST", "/api/admin/posts", postInput("图文", "with-image", "![x](/media/"+missing+")", nil)).expect(422)

	body := "![部署拓扑](/media/" + sha + ")\n\n[站内链接](/posts/other) [外链](https://example.org/x)"
	in := postInput("图文", "with-image", body, map[string]any{"cover_media_id": mediaID})
	p := c.do("POST", "/api/admin/posts", in).expect(201).json()
	postPath := fmt.Sprintf("/api/admin/posts/%d", num(p["id"]))
	c.do("PUT", postPath+"/publication", nil, hdr{"If-Match": `"v1"`}).expect(200)

	mp := fmt.Sprintf("/api/admin/media/%d", mediaID)
	detail := c.do("GET", mp, nil).expect(200).json()
	if num(detail["reference_count"]) != 1 || len(detail["referenced_by"].([]any)) != 1 {
		t.Fatalf("media detail: %v", detail)
	}
	r = c.do("DELETE", mp, nil).expect(409)
	if refs := r.json()["referenced_by"].([]any); len(refs) != 1 {
		t.Fatalf("409 should list references: %s", r.Body)
	}
	r = c.do("PATCH", mp, map[string]string{"alt": "新的替代文本"}, hdr{"If-Match": `"v1"`}).expect(200)
	if r.json()["alt"] != "新的替代文本" || r.Header().Get("ETag") != `"v2"` {
		t.Fatalf("patch media: %s", r.Body)
	}

	pd := c.do("GET", "/api/posts/with-image", nil).expect(200).json()
	if cover := pd["cover"].(map[string]any); cover["sha256"] != sha || cover["src"] != "/media/"+sha {
		t.Fatalf("cover: %v", pd["cover"])
	}

	atom := c.do("GET", "/feed.xml", nil).expect(200)
	if ct := atom.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/atom+xml") {
		t.Fatalf("atom content-type %q", ct)
	}
	ab := atom.Body.String()
	for _, want := range []string{
		`src=&#34;https://blog.example.com/media/` + sha,
		`https://blog.example.com/media/` + sha + `/960.webp 960w`,
		`href=&#34;https://blog.example.com/posts/other`,
		`href=&#34;https://example.org/x`,
		`<link rel="alternate" type="text/html" href="https://blog.example.com/posts/with-image"`,
	} {
		if !strings.Contains(ab, want) {
			t.Errorf("atom missing %q\n%s", want, ab)
		}
	}
	if atom.Header().Get("Cache-Control") != "public, max-age=300" || atom.Header().Get("Last-Modified") == "" {
		t.Fatalf("feed headers: %v", atom.Header())
	}
	c.do("GET", "/feed.xml", nil, hdr{"If-None-Match": atom.Header().Get("ETag")}).expect(304)

	rss := c.do("GET", "/rss.xml", nil).expect(200).Body.String()
	if !strings.Contains(rss, "<rss version=\"2.0\"") || !strings.Contains(rss, "<link>https://blog.example.com/posts/with-image</link>") {
		t.Fatalf("rss: %s", rss)
	}
	sm := c.do("GET", "/sitemap.xml", nil).expect(200).Body.String()
	if !strings.Contains(sm, "<loc>https://blog.example.com/</loc>") || !strings.Contains(sm, "<loc>https://blog.example.com/posts/with-image</loc>") {
		t.Fatalf("sitemap: %s", sm)
	}

	cur := c.do("GET", postPath, nil).expect(200)
	c.do("DELETE", postPath, nil, hdr{"If-Match": cur.Header().Get("ETag")}).expect(204)
	c.do("DELETE", mp, nil, hdr{"If-Match": `"v2"`}).expect(204)
	c.do("GET", "/media/"+sha, nil).expect(404)
}

func TestUploadTooLarge(t *testing.T) {
	a, _ := newTestApp(t)
	c := &client{t: t, h: a.Handler()}
	c.login()
	big := make([]byte, 21<<20)
	if r := c.upload("big.png", big, ""); r.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, body %s", r.Code, r.Body)
	}
}

func urlQuery(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

func TestSiteSettings(t *testing.T) {
	a, _ := newTestApp(t)
	c := &client{t: t, h: a.Handler()}

	pub := c.do("GET", "/api/settings", nil).expect(200)
	pm := pub.json()
	if pm["title"] != "Blog" || pm["language"] != "zh-CN" || pm["timezone"] != "Asia/Shanghai" || num(pm["posts_per_page"]) != 20 ||
		pm["noindex"] != false || len(pm["navigation"].([]any)) != 0 || len(pm["social_links"].([]any)) != 0 {
		t.Fatalf("default settings: %v", pm)
	}
	if _, ok := pm["feed_item_count"]; ok {
		t.Fatalf("public settings leak admin fields: %v", pm)
	}
	c.do("GET", "/api/settings", nil, hdr{"If-None-Match": pub.Header().Get("ETag")}).expect(304)
	c.do("GET", "/api/admin/settings", nil).expect(401)

	c.login()
	if r := c.do("GET", "/api/admin/settings", nil).expect(200); r.Header().Get("ETag") != `"v1"` || num(r.json()["feed_item_count"]) != 20 {
		t.Fatalf("admin settings: %v %s", r.Header(), r.Body)
	}

	r := c.do("PATCH", "/api/admin/settings", map[string]any{"title": "  测试博客  ", "author": "Wu", "feed_item_count": 1},
		hdr{"If-Match": `"v1"`}).expect(200)
	if m := r.json(); m["title"] != "测试博客" || m["author"] != "Wu" || num(m["feed_item_count"]) != 1 || r.Header().Get("ETag") != `"v2"` {
		t.Fatalf("patch settings: %v %s", r.Header(), r.Body)
	}
	if got := c.do("PATCH", "/api/admin/settings", map[string]any{"title": "x"}, hdr{"If-Match": `"v1"`}).expect(412).problemType(); got != "precondition-failed" {
		t.Fatalf("stale If-Match: %s", got)
	}
	r = c.do("PATCH", "/api/admin/settings", map[string]any{"title": " ", "language": "zh_CN", "posts_per_page": 0}).expect(422)
	if errs := r.json()["errors"].([]any); len(errs) != 3 {
		t.Fatalf("validation errors: %s", r.Body)
	}
	c.do("PATCH", "/api/admin/settings", map[string]any{"theme": "dark"}).expect(400)
	if r := c.do("PATCH", "/api/admin/settings", map[string]any{}).expect(200); r.Header().Get("ETag") != `"v2"` {
		t.Fatalf("empty patch bumped version: %v", r.Header())
	}
	if pm := c.do("GET", "/api/settings", nil).expect(200).json(); pm["title"] != "测试博客" || pm["author"] != "Wu" {
		t.Fatalf("public settings after patch: %v", pm)
	}

	for _, slug := range []string{"first", "second"} {
		p := c.do("POST", "/api/admin/posts", postInput(slug, slug, "正文", nil)).expect(201).json()
		c.do("PUT", fmt.Sprintf("/api/admin/posts/%d/publication", num(p["id"])), nil, hdr{"If-Match": `"v1"`}).expect(200)
	}
	atom := c.do("GET", "/feed.xml", nil).expect(200).Body.String()
	if n := strings.Count(atom, "<entry>"); n != 1 || !strings.Contains(atom, "<title>测试博客</title>") || !strings.Contains(atom, "<name>Wu</name>") {
		t.Fatalf("atom should honour settings (%d entries): %s", n, atom)
	}

	nav := []map[string]string{{"label": "关于", "url": "/about"}, {"label": "GitHub", "url": "https://github.com/wu"}}
	social := []map[string]string{{"platform": "github", "url": "https://github.com/wu"}, {"platform": "email", "url": "mailto:wu@example.com"}}
	r = c.do("PATCH", "/api/admin/settings", map[string]any{
		"subtitle": "副标题", "timezone": "America/New_York", "navigation": nav, "social_links": social,
	}).expect(200)
	if m := r.json(); m["subtitle"] != "副标题" || m["timezone"] != "America/New_York" ||
		len(m["navigation"].([]any)) != 2 || m["social_links"].([]any)[1].(map[string]any)["url"] != "mailto:wu@example.com" {
		t.Fatalf("patch links: %s", r.Body)
	}
	r = c.do("PATCH", "/api/admin/settings", map[string]any{
		"timezone":     "Local",
		"navigation":   []map[string]string{{"label": "", "url": "javascript:alert(1)"}, {"label": "x", "url": "//evil.example"}},
		"social_links": []map[string]string{{"platform": "GitHub", "url": "ftp://x"}},
	}).expect(422)
	if errs := r.json()["errors"].([]any); len(errs) != 6 {
		t.Fatalf("link validation errors: %s", r.Body)
	}
	c.do("PATCH", "/api/admin/settings", map[string]any{"navigation": []map[string]string{{"label": "x", "url": "/", "icon": "home"}}}).expect(400)
	if m := c.do("GET", "/api/admin/settings", nil).expect(200).json(); len(m["navigation"].([]any)) != 2 {
		t.Fatalf("rejected patch changed navigation: %v", m)
	}

	robots := c.do("GET", "/robots.txt", nil).expect(200).Body.String()
	if !strings.Contains(robots, "Sitemap: "+siteURL+"/sitemap.xml") {
		t.Fatalf("robots.txt: %q", robots)
	}
	c.do("PATCH", "/api/admin/settings", map[string]any{"noindex": true}).expect(200)
	if robots := c.do("GET", "/robots.txt", nil).expect(200).Body.String(); strings.Contains(robots, "Sitemap") || strings.Contains(robots, "Disallow") {
		t.Fatalf("noindex robots.txt: %q", robots)
	}
	c.do("GET", "/sitemap.xml", nil).expect(404)
	if res := c.do("GET", "/rss.xml", nil).expect(200); res.Header().Get("X-Robots-Tag") != "noindex" {
		t.Fatalf("rss headers: %v", res.Header())
	}
	if pm := c.do("GET", "/api/settings", nil).expect(200).json(); pm["noindex"] != true {
		t.Fatalf("public noindex: %v", pm)
	}
}
