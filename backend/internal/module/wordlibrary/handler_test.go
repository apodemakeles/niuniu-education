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

func (r *testRouter) Get(p string, h http.HandlerFunc)    { r.mux.HandleFunc("GET "+r.prefix+p, h) }
func (r *testRouter) Post(p string, h http.HandlerFunc)   { r.mux.HandleFunc("POST "+r.prefix+p, h) }
func (r *testRouter) Put(p string, h http.HandlerFunc)    { r.mux.HandleFunc("PUT "+r.prefix+p, h) }
func (r *testRouter) Delete(p string, h http.HandlerFunc) { r.mux.HandleFunc("DELETE "+r.prefix+p, h) }

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

// --- POST /words 手动录入 ---

func TestHandle_CreateWord(t *testing.T) {
	h, _ := newTestHandler(t)
	handler := registerForTest(t, h)

	payload := `{"text":"apple","meaningZh":"苹果","wordType":"new"}`
	w := doRequest(t, handler, "POST", "/api/v1/words", strings.NewReader(payload), "application/json")
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", w.Code, w.Body.String())
	}
	m := decodeBody(t, w)
	if m["text"] != "apple" {
		t.Errorf("text = %v", m["text"])
	}
	if m["status"] != StatusUnlearned {
		t.Errorf("默认状态应为 unlearned, got %v", m["status"])
	}
}

func TestHandle_CreateWord_EmptyText(t *testing.T) {
	h, _ := newTestHandler(t)
	handler := registerForTest(t, h)
	w := doRequest(t, handler, "POST", "/api/v1/words", strings.NewReader(`{"text":"","meaningZh":"x"}`), "application/json")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandle_CreateWord_DuplicateConflict(t *testing.T) {
	h, _ := newTestHandler(t)
	handler := registerForTest(t, h)
	payload := `{"text":"apple","meaningZh":"苹果"}`
	// 第一次成功
	doRequest(t, handler, "POST", "/api/v1/words", strings.NewReader(payload), "application/json")
	// 第二次 409
	w := doRequest(t, handler, "POST", "/api/v1/words", strings.NewReader(payload), "application/json")
	if w.Code != http.StatusConflict {
		t.Errorf("重复录入 status = %d, want 409; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "WORD_DUPLICATE") {
		t.Errorf("应返回 WORD_DUPLICATE 错误码: %s", w.Body.String())
	}
}

func TestHandle_CreateWord_ForceOverrideDuplicate(t *testing.T) {
	h, _ := newTestHandler(t)
	handler := registerForTest(t, h)
	payload := `{"text":"apple","meaningZh":"苹果"}`
	doRequest(t, handler, "POST", "/api/v1/words", strings.NewReader(payload), "application/json")
	// 带 force=1，但因 text+meaningZh 完全相同，store 仍会因唯一索引报错——
	// 此测试验证 force 参数能跳过 handler 的 409 拦截（落库错误另算）
	w := doRequest(t, handler, "POST", "/api/v1/words?force=1", strings.NewReader(payload), "application/json")
	// 完全重复时即使 force 也会因唯一约束失败，预期 500（这是合理的，force 主要用于 text 相同释义不同的场景）
	if w.Code == http.StatusConflict {
		t.Errorf("force=1 不应再返回 409")
	}
}

// --- PUT /words/{id} 编辑 ---

func TestHandle_UpdateWord(t *testing.T) {
	h, _ := newTestHandler(t)
	handler := registerForTest(t, h)
	// 先建一个
	w0 := doRequest(t, handler, "POST", "/api/v1/words", strings.NewReader(`{"text":"apple","meaningZh":"苹果","wordType":"new"}`), "application/json")
	id := decodeBody(t, w0)["id"].(string)

	payload := `{"meaningZh":"苹果果","phonetic":"/ˈæpl/","wordType":"mistake","status":"reinforce"}`
	w := doRequest(t, handler, "PUT", "/api/v1/words/"+id, strings.NewReader(payload), "application/json")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	m := decodeBody(t, w)
	if m["meaningZh"] != "苹果果" || m["wordType"] != "mistake" {
		t.Errorf("更新后字段不符: %v", m)
	}
}

func TestHandle_UpdateWord_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	handler := registerForTest(t, h)
	w := doRequest(t, handler, "PUT", "/api/v1/words/nope", strings.NewReader(`{"meaningZh":"x","wordType":"new","status":"unlearned"}`), "application/json")
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// --- DELETE /words/{id} ---

func TestHandle_DeleteWord_Physical(t *testing.T) {
	h, _ := newTestHandler(t)
	handler := registerForTest(t, h)
	w0 := doRequest(t, handler, "POST", "/api/v1/words", strings.NewReader(`{"text":"apple","meaningZh":"苹果","wordType":"new"}`), "application/json")
	id := decodeBody(t, w0)["id"].(string)

	w := doRequest(t, handler, "DELETE", "/api/v1/words/"+id, nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	m := decodeBody(t, w)
	if m["kind"] != "physical" {
		t.Errorf("未学词应物理删除, kind = %v", m["kind"])
	}
}

func TestHandle_DeleteWord_Logical(t *testing.T) {
	h, _ := newTestHandler(t)
	handler := registerForTest(t, h)
	// 建一个并改为"学习中"
	w0 := doRequest(t, handler, "POST", "/api/v1/words", strings.NewReader(`{"text":"apple","meaningZh":"苹果","wordType":"new"}`), "application/json")
	id := decodeBody(t, w0)["id"].(string)
	doRequest(t, handler, "PUT", "/api/v1/words/"+id, strings.NewReader(`{"meaningZh":"苹果","phonetic":"","wordType":"new","status":"learning"}`), "application/json")

	w := doRequest(t, handler, "DELETE", "/api/v1/words/"+id, nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	m := decodeBody(t, w)
	if m["kind"] != "logical" {
		t.Errorf("学习中应软删, kind = %v", m["kind"])
	}
}

func TestHandle_DeleteWord_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	handler := registerForTest(t, h)
	w := doRequest(t, handler, "DELETE", "/api/v1/words/nope", nil, "")
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// --- GET /words/{id} ---

func TestHandle_GetWord(t *testing.T) {
	h, _ := newTestHandler(t)
	handler := registerForTest(t, h)
	w0 := doRequest(t, handler, "POST", "/api/v1/words", strings.NewReader(`{"text":"apple","meaningZh":"苹果","wordType":"new"}`), "application/json")
	id := decodeBody(t, w0)["id"].(string)

	w := doRequest(t, handler, "GET", "/api/v1/words/"+id, nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	m := decodeBody(t, w)
	if m["text"] != "apple" {
		t.Errorf("text = %v", m["text"])
	}
}

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
