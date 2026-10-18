package main

import (
	"context"

	"blog/api/internal/app"
	"blog/api/internal/store"
)

func serve(ctx context.Context, args []string) error {
	if err := newFlags("serve").Parse(args); err != nil {
		return err
	}
	cfg, err := app.LoadConfig()
	if err != nil {
		return err
	}
	s, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer s.Close()
	if cfg.MigrateOnStart {
		if err := s.Migrate(ctx); err != nil {
			return err
		}
	}
	a, err := app.New(cfg, s)
	if err != nil {
		return err
	}
	return a.Run(ctx)
}
