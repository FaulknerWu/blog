package media

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"blog/api/internal/httpx"
	"blog/api/internal/store"
)

const (
	MaxFileSize   = 20 << 20
	MaxUploadBody = MaxFileSize + 1<<20 // 预留 1 MB 给 multipart 边界和 alt 字段
	maxBatchSHA   = 100
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

type refJSON struct {
	Type  string `json:"type"`
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

func refsJSON(refs []store.ListMediaReferencesRow) []refJSON {
	out := make([]refJSON, 0, len(refs))
	for _, r := range refs {
		out = append(out, refJSON{Type: r.OwnerType, ID: r.OwnerID, Title: r.Title})
	}
	return out
}

type mediaJSON struct {
	ID               int64         `json:"id"`
	Sha256           string        `json:"sha256"`
	Width            int32         `json:"width"`
	Height           int32         `json:"height"`
	ByteSize         int64         `json:"byte_size"`
	OriginalFilename string        `json:"original_filename"`
	Alt              string        `json:"alt"`
	Src              string        `json:"src"`
	Variants         []VariantJSON `json:"variants"`
	ReferenceCount   int32         `json:"reference_count"`
	Version          int32         `json:"version"`
	CreatedAt        httpx.Time    `json:"created_at"`
	UpdatedAt        httpx.Time    `json:"updated_at"`
	ReferencedBy     []refJSON     `json:"referenced_by,omitempty"`
}

func toMediaJSON(m store.GetMediaRow) mediaJSON {
	return mediaJSON{
		ID:               m.ID,
		Sha256:           m.Sha256,
		Width:            m.Width,
		Height:           m.Height,
		ByteSize:         m.ByteSize,
		OriginalFilename: m.OriginalFilename,
		Alt:              m.Alt,
		Src:              srcURL(m.Sha256),
		Variants:         variantsJSON(m.Sha256, decodeVariants(m.Variants)),
		ReferenceCount:   m.ReferenceCount,
		Version:          m.Version,
		CreatedAt:        httpx.NewTime(m.CreatedAt),
		UpdatedAt:        httpx.NewTime(m.UpdatedAt),
	}
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) error {
	mr, err := r.MultipartReader()
	if err != nil {
		return httpx.UnsupportedMediaType("请求体必须为 multipart/form-data")
	}
	var (
		data     []byte
		filename string
		alt      string
		gotFile  bool
	)
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return multipartError(err)
		}
		switch part.FormName() {
		case "file":
			if gotFile {
				return httpx.BadRequest("只能上传一个文件")
			}
			gotFile = true
			filename = part.FileName()
			data, err = io.ReadAll(io.LimitReader(part, MaxFileSize+1))
			if err != nil {
				return multipartError(err)
			}
			if len(data) > MaxFileSize {
				return httpx.PayloadTooLarge(MaxFileSize)
			}
		case "alt":
			b, err := io.ReadAll(io.LimitReader(part, 4*maxAltLen+1))
			if err != nil {
				return multipartError(err)
			}
			alt = string(b)
		default:
			return httpx.BadRequest(fmt.Sprintf("包含未定义的字段 %q", part.FormName()))
		}
		part.Close()
	}
	if !gotFile || len(data) == 0 {
		return httpx.Validation(httpx.FieldError{Pointer: "#/file", Code: "required", Detail: "请选择要上传的图片"})
	}
	row, created, err := h.svc.Upload(r.Context(), data, filename, alt)
	if err != nil {
		return err
	}
	httpx.SetETag(w, row.Version)
	status := http.StatusOK
	if created {
		status = http.StatusCreated
		w.Header().Set("Location", fmt.Sprintf("/api/admin/media/%d", row.ID))
	}
	return httpx.WriteJSON(w, status, toMediaJSON(row))
}

func multipartError(err error) error {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return httpx.PayloadTooLarge(maxErr.Limit)
	}
	return httpx.BadRequest("multipart 请求体格式错误")
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	if shas := q["sha256"]; len(shas) > 0 {
		if len(shas) > maxBatchSHA {
			return httpx.Validation(httpx.FieldError{Parameter: "sha256", Code: "too_many", Detail: "一次最多查询 100 个哈希"})
		}
		for _, s := range shas {
			if !sha256Pattern.MatchString(s) {
				return httpx.BadRequest("sha256 必须为 64 位小写十六进制字符串")
			}
		}
		rows, err := h.svc.store.ListMediaBySHA256(r.Context(), shas)
		if err != nil {
			return err
		}
		items := make([]mediaJSON, 0, len(rows))
		for _, row := range rows {
			items = append(items, toMediaJSON(store.GetMediaRow(row)))
		}
		return httpx.WriteJSON(w, http.StatusOK, httpx.Page[mediaJSON]{Items: items})
	}

	limit, err := httpx.QueryLimit(r, 50, 100)
	if err != nil {
		return err
	}
	unused, err := httpx.QueryBool(r, "unused")
	if err != nil {
		return err
	}
	cursor, err := httpx.ParseCursor(r)
	if err != nil {
		return err
	}
	ts, id := cursor.Params()
	rows, err := h.svc.store.ListMedia(r.Context(), store.ListMediaParams{
		Unused: unused, CursorTs: ts, CursorID: id, Lim: limit + 1,
	})
	if err != nil {
		return err
	}
	page := httpx.NewPage(rows, limit,
		func(row store.ListMediaRow) mediaJSON { return toMediaJSON(store.GetMediaRow(row)) },
		func(row store.ListMediaRow) httpx.Cursor { return httpx.Cursor{TS: row.CreatedAt, ID: row.ID} })
	return httpx.WriteJSON(w, http.StatusOK, page)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	m, err := h.svc.Get(r.Context(), id)
	if err != nil {
		return err
	}
	out := toMediaJSON(m.GetMediaRow)
	out.ReferencedBy = refsJSON(m.ReferencedBy)
	httpx.SetETag(w, m.Version)
	return httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Alt *string `json:"alt"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if in.Alt == nil {
		return httpx.Validation(httpx.FieldError{Pointer: "#/alt", Code: "required", Detail: "不能为空"})
	}
	row, err := h.svc.UpdateAlt(r.Context(), id, httpx.ParseIfMatch(r), *in.Alt)
	if err != nil {
		return err
	}
	httpx.SetETag(w, row.Version)
	return httpx.WriteJSON(w, http.StatusOK, toMediaJSON(row))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	if err := h.svc.Delete(r.Context(), id, httpx.ParseIfMatch(r)); err != nil {
		return err
	}
	return httpx.NoContent(w)
}

func (h *Handler) ServeDefault(w http.ResponseWriter, r *http.Request) error {
	return h.serve(w, r, r.PathValue("sha256"), "")
}

func (h *Handler) ServeVariant(w http.ResponseWriter, r *http.Request) error {
	return h.serve(w, r, r.PathValue("sha256"), r.PathValue("file"))
}

func (h *Handler) serve(w http.ResponseWriter, r *http.Request, sha, file string) error {
	if !sha256Pattern.MatchString(sha) {
		return httpx.NotFound("图片不存在")
	}
	row, err := h.svc.store.GetMediaVariantsBySHA256(r.Context(), sha)
	if store.IsNotFound(err) {
		return httpx.NotFound("图片不存在")
	}
	if err != nil {
		return err
	}
	vs := decodeVariants(row.Variants)
	var (
		v  Variant
		ok bool
	)
	if file == "" {
		v, ok = defaultVariant(vs)
	} else {
		v, ok = findVariant(vs, file)
	}
	if !ok {
		return httpx.NotFound("图片规格不存在")
	}
	f, err := h.svc.files.open(sha, v)
	if errors.Is(err, os.ErrNotExist) {
		slog.ErrorContext(r.Context(), "media file missing", "sha256", sha, "width", v.Width)
		return httpx.NotFound("图片不存在")
	}
	if err != nil {
		return err
	}
	defer f.Close()
	hdr := w.Header()
	hdr.Set("Content-Type", "image/"+v.Format)
	// URL 由内容哈希决定，内容不会变，可永久缓存。
	hdr.Set("Cache-Control", "public, max-age=31536000, immutable")
	hdr.Set("ETag", fmt.Sprintf(`"%s-%d"`, sha[:16], v.Width))
	hdr.Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, "", row.UpdatedAt, f)
	return nil
}

func findVariant(vs []Variant, file string) (Variant, bool) {
	ws, format, ok := strings.Cut(file, ".")
	if !ok {
		return Variant{}, false
	}
	width, err := strconv.Atoi(ws)
	// 拒绝 0480.webp 这类非规范写法，保证每个变体只有唯一的 URL。
	if err != nil || strconv.Itoa(width) != ws {
		return Variant{}, false
	}
	for _, v := range vs {
		if v.Width == width && v.Format == format {
			return v, true
		}
	}
	return Variant{}, false
}
