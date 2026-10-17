// Package render 是独立渲染服务（render/）的 HTTP 客户端。Markdown 解析、代码高亮与公式编译都在该服务中完成，
// API 只在写入时调用它，并把结果连同原文一起持久化。
package render

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"blog/api/internal/httpx"
	"blog/api/internal/media"
)

const (
	renderTimeout  = 30 * time.Second
	versionTimeout = 3 * time.Second
	versionTTL     = 30 * time.Second
	maxResponse    = 16 << 20
)

type Issue struct {
	Line     int    `json:"line"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // error 阻止保存，warning 仅作提示
}

type Result struct {
	HTML            string          `json:"html"`
	TOC             json.RawMessage `json:"toc"` // [{depth, id, text}]，原样存储
	PlainText       string          `json:"plain_text"`
	HasMath         bool            `json:"has_math"`
	RendererVersion string          `json:"renderer_version"`
	Issues          []Issue         `json:"issues"`
}

type request struct {
	Markdown string                 `json:"markdown"`
	Media    map[string]media.Image `json:"media"`
}

type Client struct {
	baseURL string
	http    *http.Client

	mu        sync.Mutex
	version   string
	versionAt time.Time
}

func NewClient(baseURL string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{}}
}

// unavailable 把渲染服务的故障统一转为 503，具体原因只写日志。
func unavailable(ctx context.Context, err error) error {
	slog.ErrorContext(ctx, "render service unavailable", "err", err)
	return httpx.ServiceUnavailable("渲染服务暂不可用，请稍后重试")
}

// Render 渲染 Markdown。images 以 sha256 为键，提供正文引用图片的尺寸与衍生规格。
func (c *Client) Render(ctx context.Context, markdown string, images map[string]media.Image) (Result, error) {
	if images == nil {
		images = map[string]media.Image{}
	}
	body, err := json.Marshal(request{Markdown: markdown, Media: images})
	if err != nil {
		return Result{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, renderTimeout)
	defer cancel()
	var res Result
	if err := c.do(ctx, http.MethodPost, "/render", body, &res); err != nil {
		return Result{}, unavailable(ctx, err)
	}
	c.remember(res.RendererVersion)
	return res, nil
}

// Version 返回渲染服务当前的渲染器版本，结果缓存 versionTTL，避免列表页每次都访问渲染服务。
func (c *Client) Version(ctx context.Context) (string, error) {
	c.mu.Lock()
	if c.version != "" && time.Since(c.versionAt) < versionTTL {
		v := c.version
		c.mu.Unlock()
		return v, nil
	}
	c.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, versionTimeout)
	defer cancel()
	var health struct {
		RendererVersion string `json:"renderer_version"`
	}
	if err := c.do(ctx, http.MethodGet, "/healthz", nil, &health); err != nil {
		return "", unavailable(ctx, err)
	}
	if health.RendererVersion == "" {
		return "", unavailable(ctx, fmt.Errorf("healthz returned no renderer_version"))
	}
	c.remember(health.RendererVersion)
	return health.RendererVersion, nil
}

func (c *Client) remember(version string) {
	if version == "" {
		return
	}
	c.mu.Lock()
	c.version, c.versionAt = version, time.Now()
	c.mu.Unlock()
}

func (c *Client) do(ctx context.Context, method, path string, body []byte, dst any) error {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, r)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, bytes.TrimSpace(data))
	}
	return json.Unmarshal(data, dst)
}
