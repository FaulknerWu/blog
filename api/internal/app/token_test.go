package app_test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestAPITokens(t *testing.T) {
	a, _ := newTestApp(t)
	h := a.Handler()
	c := &client{t: t, h: h}
	c.login()

	c.do("POST", "/api/auth/tokens", map[string]any{"name": " ", "scopes": []string{"content:root"}}).expect(422)
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	c.do("POST", "/api/auth/tokens", map[string]any{"name": "old", "scopes": []string{"content:read"}, "expires_at": past}).expect(422)

	r := c.do("POST", "/api/auth/tokens", map[string]any{"name": "agent", "scopes": []string{"content:write", "content:read"}}).expect(201)
	created := r.json()
	secret := created["secret"].(string)
	tok := created["token"].(map[string]any)
	if !strings.HasPrefix(secret, "blog_pat_") || tok["name"] != "agent" || tok["last_used_at"] != nil ||
		fmt.Sprint(tok["scopes"]) != "[content:read content:write]" {
		t.Fatalf("create token: %s", r.Body)
	}
	c.do("POST", "/api/auth/tokens", map[string]any{"name": "agent", "scopes": []string{"content:read"}}).expect(409)

	agent := &client{t: t, h: h, bearer: secret}
	me := agent.do("GET", "/api/auth/me", nil).expect(200).json()
	if me["via"] != "token" || me["username"] != testUser || len(me["scopes"].([]any)) != 2 {
		t.Fatalf("me via token: %v", me)
	}
	if me := c.do("GET", "/api/auth/me", nil).expect(200).json(); me["via"] != "session" || len(me["scopes"].([]any)) != 5 {
		t.Fatalf("me via session: %v", me)
	}

	// 令牌请求不需要也不检查浏览器跨站头。
	p := agent.do("POST", "/api/admin/posts", postInput("代理写的草稿", "agent-draft", "正文", nil)).expect(201).json()
	postPath := fmt.Sprintf("/api/admin/posts/%d", num(p["id"]))
	agent.do("GET", "/api/admin/posts?slug=agent-draft", nil).expect(200)
	if r := agent.do("PUT", postPath+"/publication", nil, hdr{"If-Match": `"v1"`}).expect(403); !strings.Contains(r.Body.String(), "content:publish") {
		t.Fatalf("publish without scope: %s", r.Body)
	}
	agent.do("DELETE", postPath, nil, hdr{"If-Match": `"v1"`}).expect(403)
	agent.upload("x.png", pngImage(t, 10, 10, 1), "").expect(403)
	agent.do("PATCH", "/api/admin/settings", map[string]any{"title": "x"}).expect(403)

	// 账户、会话与令牌管理只接受浏览器会话。
	agent.do("GET", "/api/auth/tokens", nil).expect(403)
	agent.do("POST", "/api/auth/tokens", map[string]any{"name": "more", "scopes": []string{"content:publish"}}).expect(403)
	agent.do("GET", "/api/auth/sessions", nil).expect(403)
	agent.do("PUT", "/api/auth/password", map[string]string{"current_password": testPassword, "new_password": "another-long-password"}).expect(403)

	(&client{t: t, h: h, bearer: "blog_pat_nope"}).do("GET", "/api/admin/posts", nil).expect(401)
	(&client{t: t, h: h, bearer: "not-a-token"}).do("GET", "/api/admin/posts", nil).expect(401)
	r = (&client{t: t, h: h}).do("GET", "/api/admin/posts", nil, hdr{"Authorization": "Basic Zm9vOmJhcg=="}).expect(401)
	if r.problemType() != "unauthenticated" {
		t.Fatalf("basic auth: %s", r.Body)
	}

	// 会话请求仍受跨站校验约束。
	c.do("POST", "/api/admin/tags", map[string]string{"name": "Go", "slug": "go"}, hdr{"Sec-Fetch-Site": "cross-site"}).expect(403)

	list := c.do("GET", "/api/auth/tokens", nil).expect(200).json()["items"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["last_used_at"] == nil {
		t.Fatalf("token list: %v", list)
	}
	c.do("DELETE", fmt.Sprintf("/api/auth/tokens/%d", num(tok["id"])), nil).expect(204)
	c.do("DELETE", fmt.Sprintf("/api/auth/tokens/%d", num(tok["id"])), nil).expect(404)
	agent.do("GET", "/api/admin/posts", nil).expect(401)
}
