package wordlibrary

import (
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/apodemakeles/niuniu-education/backend/internal/config"
	"github.com/apodemakeles/niuniu-education/backend/internal/platform/fs"
)

const (
	maxImageSize = 10 << 20 // 10MB
)

// Handler 处理单词库相关的 HTTP 请求。
type Handler struct {
	svc    *Service
	store  *Store
	db     *sql.DB
	cfg    *config.Config
	logger *slog.Logger
}

func NewHandler(svc *Service, store *Store, db *sql.DB, cfg *config.Config, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, store: store, db: db, cfg: cfg, logger: logger}
}

// Register 把路由挂到路由器（通过接口解耦对 chi 的直接依赖）。
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
}

func (h *Handler) Register(r Router) {
	r.Get("/words", h.handleListWords)
	r.Post("/imports/ocr", h.handleImportOCR)
	r.Post("/imports/confirm", h.handleConfirmImport)
}

func (h *Handler) handleListWords(w http.ResponseWriter, r *http.Request) {
	words, err := h.store.List(r.Context(), ListParams{
		Type:   r.URL.Query().Get("type"),
		Status: r.URL.Query().Get("status"),
		Q:      r.URL.Query().Get("q"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "查询单词失败")
		h.logger.Error("list words", slog.Any("err", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": words})
}

// handleImportOCR 接收图片，调 OCR 生成草稿行（不入库）。
func (h *Handler) handleImportOCR(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxImageSize); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "解析表单失败，请确认上传的是图片文件")
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "缺少 image 字段")
		return
	}
	defer file.Close()

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = sniffImageType(header.Filename)
	}
	if !strings.HasPrefix(mimeType, "image/") {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "仅支持图片文件")
		return
	}

	imageBytes, err := io.ReadAll(io.LimitReader(file, maxImageSize+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "读取图片失败")
		return
	}
	if len(imageBytes) > maxImageSize {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "图片不能超过 10MB")
		return
	}

	// 调 OCR 生成草稿
	rows, rawText, err := h.svc.RecognizeForDraft(r.Context(), imageBytes, mimeType)
	if err != nil {
		writeError(w, http.StatusBadGateway, "OCR_ERROR", err.Error())
		h.logger.Error("ocr recognize", slog.Any("err", err))
		return
	}

	// 保存原图与 OCR 记录（便于排查）。失败仅记日志，不阻塞草稿返回。
	imageID := h.saveImageRecord(r, imageBytes, mimeType, rawText)

	writeJSON(w, http.StatusOK, OCRDraftResponse{
		ImageID: imageID,
		Rows:    rows,
		RawText: rawText,
	})
}

// handleConfirmImport 确认草稿入库。
func (h *Handler) handleConfirmImport(w http.ResponseWriter, r *http.Request) {
	var req ConfirmImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "请求体格式错误")
		return
	}
	resp, err := h.svc.ConfirmImport(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		h.logger.Error("confirm import", slog.Any("err", err))
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// saveImageRecord 保存原图到 data/images 并写 import_images 记录。
func (h *Handler) saveImageRecord(r *http.Request, image []byte, mimeType, rawText string) string {
	relPath, err := fs.SaveImage(h.cfg, image, mimeType)
	if err != nil {
		h.logger.Warn("save image file", slog.Any("err", err))
		relPath = ""
	}

	var imageID string
	err = h.db.QueryRowContext(r.Context(), `
		INSERT INTO import_images (id, library_id, file_path, mime_type, ocr_raw_text, provider, status)
		VALUES (lower(hex(randomblob(8))), 'main-library', ?, ?, ?, ?, 'confirmed')
		RETURNING id`,
		relPath, mimeType, truncateStr(rawText, 5000), h.svc.ocr.Name()).Scan(&imageID)
	if err != nil {
		h.logger.Warn("insert import_images", slog.Any("err", err))
	}
	return imageID
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func sniffImageType(filename string) string {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// --- 响应工具（避免与 server 包循环依赖，内联）---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"code": code, "message": msg},
	})
}
