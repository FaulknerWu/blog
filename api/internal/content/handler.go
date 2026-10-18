package content

import (
	"net/http"

	"blog/api/internal/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Renderer 返回当前渲染器版本，客户端据此判断哪些内容由旧版渲染器生成。
func (h *Handler) Renderer(w http.ResponseWriter, r *http.Request) error {
	v, err := h.svc.RendererVersion(r.Context())
	if err != nil {
		return err
	}
	return httpx.WriteJSON(w, http.StatusOK, map[string]string{"renderer_version": v})
}

// Preview 渲染正文供编辑器预览与 CLI 自检，渲染问题随结果返回而不是作为错误。
func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		BodyMD string `json:"body_md"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	p, err := h.svc.Preview(r.Context(), in.BodyMD)
	if err != nil {
		return err
	}
	return httpx.WriteJSON(w, http.StatusOK, p)
}
