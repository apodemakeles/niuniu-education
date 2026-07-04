// Package ocr 提供图片文字识别能力。
//
// 设计为 Provider 适配层，把不同 OCR 厂商（DeepSeek-OCR、Mock 等）解耦：
// 业务层只依赖 Provider 接口，返回统一的草稿行结构。
// 调用方（导入 service）负责后续的去重、入库等业务逻辑。
package ocr

import "context"

// DraftRow 是 OCR/解析共同产出的草稿行，与前端 DraftRowDTO 对齐。
// WordType 默认 "new"，由调用方按上下文覆盖（如从易错词入口默认 mistake）。
type DraftRow struct {
	Text       string  `json:"text"`
	MeaningZh  string  `json:"meaningZh"`
	Phonetic   string  `json:"phonetic"`
	WordType   string  `json:"wordType"`
	Confidence float64 `json:"confidence"` // 0~1；mock 用，deepseek 暂填 1.0
}

// Result 是一次识别的完整结果。
type Result struct {
	Rows    []DraftRow `json:"rows"`
	RawText string     `json:"rawText"` // OCR 原始文本，存 import_images.ocr_raw_text 便于排查
}

// Provider 适配不同 OCR 厂商。
type Provider interface {
	// Name 返回 provider 标识（mock / deepseek / ...），用于记录与排查。
	Name() string
	// Recognize 对图片字节执行识别，返回结构化草稿。
	// image 为原始字节，mimeType 形如 "image/png"。
	Recognize(ctx context.Context, image []byte, mimeType string) (*Result, error)
}
