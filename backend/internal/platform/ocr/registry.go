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
	case "siliconflow":
		// siliconflow provider 通过 OpenAI 兼容接口调用硅基流动上的视觉模型，
		// model 决定具体用哪个（Qwen3-VL / DeepSeek-OCR / PaddleOCR-VL 等）。
		return NewSiliconFlowProvider(cfg.Endpoint, cfg.APIKey, cfg.Model), nil
	case "deepseek":
		// 向后兼容：旧配置里写的 "deepseek" 仍可用，等价于 siliconflow。
		return NewSiliconFlowProvider(cfg.Endpoint, cfg.APIKey, cfg.Model), nil
	default:
		return nil, fmt.Errorf("未知的 ocr provider: %s（可选 mock / siliconflow）", cfg.Provider)
	}
}
