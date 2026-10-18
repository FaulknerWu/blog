// Package testkit 为集成测试提供完整的服务端环境：独立的临时数据库、模拟的渲染服务与装配好的 App。
// 需要设置 BLOG_TEST_DATABASE_URL，未设置时跳过测试。
package testkit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"blog/api/internal/app"
	"blog/api/internal/auth"
	"blog/api/internal/store"
)

const (
	SiteURL  = "https://blog.example.com"
	User     = "author"
	Password = "correct-horse-battery"
)

type Env struct {
	App    *app.App
	Store  *store.Store
	Render *FakeRenderer
}

// New 创建一个空数据库并迁移、建好作者账号，测试结束时删除。
func New(t testing.TB) *Env {
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
	if _, err := auth.NewService(s).CreateUser(ctx, User, "Author", Password); err != nil {
		t.Fatalf("create user: %v", err)
	}
	fr := NewFakeRenderer(t)
	a, err := app.New(app.Config{
		SiteURL:      SiteURL,
		RenderURL:    fr.URL,
		MediaDir:     t.TempDir(),
		ImageWorkers: 2,
	}, s)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	return &Env{App: a, Store: s, Render: fr}
}

