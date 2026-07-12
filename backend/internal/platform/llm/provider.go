// Package llm 提供文本大模型调用能力（与 ocr 包并列，专做文本生成）。
//
// 设计为 Provider 适配层：业务层只依赖 Provider 接口，返回纯文本。
// 第一版通过 SiliconFlow 的 OpenAI 兼容接口调用文本模型（与 OCR 同平台，可共用 apiKey）。
package llm

import "context"

// Provider 文本生成 provider。
type Provider interface {
	// Name 返回 provider 标识（mock / siliconflow / ...）。
	Name() string
	// Generate 用 system+user prompt 调用文本模型，返回生成的文本。
	Generate(ctx context.Context, system, user string) (string, error)
}
