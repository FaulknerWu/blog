package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"blog/api/internal/httpx"
	"blog/api/internal/store"
)

const maxAltLen = 500

type Service struct {
	store *store.Store
	files *Storage
	sem   chan struct{} // 限制并发处理的图片数，解码和编码很占 CPU 与内存
}

func NewService(s *store.Store, files *Storage, concurrency int) *Service {
	return &Service{store: s, files: files, sem: make(chan struct{}, max(concurrency, 1))}
}

type Media struct {
	store.GetMediaRow
	ReferencedBy []store.ListMediaReferencesRow
}

func validateAlt(v *httpx.Validator, pointer, alt string) {
	if utf8.RuneCountInString(alt) > maxAltLen {
		v.Add(pointer, "too_long", "长度不能超过 500 个字符")
	}
}

func cleanFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	if name == "." || name == "/" {
		return ""
	}
	if !utf8.ValidString(name) {
		name = strings.ToValidUTF8(name, "")
	}
	if r := []rune(name); len(r) > 255 {
		name = string(r[:255])
	}
	return name
}

// Upload 按内容 SHA-256 去重：相同图片直接返回已有记录。第二个返回值表示是否新建。
func (s *Service) Upload(ctx context.Context, data []byte, filename, alt string) (store.GetMediaRow, bool, error) {
	var v httpx.Validator
	validateAlt(&v, "#/alt", alt)
	if err := v.Err(); err != nil {
		return store.GetMediaRow{}, false, err
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])

	if id, err := s.store.GetMediaIDBySHA256(ctx, sha); err == nil {
		row, err := s.store.GetMedia(ctx, id)
		return row, false, err
	} else if !store.IsNotFound(err) {
		return store.GetMediaRow{}, false, err
	}

	if _, err := sniff(data); err != nil {
		return store.GetMediaRow{}, false, httpx.UnsupportedMediaType("仅支持 JPEG、PNG 与 WebP 图片")
	}
	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
		return store.GetMediaRow{}, false, ctx.Err()
	}
	p, err := process(data)
	<-s.sem
	switch {
	case errors.Is(err, errUnsupportedFormat):
		return store.GetMediaRow{}, false, httpx.UnsupportedMediaType("仅支持 JPEG、PNG 与 WebP 图片")
	case errors.Is(err, errTooManyPixels):
		return store.GetMediaRow{}, false, httpx.Validation(httpx.FieldError{
			Pointer: "#/file", Code: "too_many_pixels", Detail: "图片总像素不能超过 4000 万"})
	case errors.Is(err, errCorruptImage):
		return store.GetMediaRow{}, false, httpx.Validation(httpx.FieldError{
			Pointer: "#/file", Code: "invalid_image", Detail: "图片文件已损坏或无法解码"})
	case err != nil:
		return store.GetMediaRow{}, false, err
	}

	if err := s.files.save(sha, p); err != nil {
		return store.GetMediaRow{}, false, err
	}
	variants, _ := json.Marshal(p.variants)
	id, err := s.store.InsertMedia(ctx, store.InsertMediaParams{
		Sha256:           sha,
		Width:            int32(p.width),
		Height:           int32(p.height),
		ByteSize:         int64(len(data)),
		Variants:         variants,
		OriginalFilename: cleanFilename(filename),
		Alt:              alt,
	})
	created := true
	// ON CONFLICT DO NOTHING 未返回行：并发上传同一图片，另一请求已先插入。
	if store.IsNotFound(err) {
		created = false
		id, err = s.store.GetMediaIDBySHA256(ctx, sha)
	}
	if err != nil {
		// 库中确实没有该记录时才清理文件，避免误删并发请求刚写入的同一份文件。
		if _, lookupErr := s.store.GetMediaIDBySHA256(ctx, sha); store.IsNotFound(lookupErr) {
			_ = s.files.remove(sha)
		}
		return store.GetMediaRow{}, false, err
	}
	row, err := s.store.GetMedia(ctx, id)
	return row, created, err
}

func (s *Service) Get(ctx context.Context, id int64) (Media, error) {
	row, err := s.store.GetMedia(ctx, id)
	if store.IsNotFound(err) {
		return Media{}, httpx.NotFound("图片不存在")
	}
	if err != nil {
		return Media{}, err
	}
	refs, err := s.store.ListMediaReferences(ctx, id)
	if err != nil {
		return Media{}, err
	}
	return Media{GetMediaRow: row, ReferencedBy: refs}, nil
}

func (s *Service) UpdateAlt(ctx context.Context, id int64, ifMatch httpx.IfMatch, alt string) (store.GetMediaRow, error) {
	var v httpx.Validator
	validateAlt(&v, "#/alt", alt)
	if err := v.Err(); err != nil {
		return store.GetMediaRow{}, err
	}
	err := s.store.InTx(ctx, func(q *store.Queries) error {
		cur, err := q.LockMedia(ctx, id)
		if store.IsNotFound(err) {
			return httpx.NotFound("图片不存在")
		}
		if err != nil {
			return err
		}
		if err := ifMatch.Check(cur.Version, cur.UpdatedAt); err != nil {
			return err
		}
		return q.UpdateMediaAlt(ctx, store.UpdateMediaAltParams{ID: id, Alt: alt})
	})
	if err != nil {
		return store.GetMediaRow{}, err
	}
	return s.store.GetMedia(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id int64, ifMatch httpx.IfMatch) error {
	var sha string
	err := s.store.InTx(ctx, func(q *store.Queries) error {
		cur, err := q.LockMedia(ctx, id)
		if store.IsNotFound(err) {
			return httpx.NotFound("图片不存在")
		}
		if err != nil {
			return err
		}
		if err := ifMatch.Check(cur.Version, cur.UpdatedAt); err != nil {
			return err
		}
		refs, err := q.ListMediaReferences(ctx, id)
		if err != nil {
			return err
		}
		if len(refs) > 0 {
			return referencedConflict(refs)
		}
		sha = cur.Sha256
		return q.DeleteMedia(ctx, id)
	})
	// 上面的引用检查之外，外键约束兜底并发新增的引用。
	switch store.ConstraintViolation(err) {
	case "post_media_media_id_fkey", "posts_cover_media_id_fkey", "page_media_media_id_fkey":
		refs, lerr := s.store.ListMediaReferences(ctx, id)
		if lerr != nil {
			return lerr
		}
		return referencedConflict(refs)
	}
	if err != nil {
		return err
	}
	// 文件在事务提交后才删除；删除失败只会留下孤儿文件，不影响数据一致性。
	if err := s.files.remove(sha); err != nil {
		slog.ErrorContext(ctx, "remove media files", "sha256", sha, "err", err)
	}
	return nil
}

func referencedConflict(refs []store.ListMediaReferencesRow) error {
	e := httpx.Conflict("图片仍被文章或页面引用，无法删除")
	e.Extra = map[string]any{"referenced_by": refsJSON(refs)}
	return e
}
