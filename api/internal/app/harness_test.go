package app_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"blog/api/internal/app"
	"blog/api/internal/auth"
	"blog/api/internal/store"
)

const (
	siteURL      = "https://blog.example.com"
	testUser     = "author"
	testPassword = "correct-horse-battery"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.DiscardHandler))
	os.Exit(m.Run())
}

func newTestApp(t *testing.T) (*app.App, *store.Store) {
	t.Helper()
	adminURL := os.Getenv("BLOG_TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("BLOG_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect admin db: %v", err)
	}
	var b [6]byte
	_, _ = rand.Read(b[:])
	dbName := "blog_test_" + hex.EncodeToString(b[:])
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+dbName+" TEMPLATE template0 ENCODING 'UTF8' LOCALE 'C.UTF-8'"); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE "+dbName+" WITH (FORCE)")
		admin.Close(context.Background())
	})

	u, _ := url.Parse(adminURL)
	u.Path = "/" + dbName
	s, err := store.Open(ctx, u.String())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(s.Close)
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := auth.NewService(s).CreateUser(ctx, testUser, "Author", testPassword); err != nil {
		t.Fatalf("create user: %v", err)
	}
	a, err := app.New(app.Config{
		SiteURL:      siteURL,
		MediaDir:     t.TempDir(),
		ImageWorkers: 2,
	}, s)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	return a, s
}

type client struct {
	t      *testing.T
	h      http.Handler
	cookie string
}

type response struct {
	*httptest.ResponseRecorder
	t *testing.T
}

func (r response) json() map[string]any {
	r.t.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.Body.Bytes(), &m); err != nil {
		r.t.Fatalf("decode response %q: %v", r.Body.String(), err)
	}
	return m
}

func (r response) expect(status int) response {
	r.t.Helper()
	if r.Code != status {
		r.t.Fatalf("status = %d, want %d; body: %s", r.Code, status, r.Body.String())
	}
	return r
}

func (r response) problemType() string {
	r.t.Helper()
	if ct := r.Header().Get("Content-Type"); ct != "application/problem+json" {
		r.t.Fatalf("content-type = %q, want problem+json", ct)
	}
	typ, _ := r.json()["type"].(string)
	return strings.TrimPrefix(typ, siteURL+"/problems/")
}

type hdr map[string]string

func (c *client) send(req *http.Request, headers hdr) response {
	c.t.Helper()
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		req.Header.Set("Origin", siteURL)
		req.Header.Set("Sec-Fetch-Site", "same-origin")
	}
	if c.cookie != "" {
		req.Header.Set("Cookie", "__Host-session="+c.cookie)
	}
	for k, v := range headers {
		if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "__Host-session" {
			if ck.MaxAge < 0 {
				c.cookie = ""
			} else {
				c.cookie = ck.Value
			}
		}
	}
	return response{rec, c.t}
}

func (c *client) do(method, path string, body any, headers ...hdr) response {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		switch b := body.(type) {
		case string:
			r = strings.NewReader(b)
		default:
			buf, _ := json.Marshal(b)
			r = bytes.NewReader(buf)
		}
	}
	req := httptest.NewRequest(method, path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	var h hdr
	if len(headers) > 0 {
		h = headers[0]
	}
	return c.send(req, h)
}

func (c *client) upload(filename string, data []byte, alt string) response {
	c.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", filename)
	fw.Write(data)
	if alt != "" {
		mw.WriteField("alt", alt)
	}
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/media", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return c.send(req, nil)
}

func (c *client) login() {
	c.t.Helper()
	c.do("POST", "/api/auth/sessions", map[string]string{"username": testUser, "password": testPassword}).expect(201)
	if c.cookie == "" {
		c.t.Fatal("login did not set session cookie")
	}
}
