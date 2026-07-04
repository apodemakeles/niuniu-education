package wordlibrary

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apodemakeles/niuniu-education/backend/internal/config"
	"github.com/apodemakeles/niuniu-education/backend/internal/platform/ocr"
)

// newTestHandler 构造一个完整可用的 handler，数据目录指向临时目录。
func newTestHandler(t *testing.T) (*Handler, *config.Config) {
	t.Helper()
	db := newTestDB(t)
	store := NewStore(db)
	svc := NewService(store, ocr.NewMockProvider(), slog.Default())

	// 临时数据目录，供 fs.SaveImage 使用，测试结束自动清理
	tmpDir := t.TempDir()
	cfg := &config.Config{}
	cfg.DataDir = tmpDir
	return NewHandler(svc, store, db, cfg, slog.Default()), cfg
}

// registerForTest 把 handler 路由挂到一个 test mux 上，返回可发请求的 handler。
func registerForTest(t *testing.T, h *Handler) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	// 用 /api/v1 前缀模拟真实路径
	h.Register(&testRouter{mux: mux, prefix: "/api/v1"})
	return mux
}

type testRouter struct {
	mux    *http.ServeMux
	prefix string
}

func (r *testRouter) Get(p string, h http.HandlerFunc)  { r.mux.HandleFunc(r.prefix+p, h) }
func (r *testRouter) Post(p string, h http.HandlerFunc) { r.mux.HandleFunc(r.prefix+p, h) }

// addImageField 向 multipart 写入一个带正确 image/png Content-Type 的 image 字段。
// 注意：不用 multipart.Writer.CreateFormFile（它写 application/octet-stream），
// 而是手动 CreatePart 设置 image/png，模拟真实浏览器上传图片。
func addImageField(t *testing.T, mw *multipart.Writer, fieldname, filename string, content []byte) {
	t.Helper()
	h := map[string][]string{
		"Content-Disposition": {`form-data; name="` + fieldname + `"; filename="` + filename + `"`},
		"Content-Type":        {"image/png"},
	}
	part, err := mw.CreatePart(h)
	if err != nil {
		t.Fatalf("create part: %v", err)
	}
	part.Write(content)
}

func doRequest(t *testing.T, handler http.Handler, method, path string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode response %q: %v", w.Body.String(), err)
	}
	return m
}

// --- GET /words ---

func TestHandle_ListWords(t *testing.T) {
	h, _ := newTestHandler(t)
	// 预置一条数据
	_, _ = h.store.CreateWord(context.Background(), CreateWordParams{Text: "apple", MeaningZh: "苹果", WordType: TypeNew})

	handler := registerForTest(t, h)
	w := doRequest(t, handler, "GET", "/api/v1/words", nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	m := decodeBody(t, w)
	data := m["data"].([]any)
	if len(data) != 1 {
		t.Errorf("data len = %d, want 1", len(data))
	}
}

// --- POST /imports/ocr (multipart, mock provider) ---

func TestHandle_ImportOCR_Mock(t *testing.T) {
	h, cfg := newTestHandler(t)
	handler := registerForTest(t, h)

	// 构造 multipart 请求：一张假 PNG（模拟真实浏览器上传，带 image/png Content-Type）
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	addImageField(t, mw, "image", "test.png", []byte("\x89PNG\r\n\x1a\n fake png bytes"))
	mw.Close()

	req := httptest.NewRequest("POST", "/api/v1/imports/ocr", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	m := decodeBody(t, w)
	rows := m["rows"].([]any)
	if len(rows) != 3 {
		t.Errorf("rows = %d, want 3 (mock)", len(rows))
	}

	// 验证原图已保存到数据目录
	totalImages := 0
	_ = filepath.Walk(filepath.Join(cfg.DataDir, "images"), func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			totalImages++
		}
		return nil
	})
	if totalImages == 0 {
		t.Error("原图未保存到 data/images/")
	}
}

// --- POST /imports/ocr 缺少 image 字段 ---

func TestHandle_ImportOCR_MissingImage(t *testing.T) {
	h, _ := newTestHandler(t)
	handler := registerForTest(t, h)

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	mw.Close() // 没有 image 字段

	req := httptest.NewRequest("POST", "/api/v1/imports/ocr", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "image") {
		t.Errorf("错误信息应提示缺少 image: %s", w.Body.String())
	}
}

// --- POST /imports/confirm ---

func TestHandle_ConfirmImport(t *testing.T) {
	h, _ := newTestHandler(t)
	handler := registerForTest(t, h)

	payload := `{"rows":[{"text":"apple","meaningZh":"苹果","wordType":"new"}]}`
	w := doRequest(t, handler, "POST", "/api/v1/imports/confirm", strings.NewReader(payload), "application/json")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	m := decodeBody(t, w)
	if int(m["added"].(float64)) != 1 {
		t.Errorf("added = %v, want 1", m["added"])
	}

	// 集成验证：入库后列表能查到
	w2 := doRequest(t, handler, "GET", "/api/v1/words", nil, "")
	m2 := decodeBody(t, w2)
	data := m2["data"].([]any)
	if len(data) != 1 {
		t.Errorf("入库后列表 = %d, want 1", len(data))
	}
}

// --- POST /imports/confirm 重复入库跳过（端到端集成）---

func TestHandle_ConfirmImport_DuplicateSkipped(t *testing.T) {
	h, _ := newTestHandler(t)
	handler := registerForTest(t, h)

	payload := `{"rows":[{"text":"apple","meaningZh":"苹果"}]}`
	// 第一次
	doRequest(t, handler, "POST", "/api/v1/imports/confirm", strings.NewReader(payload), "application/json")
	// 第二次同数据
	w := doRequest(t, handler, "POST", "/api/v1/imports/confirm", strings.NewReader(payload), "application/json")
	m := decodeBody(t, w)
	if int(m["added"].(float64)) != 0 || int(m["skipped"].(float64)) != 1 {
		t.Errorf("got added=%v skipped=%v, want added=0 skipped=1", m["added"], m["skipped"])
	}
}

// --- POST /imports/confirm 请求体格式错误 ---

func TestHandle_ConfirmImport_BadJSON(t *testing.T) {
	h, _ := newTestHandler(t)
	handler := registerForTest(t, h)

	w := doRequest(t, handler, "POST", "/api/v1/imports/confirm", strings.NewReader("not json"), "application/json")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// --- OCR 全链路：OCR 草稿 → 确认入库 → 列表反映（集成测试）---

func TestIntegration_OCRToLibrary(t *testing.T) {
	h, _ := newTestHandler(t)
	handler := registerForTest(t, h)

	// 1. mock OCR 生成草稿
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	addImageField(t, mw, "image", "test.png", []byte("fake"))
	mw.Close()
	req := httptest.NewRequest("POST", "/api/v1/imports/ocr", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("OCR status = %d, body=%s", w.Code, w.Body.String())
	}
	ocrResp := decodeBody(t, w)
	rows := ocrResp["rows"].([]any)
	if len(rows) != 3 {
		t.Fatalf("OCR rows = %d, want 3", len(rows))
	}

	// 2. 把 mock 草稿（clirnb→climb 修正, window, chair）确认入库
	confirmPayload := `{"rows":[
		{"text":"climb","meaningZh":"攀爬","phonetic":"/klaɪm/","wordType":"mistake"},
		{"text":"window","meaningZh":"窗户","wordType":"new"}
	]}`
	w2 := doRequest(t, handler, "POST", "/api/v1/imports/confirm", strings.NewReader(confirmPayload), "application/json")
	if w2.Code != http.StatusOK {
		t.Fatalf("confirm status = %d, body=%s", w2.Code, w2.Body.String())
	}
	m2 := decodeBody(t, w2)
	if int(m2["added"].(float64)) != 2 {
		t.Errorf("added = %v, want 2", m2["added"])
	}

	// 3. 列表应反映新词
	w3 := doRequest(t, handler, "GET", "/api/v1/words", nil, "")
	m3 := decodeBody(t, w3)
	data := m3["data"].([]any)
	if len(data) != 2 {
		t.Errorf("最终列表 = %d, want 2", len(data))
	}
}
