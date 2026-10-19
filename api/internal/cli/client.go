package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// client 以 Bearer 令牌调用博客 API。非 2xx 响应转为 *apiError，网络故障转为 blog-cli:network。
type client struct {
	base  string
	token string
	http  *http.Client
	err   error // 凭据缺失时推迟到第一次请求再报告，使参数用法错误优先暴露
}

type response struct {
	status int
	header http.Header
	body   []byte
}

// version 从 ETag（"v7"）解析出资源版本号。
func (r *response) version() int32 {
	n, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(r.header.Get("ETag"), `"v`), `"`))
	return int32(n)
}

func (r *response) decode(dst any) error {
	if err := json.Unmarshal(r.body, dst); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

type request struct {
	method  string
	path    string
	query   url.Values
	body    any       // 编码为 JSON；nil 表示无请求体
	raw     io.Reader // 已编码的请求体，与 contentType 一起使用
	ctype   string
	ifMatch int32 // >0 时携带 If-Match: "v{n}"
}

func ifMatch(version int32) string { return fmt.Sprintf(`"v%d"`, version) }

func (c *client) do(ctx context.Context, rq request) (*response, error) {
	if c.err != nil {
		return nil, c.err
	}
	u := c.base + rq.path
	if len(rq.query) > 0 {
		u += "?" + rq.query.Encode()
	}
	body, ctype := rq.raw, rq.ctype
	if rq.body != nil {
		data, err := json.Marshal(rq.body)
		if err != nil {
			return nil, err
		}
		body, ctype = bytes.NewReader(data), "application/json"
	}
	req, err := http.NewRequestWithContext(ctx, rq.method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "blog-cli")
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	if rq.ifMatch > 0 {
		req.Header.Set("If-Match", ifMatch(rq.ifMatch))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, networkError(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, networkError(err)
	}
	if resp.StatusCode >= 300 {
		return nil, &apiError{status: resp.StatusCode, body: data}
	}
	return &response{status: resp.StatusCode, header: resp.Header, body: data}, nil
}

func (c *client) get(ctx context.Context, path string, query url.Values) (*response, error) {
	return c.do(ctx, request{method: http.MethodGet, path: path, query: query})
}

// getJSON 请求并解码 JSON 响应。
func (c *client) getJSON(ctx context.Context, path string, query url.Values, dst any) error {
	r, err := c.get(ctx, path, query)
	if err != nil {
		return err
	}
	return r.decode(dst)
}

// list 拉取游标分页列表；all 为 true 时跟随 next_cursor 取完全部页，合并为一页返回。
func (c *client) list(ctx context.Context, path string, query url.Values, all bool) (json.RawMessage, error) {
	var items []json.RawMessage
	for {
		r, err := c.get(ctx, path, query)
		if err != nil {
			return nil, err
		}
		if !all {
			return r.body, nil
		}
		var page struct {
			Items      []json.RawMessage `json:"items"`
			NextCursor *string           `json:"next_cursor"`
		}
		if err := r.decode(&page); err != nil {
			return nil, err
		}
		items = append(items, page.Items...)
		if page.NextCursor == nil {
			if items == nil {
				items = []json.RawMessage{}
			}
			return json.Marshal(map[string]any{"items": items, "next_cursor": nil})
		}
		query = cloneQuery(query)
		query.Set("cursor", *page.NextCursor)
	}
}

func cloneQuery(q url.Values) url.Values {
	out := url.Values{}
	for k, v := range q {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// upload 上传图片；内容已存在时服务端返回已有记录。
func (c *client) upload(ctx context.Context, path, alt string) (*response, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, invalidError("读取图片 %s：%v", path, err)
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return nil, err
	}
	fw.Write(data)
	if alt != "" {
		mw.WriteField("alt", alt)
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	return c.do(ctx, request{method: http.MethodPost, path: "/api/admin/media", raw: &buf, ctype: mw.FormDataContentType()})
}
