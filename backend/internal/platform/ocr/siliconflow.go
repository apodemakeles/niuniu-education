package ocr

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SiliconFlowProvider 通过 OpenAI 兼容接口调用硅基流动平台上的视觉模型。
//
// 平台上的模型统一用 OpenAI 兼容接口，按 model 名自动选择合适的 prompt：
//   - DeepSeek-OCR 系列：用 "Free OCR."（OCR 专用指令）
//   - Qwen-VL / 其他通用视觉模型：用结构化识别 prompt
//
// 经实测：DeepSeek-OCR 对竖版/复杂教材图返回乱码，Qwen3-VL-32B 能稳定识别。
type SiliconFlowProvider struct {
	endpoint string // 形如 https://api.siliconflow.cn/v1
	apiKey   string
	model    string // 形如 Qwen/Qwen3-VL-32B-Instruct
	client   *http.Client
}

// NewSiliconFlowProvider 构造硅基流动 OCR provider。
func NewSiliconFlowProvider(endpoint, apiKey, model string) *SiliconFlowProvider {
	return &SiliconFlowProvider{
		endpoint: endpoint,
		apiKey:   apiKey,
		model:    model,
		client:   &http.Client{Timeout: 180 * time.Second},
	}
}

func (p *SiliconFlowProvider) Name() string { return "siliconflow" }

// promptForModel 按 model 名返回合适的识别 prompt。
// DeepSeek-OCR 专用 "Free OCR."；通用视觉模型用结构化识别指令。
func (p *SiliconFlowProvider) promptForModel() string {
	m := strings.ToLower(p.model)
	if strings.Contains(m, "deepseek-ocr") {
		return "Free OCR."
	}
	// Qwen-VL 等通用视觉模型：用明确指令，要求逐行列出
	return "识别这张图片里的英语单词、音标和中文释义，逐行列出。只输出识别到的文字内容，不要额外解释。"
}

func (p *SiliconFlowProvider) Recognize(ctx context.Context, image []byte, mimeType string) (*Result, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("OCR 未配置 apiKey，请在 data/config.yaml 的 ocr.apiKey 填入 API Key")
	}
	if p.endpoint == "" || p.model == "" {
		return nil, fmt.Errorf("OCR 配置不完整：endpoint=%q model=%q", p.endpoint, p.model)
	}

	raw, err := p.callOCR(ctx, image, mimeType)
	if err != nil {
		return nil, err
	}
	rows := ParseOCRText(raw)
	for i := range rows {
		if rows[i].WordType == "" {
			rows[i].WordType = "new"
		}
		if rows[i].Confidence == 0 {
			rows[i].Confidence = 1.0
		}
	}
	return &Result{Rows: rows, RawText: raw}, nil
}

// callOCR 调 OpenAI 兼容的 chat/completions，返回识别出的文本。
func (p *SiliconFlowProvider) callOCR(ctx context.Context, image []byte, mimeType string) (string, error) {
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(image))

	payload := map[string]any{
		"model": p.model,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": p.promptForModel()},
					{"type": "image_url", "image_url": map[string]string{"url": dataURL}},
				},
			},
		},
		"temperature": 0.0,
		"max_tokens":  4096,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	url := p.endpoint + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call OCR API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read OCR response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OCR API 返回 %d: %s", resp.StatusCode, truncate(string(respBody), 300))
	}

	var oai openAIResponse
	if err := json.Unmarshal(respBody, &oai); err != nil {
		return "", fmt.Errorf("decode OCR response: %w", err)
	}
	if len(oai.Choices) == 0 {
		return "", fmt.Errorf("OCR 响应无 choices")
	}
	return oai.Choices[0].Message.Content, nil
}

// openAIResponse 是 OpenAI chat/completions 响应的最小子集。
type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
