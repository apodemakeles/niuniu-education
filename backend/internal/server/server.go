package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/apodemakeles/niuniu-education/backend/internal/config"
	"github.com/apodemakeles/niuniu-education/backend/internal/version"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// SubRouter 是业务模块挂载路由用的最小接口（Get/Post/Put/Delete）。
// chi.Router 天然满足该接口，wordlibrary.Handler.Register 接收同名接口。
type SubRouter interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
}

// Server 负责 HTTP 路由装配。业务路由由各模块 Handler 自行注册。
type Server struct {
	cfg    *config.Config
	logger *slog.Logger
	// routeRegistrar 让 main 注入业务模块的路由注册函数
	routeRegistrar func(SubRouter)
}

func New(cfg *config.Config, logger *slog.Logger, registrar func(SubRouter)) *Server {
	return &Server{cfg: cfg, logger: logger, routeRegistrar: registrar}
}

// Router 装配全部中间件与路由。
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(recoverer(s.logger))
	r.Use(logger(s.logger))

	if s.cfg.Server.CORS.Enabled {
		r.Use(s.corsMiddleware())
	}

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", s.handleHealth)
		// 业务模块（wordlibrary 等）的路由挂载由 main 注入
		if s.routeRegistrar != nil {
			s.routeRegistrar(r)
		}
	})

	r.NotFound(s.handleNotFound)
	r.MethodNotAllowed(s.handleMethodNotAllowed)
	return r
}

func (s *Server) corsMiddleware() func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   s.cfg.Server.CORS.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": version.Version,
	})
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "NOT_FOUND", "接口不存在")
}

func (s *Server) handleMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "请求方法不被允许")
}

// --- 响应工具 ---

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
