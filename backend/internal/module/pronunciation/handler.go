package pronunciation

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"

	provider "github.com/apodemakeles/niuniu-education/backend/internal/platform/pronunciation"
)

type Router interface {
	Get(string, http.HandlerFunc)
}
type Handler struct {
	svc    *Service
	locale string
}

func NewHandler(s *Service, locale string) *Handler { return &Handler{svc: s, locale: locale} }
func (h *Handler) Register(r Router) {
	r.Get("/pronunciations/{wordId}", h.metadata)
	r.Get("/pronunciations/{wordId}/audio", h.audio)
}
func (h *Handler) metadata(w http.ResponseWriter, r *http.Request) {
	rec, err := h.svc.Resolve(r.Context(), r.PathValue("wordId"), h.locale)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, Response{WordID: rec.WordID, Locale: rec.Locale, Provider: rec.Provider, Phonetic: rec.Phonetic, AudioURL: "/api/v1/pronunciations/" + rec.WordID + "/audio", SourceURL: rec.SourceURL, LicenseName: rec.LicenseName, LicenseURL: rec.LicenseURL, Attribution: rec.Attribution})
}
func (h *Handler) audio(w http.ResponseWriter, r *http.Request) {
	rec, err := h.svc.Resolve(r.Context(), r.PathValue("wordId"), h.locale)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=86400")
	if rec.MIMEType != "" {
		w.Header().Set("Content-Type", rec.MIMEType)
	}
	http.ServeFile(w, r, rec.FilePath)
}
func writeErr(w http.ResponseWriter, err error) {
	slog.Error("resolve pronunciation", slog.Any("err", err))
	status, code, msg := http.StatusServiceUnavailable, "PRONUNCIATION_UPSTREAM", "发音来源暂时不可用，请稍后再试"
	if errors.Is(err, provider.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		status, code, msg = http.StatusNotFound, "PRONUNCIATION_NOT_FOUND", "暂时没有找到英式发音"
	}
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
