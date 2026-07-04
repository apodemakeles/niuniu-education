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
	Put(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
}

func (h *Handler) Register(r Router) {
	r.Get("/words", h.handleListWords)
	r.Get("/words/{id}", h.handleGetWord)
	r.Post("/words", h.handleCreateWord)
	r.Put("/words/{id}", h.handleUpdateWord)
	r.Delete("/words/{id}", h.handleDeleteWord)
	r.Post("/imports/parse", h.handleImportParse)
	r.Post("/imports/ocr", h.handleImportOCR)
	r.Post("/imports/confirm", h.handleConfirmImport)
	r.Post("/exports/dictation:preview", h.handleExportPreview)
	r.Post("/exports/dictation", h.handleExportDoc)
}

// RegisterTestEndpoints 注册仅测试用的端点（物理清空库等）。
// 仅在 NIUNIU_ENABLE_TEST_ENDPOINTS=1 时由 main 调用，生产路由表不含这些端点。
func (h *Handler) RegisterTestEndpoints(r Router) {
	r.Post("/_test/reset", h.handleTestReset)
}

func (h *Handler) handleTestReset(w http.ResponseWriter, r *http.Request) {
	if err := h.store.ResetAll(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "重置失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reset": true})
}

func (h *Handler) handleGetWord(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	w0, err := h.store.Get(r.Context(), id)
	if err != nil {
		if err == ErrNotFound {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "单词不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "查询单词失败")
		return
	}
	writeJSON(w, http.StatusOK, w0)
}

// handleCreateWord 手动逐个录入。
func (h *Handler) handleCreateWord(w http.ResponseWriter, r *http.Request) {
	var req CreateWordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "请求体格式错误")
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "英文单词不能为空")
		return
	}
	if strings.TrimSpace(req.MeaningZh) == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "中文释义不能为空")
		return
	}

	// 重复提示（text+meaningZh 命中）：PRD 要求优先提示，不强拦。
	// 手动录入返回 409 让前端二次确认；前端确认后可带 force 重新提交。
	exists, err := h.store.ExistsByTextMeaning(r.Context(), req.Text, req.MeaningZh)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "查重失败")
		return
	}
	if exists && r.URL.Query().Get("force") != "1" {
		writeError(w, http.StatusConflict, "WORD_DUPLICATE", "词库内已存在该单词")
		return
	}

	w0, err := h.store.CreateWord(r.Context(), CreateWordParams{
		Text:      strings.TrimSpace(req.Text),
		MeaningZh: strings.TrimSpace(req.MeaningZh),
		Phonetic:  req.Phonetic,
		WordType:  req.WordType,
		Source:    SourceManual,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "新增单词失败")
		h.logger.Error("create word", slog.Any("err", err))
		return
	}
	writeJSON(w, http.StatusCreated, w0)
}

// handleUpdateWord 编辑单词属性（不含 text）。
func (h *Handler) handleUpdateWord(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req UpdateWordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "请求体格式错误")
		return
	}
	w0, err := h.store.Update(r.Context(), UpdateParams{
		ID:        id,
		MeaningZh: req.MeaningZh,
		Phonetic:  req.Phonetic,
		WordType:  req.WordType,
		Status:    req.Status,
	})
	if err != nil {
		if err == ErrNotFound {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "单词不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "更新失败")
		return
	}
	writeJSON(w, http.StatusOK, w0)
}

// handleDeleteWord 删除单词（按 status 走物理/逻辑删除）。
func (h *Handler) handleDeleteWord(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	res, err := h.store.Delete(r.Context(), id)
	if err != nil {
		if err == ErrNotFound {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "单词不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "删除失败")
		return
	}
	writeJSON(w, http.StatusOK, DeleteWordResponse{Kind: string(res.Kind)})
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

// handleImportParse 解析粘贴的文本为草稿行（不入库）。
func (h *Handler) handleImportParse(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "请求体格式错误")
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "文本不能为空")
		return
	}
	rows := h.svc.ParsePasteForDraft(req.Text)
	writeJSON(w, http.StatusOK, map[string]any{"rows": rows})
}

// handleExportPreview 生成默写表预览数据（前端渲染表格）。
func (h *Handler) handleExportPreview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Scope string `json:"scope"` // all / unlearned / learning / reinforce / mastered
		Title string `json:"title"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req) // body 可选
	preview, err := h.BuildDictationPreview(r.Context(), req.Scope, req.Title)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "生成预览失败")
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

// handleExportDoc 生成 Word 兼容 HTML 并以 .doc 下载。
// 家长可在前端预览阶段临时修改中文，修改内容不回写词库，仅当次导出生效。
func (h *Handler) handleExportDoc(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string           `json:"title"`
		Items []map[string]any `json:"items"` // 预览阶段可能被编辑过的中文
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "请求体格式错误")
		return
	}
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "没有可导出的内容")
		return
	}
	items := parseExportItems(req.Items)
	doc := BuildDictationDoc(req.Title, items)

	w.Header().Set("Content-Type", "application/msword; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filenameFromTitle(req.Title)+`"; filename*=UTF-8''`+filenameFromTitle(req.Title))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(doc))
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
