package llm

import "context"

// MockProvider 测试用文本 provider：按回调返回固定文本。
type MockProvider struct {
	Text string
	Err  error
}

func NewMockProvider(text string) *MockProvider { return &MockProvider{Text: text} }

func (p *MockProvider) Name() string { return "mock" }

func (p *MockProvider) Generate(ctx context.Context, system, user string) (string, error) {
	if p.Err != nil {
		return "", p.Err
	}
	return p.Text, nil
}
