package ocr

import "context"

// MockProvider 是离线兜底实现，复刻原型 app.js 的 mock 数据。
//
// 返回固定 3 行示例（含一个低置信度词 clirnb 供家长修正演示），
// 让前端在没有真实 API Key 时也能跑通完整拍照导入流程。
type MockProvider struct{}

func NewMockProvider() *MockProvider { return &MockProvider{} }

func (m *MockProvider) Name() string { return "mock" }

func (m *MockProvider) Recognize(_ context.Context, _ []byte, _ string) (*Result, error) {
	return &Result{
		Rows: []DraftRow{
			{Text: "clirnb", MeaningZh: "攀爬", Phonetic: "/klaɪm/", WordType: "mistake", Confidence: 0.62},
			{Text: "window", MeaningZh: "窗户", Phonetic: "/ˈwɪndoʊ/", WordType: "new", Confidence: 0.91},
			{Text: "chair", MeaningZh: "椅子", Phonetic: "/tʃer/", WordType: "new", Confidence: 0.87},
		},
		RawText: "[mock OCR] 这是由 MockProvider 返回的示例数据。\nclirnb /klaɪm/ 攀爬\nwindow /ˈwɪndoʊ/ 窗户\nchair /tʃer/ 椅子",
	}, nil
}
