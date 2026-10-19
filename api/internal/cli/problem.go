package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// 退出码是 CLI 对 agent 的稳定契约：调用方据此决定重试、重新拉取还是修正输入。
const (
	exitOK          = 0
	exitInternal    = 1 // 未预期的错误
	exitUsage       = 2 // 命令或参数用法错误
	exitAuth        = 3 // 未配置、未登录或权限不足（401/403）
	exitNotFound    = 4 // 资源不存在（404）
	exitConflict    = 5 // 版本冲突或重复（409/412/428），应重新拉取后再试
	exitInvalid     = 6 // 输入被拒绝或渲染出错（400/413/415/422）
	exitUnavailable = 7 // 网络故障、限流或服务暂不可用（429/5xx），可稍后重试
)

var exitCodes = []struct {
	Code    int    `json:"code"`
	Meaning string `json:"meaning"`
}{
	{exitOK, "成功"},
	{exitInternal, "未预期的错误"},
	{exitUsage, "命令或参数用法错误"},
	{exitAuth, "未配置凭据、令牌无效或缺少权限（401/403）"},
	{exitNotFound, "资源不存在（404）"},
	{exitConflict, "版本冲突或重复（409/412/428）：重新 pull 后再修改"},
	{exitInvalid, "输入被拒绝或正文渲染出错（400/413/415/422）"},
	{exitUnavailable, "网络故障、限流或服务暂不可用（429/5xx）：稍后重试"},
}

// apiError 是服务端返回的 RFC 9457 problem，原样输出到 stderr。
type apiError struct {
	status int
	body   json.RawMessage
}

func (e *apiError) Error() string {
	var p struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
	}
	_ = json.Unmarshal(e.body, &p)
	return fmt.Sprintf("%d %s: %s", e.status, p.Title, p.Detail)
}

// problem 是 CLI 本地产生的错误，与服务端 problem 结构一致，type 以 blog-cli: 开头。
type problem struct {
	Type   string       `json:"type"`
	Title  string       `json:"title"`
	Status int          `json:"status,omitempty"`
	Detail string       `json:"detail,omitempty"`
	Errors []fieldError `json:"errors,omitempty"`
	exit   int
}

type fieldError struct {
	Pointer string `json:"pointer,omitempty"`
	Code    string `json:"code"`
	Detail  string `json:"detail"`
	Line    int    `json:"line,omitempty"`
}

func (p *problem) Error() string { return p.Title + ": " + p.Detail }

func usageError(format string, args ...any) error {
	return &problem{Type: "blog-cli:usage", Title: "用法错误", Detail: fmt.Sprintf(format, args...), exit: exitUsage}
}

func invalidError(format string, args ...any) error {
	return &problem{Type: "blog-cli:invalid-input", Title: "输入无效", Detail: fmt.Sprintf(format, args...), exit: exitInvalid}
}

func notFoundError(format string, args ...any) error {
	return &problem{Type: "blog-cli:not-found", Title: "资源不存在", Status: http.StatusNotFound,
		Detail: fmt.Sprintf(format, args...), exit: exitNotFound}
}

func authError(format string, args ...any) error {
	return &problem{Type: "blog-cli:not-configured", Title: "未配置凭据", Detail: fmt.Sprintf(format, args...), exit: exitAuth}
}

func networkError(err error) error {
	return &problem{Type: "blog-cli:network", Title: "无法连接博客服务", Detail: err.Error(), exit: exitUnavailable}
}

func exitForStatus(status int) int {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return exitAuth
	case status == http.StatusNotFound:
		return exitNotFound
	case status == http.StatusConflict || status == http.StatusPreconditionFailed || status == http.StatusPreconditionRequired:
		return exitConflict
	case status == http.StatusTooManyRequests || status >= 500:
		return exitUnavailable
	case status >= 400:
		return exitInvalid
	}
	return exitInternal
}

// report 把错误写为一行 JSON 并返回对应的退出码。
func report(env Env, err error) int {
	var ae *apiError
	if errors.As(err, &ae) {
		var compact = ae.body
		if !json.Valid(compact) {
			compact, _ = json.Marshal(problem{Type: "blog-cli:bad-response", Title: "服务端返回了非 JSON 错误",
				Status: ae.status, Detail: string(ae.body)})
		}
		fmt.Fprintln(env.Stderr, string(compactJSON(compact)))
		return exitForStatus(ae.status)
	}
	var p *problem
	if !errors.As(err, &p) {
		p = &problem{Type: "blog-cli:internal", Title: "内部错误", Detail: err.Error(), exit: exitInternal}
	}
	b, _ := json.Marshal(p)
	fmt.Fprintln(env.Stderr, string(b))
	return p.exit
}
