package studentwordtask

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/apodemakeles/niuniu-education/backend/internal/config"
)

// Handler 学生端任务前台 HTTP 请求处理。
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

// Router 与 wordlibrary 对齐的最小路由接口（chi.Router 满足）。
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
}

func (h *Handler) Register(r Router) {
	r.Get("/practice/today", h.handleGetToday)
	r.Post("/practice/cards/{wordId}/enter", h.handleEnterCard)
	r.Post("/practice/cards/{wordId}/listen", h.handleListen)
	r.Post("/practice/cards/{wordId}/read-done", h.handleReadDone)
	r.Post("/practice/cards/{wordId}/hard", h.handleHard)
	r.Get("/practice/reading", h.handleGetReading)
	r.Post("/practice/reading/regenerate", h.handleRegenerateReading)
	r.Post("/practice/reading/complete", h.handleCompleteReading)
	r.Get("/practice/dictation", h.handleGetDictation)
	r.Post("/practice/dictation/submit", h.handleSubmitDictation)
	r.Post("/practice/dictation/correct", h.handleCorrectDictation)
	r.Get("/practice/done", h.handleGetDone)
	r.Post("/practice/checkin", h.handleCheckin)
}

// RegisterTestEndpoints 仅测试用（物理清空学生端数据），与 wordlibrary 同惯例。
func (h *Handler) RegisterTestEndpoints(r Router) {
	r.Post("/_test/reset-practice", h.handleTestReset)
}

func (h *Handler) handleTestReset(w http.ResponseWriter, r *http.Request) {
	if err := h.store.ResetAll(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "重置失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reset": true})
}

func (h *Handler) handleGetToday(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetToday(r.Context())
	if err != nil {
		h.logger.Error("get today", slog.Any("err", err))
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "加载今日任务失败")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleEnterCard(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.EnterCard(r.Context(), r.PathValue("wordId"))
	if err != nil {
		writePracticeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleListen(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.MarkListened(r.Context(), r.PathValue("wordId"))
	if err != nil {
		writePracticeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleReadDone(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.MarkReadDone(r.Context(), r.PathValue("wordId"))
	if err != nil {
		writePracticeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleHard(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.MarkHard(r.Context(), r.PathValue("wordId"))
	if err != nil {
		writePracticeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleGetReading(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetReading(r.Context())
	if err != nil {
		h.logger.Error("get reading", slog.Any("err", err))
		writeErr(w, http.StatusInternalServerError, "READING_ERROR", "加载阅读短文失败")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleRegenerateReading(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.RegenerateReading(r.Context())
	if err != nil {
		h.logger.Error("regenerate reading", slog.Any("err", err))
		writeErr(w, http.StatusInternalServerError, "READING_ERROR", "重新生成短文失败")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleCompleteReading(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.CompleteReading(r.Context()); err != nil {
		switch {
		case errors.Is(err, ErrReadingTooShort):
			writeErr(w, http.StatusConflict, "READING_TOO_SHORT", err.Error())
		case errors.Is(err, ErrReadingNotStarted):
			writeErr(w, http.StatusConflict, "READING_NOT_STARTED", err.Error())
		case errors.Is(err, ErrReadingNotReady):
			writeErr(w, http.StatusConflict, "READING_NOT_READY", err.Error())
		default:
			h.logger.Error("complete reading", slog.Any("err", err))
			writeErr(w, http.StatusInternalServerError, "READING_ERROR", "完成阅读失败")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"done": true})
}

func (h *Handler) handleGetDictation(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetDictation(r.Context())
	if err != nil {
		writePracticeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleSubmitDictation(w http.ResponseWriter, r *http.Request) {
	var req SubmitDictationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "请求体格式错误")
		return
	}
	resp, err := h.svc.SubmitDictation(r.Context(), req)
	if err != nil {
		writePracticeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleCorrectDictation(w http.ResponseWriter, r *http.Request) {
	var req CorrectDictationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "请求体格式错误")
		return
	}
	resp, err := h.svc.CorrectDictation(r.Context(), req)
	if err != nil {
		writePracticeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleGetDone(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetDone(r.Context())
	if err != nil {
		writePracticeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleCheckin(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Checkin(r.Context())
	if err != nil {
		writePracticeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// writePracticeErr 把 service 的领域错误映射为合适的 HTTP 状态。
func writePracticeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotInTodayTask):
		writeErr(w, http.StatusNotFound, "NOT_IN_TASK", err.Error())
	case errors.Is(err, ErrAlreadyLocked):
		writeErr(w, http.StatusConflict, "ALREADY_LOCKED", err.Error())
	case errors.Is(err, ErrLearningNotFound):
		writeErr(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	default:
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"code": code, "message": msg},
	})
}
