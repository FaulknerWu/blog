package httpx

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
)

// Handler 是可返回错误的 http handler，错误统一由 WriteError 转为 problem+json 响应。
type Handler func(w http.ResponseWriter, r *http.Request) error

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h(w, r); err != nil {
		WriteError(w, r, err)
	}
}

func DecodeJSON(r *http.Request, dst any) error {
	if err := requireJSON(r); err != nil {
		return err
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return decodeError(err)
	}
	// 第一个 JSON 值之后不允许再有其他内容。
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return PayloadTooLarge(maxErr.Limit)
		}
		return BadRequest("请求体只能包含一个 JSON 值")
	}
	return nil
}

// DecodeOptionalJSON 与 DecodeJSON 相同，但允许请求体为空，此时 dst 保持不变。
func DecodeOptionalJSON(r *http.Request, dst any) error {
	if r.ContentLength == 0 && r.Header.Get("Content-Type") == "" {
		return nil
	}
	buf, err := io.ReadAll(r.Body)
	if err != nil {
		return decodeError(err)
	}
	if len(bytes.TrimSpace(buf)) == 0 {
		return nil
	}
	r.Body = io.NopCloser(bytes.NewReader(buf))
	return DecodeJSON(r, dst)
}

func requireJSON(r *http.Request) error {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" {
		return UnsupportedMediaType("请求体必须为 application/json")
	}
	return nil
}

func decodeError(err error) error {
	var (
		maxErr    *http.MaxBytesError
		syntaxErr *json.SyntaxError
		typeErr   *json.UnmarshalTypeError
	)
	switch {
	case errors.As(err, &maxErr):
		return PayloadTooLarge(maxErr.Limit)
	case errors.As(err, &syntaxErr):
		return BadRequest(fmt.Sprintf("JSON 语法错误（偏移 %d）", syntaxErr.Offset))
	case errors.As(err, &typeErr):
		return BadRequest(fmt.Sprintf("字段 %q 的类型不正确", typeErr.Field))
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return BadRequest("请求体为空或不完整")
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		return BadRequest("包含未定义的字段 " + strings.TrimPrefix(err.Error(), "json: unknown field "))
	default:
		return BadRequest(err.Error())
	}
}

func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func WriteJSON(w http.ResponseWriter, status int, v any) error {
	body, err := marshal(v)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, err = w.Write(body)
	return err
}

// WritePublicJSON 用于公开接口：按响应内容生成弱 ETag，命中 If-None-Match 时返回 304。
func WritePublicJSON(w http.ResponseWriter, r *http.Request, v any) error {
	body, err := marshal(v)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	etag := `W/"` + hex.EncodeToString(sum[:12]) + `"`
	h := w.Header()
	h.Set("ETag", etag)
	h.Set("Content-Type", "application/json; charset=utf-8")
	if inm := r.Header.Get("If-None-Match"); inm != "" && etagListContains(inm, etag) {
		w.WriteHeader(http.StatusNotModified)
		return nil
	}
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(body)
	return err
}

func etagListContains(header, etag string) bool {
	weakless := strings.TrimPrefix(etag, "W/")
	for part := range strings.SplitSeq(header, ",") {
		part = strings.TrimSpace(part)
		if part == "*" || strings.TrimPrefix(part, "W/") == weakless {
			return true
		}
	}
	return false
}

// Time 在 JSON 中统一输出为 UTC、微秒精度，与 PostgreSQL timestamptz 的精度一致。
type Time time.Time

const timeLayout = "2006-01-02T15:04:05.000000Z"

func NewTime(t time.Time) Time { return Time(t) }

func NewTimePtr(t *time.Time) *Time {
	if t == nil {
		return nil
	}
	v := Time(*t)
	return &v
}

func (t Time) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Time(t).UTC().Format(timeLayout) + `"`), nil
}

func (t Time) String() string { return time.Time(t).UTC().Format(timeLayout) }

func NoContent(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
}
