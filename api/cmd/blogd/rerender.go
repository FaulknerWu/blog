package main

import (
	"context"
	"fmt"
	"os"

	"blog/api/internal/app"
	"blog/api/internal/content"
	"blog/api/internal/render"
	"blog/api/internal/store"
)

func rerender(ctx context.Context, args []string) error {
	if err := newFlags("rerender").Parse(args); err != nil {
		return err
	}
	renderURL, err := app.RenderURL()
	if err != nil {
		return err
	}
	return withStore(ctx, func(s *store.Store) error {
		report, err := content.NewService(s, render.NewClient(renderURL)).Rerender(ctx)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "renderer %s: %d posts and %d pages rerendered, %d failed\n",
			report.RendererVersion, report.Posts, report.Pages, report.Failed)
		if report.Failed > 0 {
			return fmt.Errorf("%d items could not be rendered; see the log for details", report.Failed)
		}
		return nil
	})
}
