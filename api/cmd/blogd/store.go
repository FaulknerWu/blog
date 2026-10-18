package main

import (
	"context"

	"blog/api/internal/app"
	"blog/api/internal/store"
)

// withStore 为运维命令打开数据库连接，命令结束后关闭。
func withStore(ctx context.Context, fn func(*store.Store) error) error {
	dsn, err := app.DatabaseURL()
	if err != nil {
		return err
	}
	s, err := store.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer s.Close()
	return fn(s)
}

func migrate(ctx context.Context, args []string) error {
	if err := newFlags("migrate").Parse(args); err != nil {
		return err
	}
	return withStore(ctx, func(s *store.Store) error { return s.Migrate(ctx) })
}
