package main

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

func healthcheck(ctx context.Context, args []string) error {
	fs := newFlags("healthcheck")
	url := fs.String("url", "http://127.0.0.1:8080/healthz", "健康检查地址")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, *url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: %s", resp.Status)
	}
	return nil
}
