package httpx

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ETag 由资源的 version 列生成强 ETag。写操作通过 If-Match 携带它实现乐观并发控制。
func ETag(version int32) string { return fmt.Sprintf(`"v%d"`, version) }

func SetETag(w http.ResponseWriter, version int32) { w.Header().Set("ETag", ETag(version)) }

type IfMatch struct {
	present bool
	header  string
}

func ParseIfMatch(r *http.Request) IfMatch {
	v := strings.TrimSpace(r.Header.Get("If-Match"))
	return IfMatch{present: v != "", header: v}
}

func (m IfMatch) Present() bool { return m.present }

func (m IfMatch) Require() error {
	if !m.present {
		return PreconditionRequired()
	}
	return nil
}

// Check 校验 If-Match 是否匹配当前版本；未携带 If-Match 时直接放行，强制携带的接口须先调用 Require。
func (m IfMatch) Check(version int32, updatedAt time.Time) error {
	if !m.present {
		return nil
	}
	want := ETag(version)
	for part := range strings.SplitSeq(m.header, ",") {
		part = strings.TrimSpace(part)
		if part == "*" || part == want {
			return nil
		}
	}
	return PreconditionFailed(version, updatedAt)
}

// Cursor 是键集分页游标，以 (时间戳, ID) 定位上一页最后一行，对外编码为 base64url(JSON)。
type Cursor struct {
	TS time.Time
	ID int64
}

type cursorWire struct {
	P string `json:"p"`
	I int64  `json:"i"`
}

func (c Cursor) Encode() string {
	b, _ := json.Marshal(cursorWire{P: NewTime(c.TS).String(), I: c.ID})
	return base64.RawURLEncoding.EncodeToString(b)
}

func ParseCursor(r *http.Request) (*Cursor, error) {
	raw := r.URL.Query().Get("cursor")
	if raw == "" {
		return nil, nil
	}
	bad := BadRequest("分页游标格式非法")
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(raw, "="))
	if err != nil {
		return nil, bad
	}
	var w cursorWire
	if err := json.Unmarshal(b, &w); err != nil || w.I <= 0 {
		return nil, bad
	}
	ts, err := time.Parse(time.RFC3339Nano, w.P)
	if err != nil {
		return nil, bad
	}
	return &Cursor{TS: ts, ID: w.I}, nil
}

func (c *Cursor) Params() (*time.Time, *int64) {
	if c == nil {
		return nil, nil
	}
	return &c.TS, &c.ID
}

type Page[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

// NewPage 组装分页响应。rows 应按 limit+1 查询，多出的一行只用于判断是否存在下一页。
func NewPage[R, T any](rows []R, limit int32, convert func(R) T, cursorOf func(R) Cursor) Page[T] {
	p := Page[T]{Items: make([]T, 0, min(len(rows), int(limit)))}
	if len(rows) > int(limit) {
		rows = rows[:limit]
		next := cursorOf(rows[len(rows)-1]).Encode()
		p.NextCursor = &next
	}
	for _, row := range rows {
		p.Items = append(p.Items, convert(row))
	}
	return p
}

func QueryLimit(r *http.Request, def, maxLimit int32) (int32, error) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, BadRequest("limit 必须为整数")
	}
	if n < 1 || n > int64(maxLimit) {
		return 0, Validation(FieldError{Parameter: "limit", Code: "out_of_range",
			Detail: fmt.Sprintf("取值范围为 1–%d", maxLimit)})
	}
	return int32(n), nil
}

func QueryInt64(r *http.Request, name string) (*int64, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, BadRequest(name + " 必须为整数")
	}
	return &n, nil
}

func QueryBool(r *http.Request, name string) (bool, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return false, BadRequest(name + " 必须为 true 或 false")
	}
	return b, nil
}

func QueryString(r *http.Request, name string) *string {
	if v := r.URL.Query().Get(name); v != "" {
		return &v
	}
	return nil
}

func PathID(r *http.Request, name string) (int64, error) {
	n, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || n <= 0 {
		return 0, NotFound("")
	}
	return n, nil
}

// LikePattern 转义 LIKE 通配符后两端加 %，用于子串匹配。
func LikePattern(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(s) + "%"
}
