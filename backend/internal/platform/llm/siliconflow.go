package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SiliconFlowProvider 通过 OpenAI 兼容接口调用硅基流动平台上的文本模型。
// 与 ocr.SiliconFlowProvider 共用 endpoint/apiKey，仅 model 不同（文本而非视觉）。
type SiliconFlowProvider struct {
	endpoint string // 形如 https://api.siliconflow.cn/v1
	apiKey   string
	model    string // 形如 Qwen/Qwen2.5-7B-Instruct
	client   *http.Client
}

// NewSiliconFlowProvider 构造文本生成 provider。
func NewSiliconFlowProvider(endpoint, apiKey, model string) *SiliconFlowProvider {
	return &SiliconFlowProvider{
		endpoint: endpoint,
		apiKey:   apiKey,
		model:    model,
		// 文本生成（尤其推理类大模型）可能较慢，给充裕超时。
		client: &http.Client{Timeout: 180 * time.Second},
	}
}

func (p *SiliconFlowProvider) Name() string { return "siliconflow" }

func (p *SiliconFlowProvider) Generate(ctx context.Context, system, user string) (string, error) {
	if p.apiKey == "" {
		return "", fmt.Errorf("文本模型未配置 apiKey，请在 data/config.yaml 的 llm.apiKey（或 ocr.apiKey）填入")
	}
	if p.endpoint == "" || p.model == "" {
		return "", fmt.Errorf("文本模型配置不完整：endpoint=%q model=%q", p.endpoint, p.model)
	}

	messages := []map[string]string{
		{"role": "system", "content": system},
		{"role": "user", "content": user},
	}
	payload := map[string]any{
		"model":       p.model,
		"messages":    messages,
		"temperature": 0.5, // 偏低以稳定 JSON 格式与目标词原形复现
		"max_tokens":  8192, // 推理模型（如 deepseek-v4-flash）会先消耗 reasoning token，需留足余量
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
		return "", fmt.Errorf("call LLM API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read LLM response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM API 返回 %d: %s", resp.StatusCode, truncate(string(respBody), 300))
	}

	var oai openAIResponse
	if err := json.Unmarshal(respBody, &oai); err != nil {
		return "", fmt.Errorf("decode LLM response: %w", err)
	}
	if len(oai.Choices) == 0 {
		return "", fmt.Errorf("LLM 响应无 choices")
	}
	return oai.Choices[0].Message.Content, nil
}

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
