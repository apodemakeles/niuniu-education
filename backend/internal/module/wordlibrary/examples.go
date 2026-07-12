package wordlibrary

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/apodemakeles/niuniu-education/backend/internal/platform/llm"
)

const exampleCount = 3

// ExampleService 负责把模型输出收敛为可直接给小学生阅读的例句。
// 例句生成与词库写入解耦：单个词失败不会影响已保存的单词或同批其他词。
type ExampleService struct {
	store *Store
	llm   llm.Provider
}

func NewExampleService(store *Store, provider llm.Provider) *ExampleService {
	return &ExampleService{store: store, llm: provider}
}

func (s *ExampleService) GenerateForWord(ctx context.Context, wordID string) ([]WordExample, error) {
	w, err := s.store.Get(ctx, wordID)
	if err != nil {
		return nil, err
	}
	sentences, err := s.generate(ctx, w.Text, w.MeaningZh, nil, exampleCount)
	if err != nil {
		return nil, err
	}
	return s.store.ReplaceExamples(ctx, wordID, sentences)
}

func (s *ExampleService) RegenerateOne(ctx context.Context, wordID, exampleID string) (WordExample, error) {
	w, err := s.store.Get(ctx, wordID)
	if err != nil {
		return WordExample{}, err
	}
	existing, err := s.store.ListExamples(ctx, wordID)
	if err != nil {
		return WordExample{}, err
	}
	if !containsExampleID(existing, exampleID) {
		return WordExample{}, ErrNotFound
	}
	avoid := make([]string, 0, len(existing))
	for _, e := range existing {
		if e.ID != exampleID {
			avoid = append(avoid, e.Sentence)
		}
	}
	sentences, err := s.generate(ctx, w.Text, w.MeaningZh, avoid, 1)
	if err != nil {
		return WordExample{}, err
	}
	return s.store.UpdateExample(ctx, wordID, exampleID, sentences[0])
}

func containsExampleID(examples []WordExample, id string) bool {
	for _, e := range examples {
		if e.ID == id {
			return true
		}
	}
	return false
}

func (s *ExampleService) generate(ctx context.Context, text, meaning string, avoid []string, count int) ([]string, error) {
	if s.llm == nil {
		return nil, fmt.Errorf("例句生成模型未配置")
	}
	for attempt := 0; attempt < 2; attempt++ {
		user := examplePrompt(text, meaning, avoid, count)
		raw, err := s.llm.Generate(ctx, exampleSystemPrompt, user)
		if err != nil {
			continue
		}
		sentences, err := parseExampleJSON(raw, count)
		if err != nil || !validExamples(sentences, text, avoid) {
			continue
		}
		return sentences, nil
	}
	return nil, fmt.Errorf("例句生成失败，请稍后重试")
}

const exampleSystemPrompt = `你是面向中国小学生的英语教材编辑。输出必须是 JSON，不要解释、不要 Markdown。句子必须自然、积极、具体，使用常见小学英语词汇，避免复杂从句、生僻词、抽象话题和成人内容。`

func examplePrompt(text, meaning string, avoid []string, count int) string {
	base := fmt.Sprintf("目标英语词或短语：%q。中文意思：%q。请写 %d 个英文例句。每句不超过 12 个英文词，必须包含目标词或短语（大小写可变化），目标词出现多次也可以。", text, meaning, count)
	if len(avoid) > 0 {
		base += " 新句不能与这些已有例句重复：" + strings.Join(avoid, " | ")
	}
	if count == 1 {
		return base + ` 只返回 JSON：{"sentences":["..."]}`
	}
	return base + ` 只返回 JSON：{"sentences":["...","...","..."]}`
}

func parseExampleJSON(raw string, want int) ([]string, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(strings.TrimSpace(raw), "```")
	var payload struct {
		Sentences []string `json:"sentences"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	if len(payload.Sentences) != want {
		return nil, fmt.Errorf("例句数量不符")
	}
	for i := range payload.Sentences {
		payload.Sentences[i] = strings.TrimSpace(payload.Sentences[i])
	}
	return payload.Sentences, nil
}

func validExamples(sentences []string, target string, avoid []string) bool {
	seen := map[string]bool{}
	for _, sentence := range sentences {
		key := strings.ToLower(strings.TrimSpace(sentence))
		if key == "" || seen[key] || !containsTarget(sentence, target) || englishWordCount(sentence) > 12 {
			return false
		}
		for _, old := range avoid {
			if strings.EqualFold(strings.TrimSpace(old), sentence) {
				return false
			}
		}
		seen[key] = true
	}
	return true
}

func containsTarget(sentence, target string) bool {
	sentence = strings.ToLower(sentence)
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		return false
	}
	for from := 0; ; {
		index := strings.Index(sentence[from:], target)
		if index < 0 {
			return false
		}
		index += from
		end := index + len(target)
		if targetBoundary(sentence, index-1) && targetBoundary(sentence, end) {
			return true
		}
		from = end
	}
}

// targetBoundary 保证 "he" 不会被 "the" 误判为包含目标词；短语两端同样需要完整词边界。
func targetBoundary(s string, byteIndex int) bool {
	if byteIndex < 0 || byteIndex >= len(s) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(s[byteIndex:])
	return !unicode.IsLetter(r)
}

func englishWordCount(s string) int {
	count, inWord := 0, false
	for _, r := range s {
		if unicode.IsLetter(r) || r == '\'' {
			if !inWord {
				count++
				inWord = true
			}
		} else {
			inWord = false
		}
	}
	return count
}
