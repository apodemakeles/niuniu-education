package ocr

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DeepSeekProvider 通过 OpenAI 兼容接口调用 DeepSeek-OCR 模型。
//
// 经实测（~/work/deepseek-ocr-demo）：
//   - 走硅基流动 endpoint：https://api.siliconflow.cn/v1
//   - 模型名：deepseek-ai/DeepSeek-OCR
//   - prompt 用 "Free OCR." 效果最好（忠实转录）
//   - OCR 模型不听从 JSON 输出指令，因此采用两步法：先转录文本，再由 ParseOCRText 解析
type DeepSeekProvider struct {
	endpoint string // 形如 https://api.siliconflow.cn/v1
	apiKey   string
	model    string // 形如 deepseek-ai/DeepSeek-OCR
	client   *http.Client
}

func NewDeepSeekProvider(endpoint, apiKey, model string) *DeepSeekProvider {
	return &DeepSeekProvider{
		endpoint: endpoint,
		apiKey:   apiKey,
		model:    model,
		client:   &http.Client{Timeout: 180 * time.Second}, // OCR 较慢
	}
}

func (p *DeepSeekProvider) Name() string { return "deepseek" }

func (p *DeepSeekProvider) Recognize(ctx context.Context, image []byte, mimeType string) (*Result, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("OCR 未配置 apiKey，请在 data/config.yaml 的 ocr.apiKey 填入硅基流动 API Key")
	}
	if p.endpoint == "" || p.model == "" {
		return nil, fmt.Errorf("OCR 配置不完整：endpoint=%q model=%q", p.endpoint, p.model)
	}

	raw, err := p.callOCR(ctx, image, mimeType)
	if err != nil {
		return nil, err
	}
	rows := ParseOCRText(raw)
	// 确保每行有默认 wordType 与 confidence
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
func (p *DeepSeekProvider) callOCR(ctx context.Context, image []byte, mimeType string) (string, error) {
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(image))

	payload := map[string]any{
		"model": p.model,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": "Free OCR."},
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

	// 解析 OpenAI 格式响应
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
