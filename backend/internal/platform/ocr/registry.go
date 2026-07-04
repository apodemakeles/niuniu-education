package ocr

import (
	"fmt"

	"github.com/apodemakeles/niuniu-education/backend/internal/config"
)

// New 按 config.OCR.Provider 返回对应的 Provider 实现。
func New(cfg config.OCRConfig) (Provider, error) {
	switch cfg.Provider {
	case "mock":
		return NewMockProvider(), nil
	case "deepseek":
		return NewDeepSeekProvider(cfg.Endpoint, cfg.APIKey, cfg.Model), nil
	default:
		return nil, fmt.Errorf("未知的 ocr provider: %s（可选 mock / deepseek）", cfg.Provider)
	}
}
