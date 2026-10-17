package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

type problemBaseKey struct{}

func WithProblemBase(base string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), problemBaseKey{}, base)))
	})
}

func problemBase(ctx context.Context) string {
	if base, ok := ctx.Value(problemBaseKey{}).(string); ok {
		return base
	}
	return "about:blank#"
}

type FieldError struct {
	Pointer   string `json:"pointer,omitempty"`
	Parameter string `json:"parameter,omitempty"`
	Code      string `json:"code"`
	Detail    string `json:"detail"`
	Line      int    `json:"line,omitempty"` // 正文渲染错误所在的 Markdown 行号
}

// Error 对应 RFC 9457 problem details 响应，Kind 拼接在 problem base 之后作为 type 字段。
type Error struct {
	Status int
	Kind   string
	Title  string
	Detail string
	Errors []FieldError
	Extra  map[string]any
	Header http.Header
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s", e.Kind, e.Detail)
	}
	return e.Kind
}

func newError(status int, kind, title, detail string) *Error {
	return &Error{Status: status, Kind: kind, Title: title, Detail: detail}
}

func BadRequest(detail string) *Error {
	return newError(http.StatusBadRequest, "malformed-request", "请求格式错误", detail)
}

func Unauthenticated(detail string) *Error {
	return newError(http.StatusUnauthorized, "unauthenticated", "未登录或会话已失效", detail)
}

func Forbidden(detail string) *Error {
	return newError(http.StatusForbidden, "forbidden", "禁止访问", detail)
}

func NotFound(detail string) *Error {
	return newError(http.StatusNotFound, "not-found", "资源不存在", detail)
}

func MethodNotAllowed(allow string) *Error {
	e := newError(http.StatusMethodNotAllowed, "method-not-allowed", "请求方法不受支持", "")
	if allow != "" {
		e.Header = http.Header{"Allow": {allow}}
	}
	return e
}

func Conflict(detail string, errs ...FieldError) *Error {
	e := newError(http.StatusConflict, "conflict", "资源冲突", detail)
	e.Errors = errs
	return e
}

func PreconditionFailed(currentVersion int32, updatedAt time.Time) *Error {
	e := newError(http.StatusPreconditionFailed, "precondition-failed", "资源版本不匹配", "资源已被修改，请刷新后重试")
	e.Extra = map[string]any{
		"current_etag": ETag(currentVersion),
		"updated_at":   NewTime(updatedAt),
	}
	return e
}

func PreconditionRequired() *Error {
	return newError(http.StatusPreconditionRequired, "precondition-required", "缺少 If-Match 请求头", "修改该资源必须携带 If-Match 请求头")
}

func PayloadTooLarge(limit int64) *Error {
	return newError(http.StatusRequestEntityTooLarge, "payload-too-large", "请求体过大",
		fmt.Sprintf("请求体不能超过 %d 字节", limit))
}

func UnsupportedMediaType(detail string) *Error {
	return newError(http.StatusUnsupportedMediaType, "unsupported-media-type", "不支持的媒体类型", detail)
}

func Validation(errs ...FieldError) *Error {
	e := newError(http.StatusUnprocessableEntity, "validation-error", "请求参数校验失败",
		fmt.Sprintf("请求中有 %d 个字段不符合要求", len(errs)))
	e.Errors = errs
	return e
}

func Unprocessable(detail string) *Error {
	return newError(http.StatusUnprocessableEntity, "validation-error", "请求参数校验失败", detail)
}

func RateLimited(retryAfter time.Duration) *Error {
	secs := max(int(retryAfter.Round(time.Second)/time.Second), 1)
	e := newError(http.StatusTooManyRequests, "rate-limited", "请求过于频繁", "请稍后再试")
	e.Header = http.Header{"Retry-After": {strconv.Itoa(secs)}}
	return e
}

func ServiceUnavailable(detail string) *Error {
	return newError(http.StatusServiceUnavailable, "service-unavailable", "服务暂不可用", detail)
}

func Internal() *Error {
	return newError(http.StatusInternalServerError, "internal-error", "服务器内部错误", "")
}

// AsError 把 err 转为 *Error；第二个返回值为 false 表示未预期的错误，按 500 处理。
func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return PayloadTooLarge(maxErr.Limit), true
	}
	return Internal(), false
}

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	e, known := AsError(err)
	if !known {
		slog.ErrorContext(r.Context(), "unhandled error",
			"err", err, "method", r.Method, "path", r.URL.Path, "request_id", RequestID(r.Context()))
	}
	body := map[string]any{
		"type":       problemBase(r.Context()) + e.Kind,
		"title":      e.Title,
		"status":     e.Status,
		"instance":   r.URL.Path,
		"request_id": RequestID(r.Context()),
	}
	if e.Detail != "" {
		body["detail"] = e.Detail
	}
	if len(e.Errors) > 0 {
		body["errors"] = e.Errors
	}
	for k, v := range e.Extra {
		body[k] = v
	}
	h := w.Header()
	for k, vs := range e.Header {
		h[k] = vs
	}
	// handler 可能在出错前已设置 ETag，错误响应不能带上它。
	h.Del("ETag")
	h.Set("Content-Type", "application/problem+json")
	if h.Get("Cache-Control") == "" {
		h.Set("Cache-Control", "no-store")
	}
	w.WriteHeader(e.Status)
	_ = json.NewEncoder(w).Encode(body)
}
