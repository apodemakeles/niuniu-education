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

// Server 持有 HTTP 服务所需依赖。M0 仅含健康检查与临时单词列表用于联调。
type Server struct {
	cfg    *config.Config
	logger *slog.Logger
}

func New(cfg *config.Config, logger *slog.Logger) *Server {
	return &Server{cfg: cfg, logger: logger}
}

// Router 装配全部路由与中间件。
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
		// 临时联调端点：返回预置单词，前端 M0 即可看到首屏数据。
		r.Get("/words", s.handleListWords)
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

// --- handlers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	b := errorBody{}
	b.Error.Code = code
	b.Error.Message = msg
	writeJSON(w, status, b)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": version.Version,
	})
}

// TODO(M1): 由 wordlibrary 模块接管，下面为 M0 联调用临时实现。
func (s *Server) handleListWords(w http.ResponseWriter, r *http.Request) {
	// 复刻原型 app.js 中的示例数据
	words := []map[string]any{
		{"id": "w-1", "text": "apple", "meaningZh": "苹果", "phonetic": "/ˈæpl/", "wordType": "new", "status": "unlearned"},
		{"id": "w-2", "text": "read", "meaningZh": "阅读", "phonetic": "/riːd/", "wordType": "mistake", "status": "reinforce"},
		{"id": "w-3", "text": "desk", "meaningZh": "书桌", "phonetic": "/desk/", "wordType": "new", "status": "learning"},
		{"id": "w-4", "text": "climb", "meaningZh": "攀爬", "phonetic": "/klaɪm/", "wordType": "mistake", "status": "reinforce"},
		{"id": "w-5", "text": "water", "meaningZh": "水", "phonetic": "/ˈwɔːtər/", "wordType": "new", "status": "mastered"},
		{"id": "w-6", "text": "their", "meaningZh": "他们的", "phonetic": "/ðer/", "wordType": "mistake", "status": "reinforce"},
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": words})
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "NOT_FOUND", "接口不存在")
}

func (s *Server) handleMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "请求方法不被允许")
}
