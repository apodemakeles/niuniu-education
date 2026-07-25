package studentwordtask

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/apodemakeles/niuniu-education/backend/internal/platform/llm"
)

// reading.go 实现延伸阅读 AI 短文生成与校验（对齐 PRD「延伸阅读生成机制」）。
//
// 流程：
//  1. SelectCoveredWords 从今日任务词中按优先级选 3~6 个覆盖词（新词池→需强化→学习中→已掌握）。
//  2. GeneratePassage 调 LLM 生成「标题/正文/场景提示」，最多重试 2 次。
//  3. ValidatePassage 校验长度（80~140 英文词）与每个覆盖词原词精确出现 2 次。
//  4. HighlightText 把正文中的覆盖词用 <mark> 包裹，并统计出现次数。

const (
	readingMinWords = 80
	readingMaxWords = 140
	readingAppear   = 2  // 每个覆盖词在正文中至少出现的次数
	readingMaxRetry = 4  // 首次 + 最多 4 次重试（deepseek-v4-flash 每次约 3s，5 次约 15s）
)

// SelectCoveredWords 按 PRD 优先级选择覆盖词：新词池→需强化→学习中→已掌握，3~6 个。
// 任务词不足 3 个时按实际数量。
// 含空格或非字母字符的多词短语（如 "Nice to see you!"、"put away"）不参与选择，
// 因为它们无法在正文里被原形精确匹配 2 次，会导致校验必然失败。
func SelectCoveredWords(firstPool, reviewPool []CandidateWord, todayLearnings map[string]WordLearning) []CandidateWord {
	const minCover = 3
	const maxCover = 6

	// 把 reviewPool 按状态分组（需强化/学习中/已掌握），稳定排序
	var reinforce, learning, mastered []CandidateWord
	for _, c := range reviewPool {
		if !isSingleWord(c.Word.Text) {
			continue
		}
		st := c.Word.Status
		if l, ok := todayLearnings[c.Word.ID]; ok {
			st = l.LearningStatus
		}
		switch st {
		case StatusReinforce:
			reinforce = append(reinforce, c)
		case StatusLearning:
			learning = append(learning, c)
		case StatusMastered:
			mastered = append(mastered, c)
		}
	}
	sortStable := func(s []CandidateWord) {
		sort.SliceStable(s, func(i, j int) bool { return s[i].Word.ID < s[j].Word.ID })
	}
	sortStable(reinforce)
	sortStable(learning)
	sortStable(mastered)

	ordered := make([]CandidateWord, 0, len(firstPool)+len(reviewPool))
	for _, c := range firstPool { // 1. 新词池优先
		if isSingleWord(c.Word.Text) {
			ordered = append(ordered, c)
		}
	}
	ordered = append(ordered, reinforce...) // 2. 需强化
	ordered = append(ordered, learning...)  // 3. 学习中
	ordered = append(ordered, mastered...)  // 4. 已掌握

	// 去重（同一词组不重复选择；firstPool 与 reviewPool 本就互斥，这里兜底）
	seen := map[string]bool{}
	dedup := ordered[:0]
	for _, c := range ordered {
		if seen[c.Word.ID] {
			continue
		}
		seen[c.Word.ID] = true
		dedup = append(dedup, c)
	}

	// 上限 6 个
	if len(dedup) > maxCover {
		dedup = dedup[:maxCover]
	}
	// 任务词不足 3 个：按实际数量（不强制补足）
	if len(dedup) < minCover {
		return dedup
	}
	return dedup
}

// isSingleWord 判断词库条目是否为单个英文单词（仅含字母，可含连字符和撇号如 don't、well-known）。
// 多词短语（含空格）或带标点的句子（如 "Nice to see you!"）返回 false。
func isSingleWord(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, r := range s {
		if unicode.IsLetter(r) || r == '\'' || r == '-' {
			continue
		}
		return false
	}
	return true
}

// GeneratePassage 调 LLM 生成短文。返回标题、正文、场景提示。
// 失败时返回错误；调用方负责重试与兜底。
func GeneratePassage(ctx context.Context, provider llm.Provider, covered []CandidateWord, grade string) (title, text, sceneHint string, err error) {
	if provider == nil {
		return "", "", "", fmt.Errorf("未配置文本模型 provider")
	}
	if len(covered) == 0 {
		return "", "", "", fmt.Errorf("没有覆盖词")
	}

	system := "你是给中国小学生写英语阅读短文的助手。" +
		"必须严格输出合法 JSON（不要 markdown 代码块、不要解释文字、不要多余逗号），" +
		"格式为 {\"title\":\"英文标题\",\"text\":\"英文正文\",\"sceneHint\":\"中文场景提示\"}。" +
		"所有字符串值必须是合法 JSON 字符串（双引号转义）。"
	user := buildGenerationPrompt(covered, grade)

	raw, err := provider.Generate(ctx, system, user)
	if err != nil {
		return "", "", "", fmt.Errorf("调 LLM: %w", err)
	}

	title, text, sceneHint, err = parsePassageJSON(raw)
	if err != nil {
		return "", "", "", fmt.Errorf("解析短文: %w", err)
	}
	return title, text, sceneHint, nil
}

func buildGenerationPrompt(covered []CandidateWord, grade string) string {
	var lines []string
	lines = append(lines, "请用下面这些英语单词，为中国小学生写一篇英语小故事。")
	lines = append(lines, "")
	lines = append(lines, "要求：")
	lines = append(lines, "1. 正文 text 长度必须在 80 到 140 个英文单词之间（重要：不要少于 80 词）。")
	lines = append(lines, "2. 每个目标单词以【字典原形】在正文中【最多出现 2 次】，不能超过 2 次。不要求每个目标词都必须出现，但尽量多用上。")
	lines = append(lines, "3. 语言简单、有趣，有完整的小故事情节。")
	lines = append(lines, "4. sceneHint 用一句话解释场景，不要逐句翻译，不要直接列出英文答案。")
	if grade != "" {
		lines = append(lines, "孩子年级/难度："+grade)
	}
	lines = append(lines, "")
	lines = append(lines, "目标单词：")
	for _, c := range covered {
		ph := c.Word.Phonetic
		line := fmt.Sprintf("- %s (%s) %s", c.Word.Text, c.Word.MeaningZh, ph)
		lines = append(lines, strings.TrimSpace(line))
	}
	lines = append(lines, "")
	lines = append(lines, "只输出 JSON：")
	lines = append(lines, `{"title":"英文标题","text":"英文正文（80到140词）","sceneHint":"中文场景提示"}`)
	return strings.Join(lines, "\n")
}

// parsePassageJSON 解析 LLM 返回（容错：剥离 markdown 代码块围栏、修复常见 JSON 错误）。
func parsePassageJSON(raw string) (title, text, sceneHint string, err error) {
	s := stripCodeFence(raw)
	var obj struct {
		Title     string `json:"title"`
		Text      string `json:"text"`
		SceneHint string `json:"sceneHint"`
	}
	if e := json.Unmarshal([]byte(s), &obj); e != nil {
		// 尝试修复常见 JSON 错误：连续逗号、尾逗号
		fixed := fixCommonJSONErrors(s)
		if e2 := json.Unmarshal([]byte(fixed), &obj); e2 != nil {
			return "", "", "", fmt.Errorf("JSON 解析失败: %w; 原始: %s", e, truncateForLog(raw))
		}
	}
	obj.Title = strings.TrimSpace(obj.Title)
	obj.Text = strings.TrimSpace(obj.Text)
	obj.SceneHint = strings.TrimSpace(obj.SceneHint)
	if obj.Text == "" {
		return "", "", "", fmt.Errorf("正文为空; 原始: %s", truncateForLog(raw))
	}
	if obj.Title == "" {
		obj.Title = "Today's Reading"
	}
	return obj.Title, obj.Text, obj.SceneHint, nil
}

// fixCommonJSONErrors 修复小模型常见的 JSON 格式错误：
//   - 连续逗号 ",," → ","
//   - 尾逗号 ",}" → "}"、",]" → "]"
var (
	reMultiComma       = regexp.MustCompile(`,{2,}`)
	reTrailingCommaObj = regexp.MustCompile(`,\s*}`)
	reTrailingCommaArr = regexp.MustCompile(`,\s*]`)
)

func fixCommonJSONErrors(s string) string {
	s = reMultiComma.ReplaceAllString(s, ",")
	s = reTrailingCommaObj.ReplaceAllString(s, "}")
	s = reTrailingCommaArr.ReplaceAllString(s, "]")
	return s
}

func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// 去掉首行围栏（可能带语言标识）
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		}
		s = strings.TrimSpace(s)
	}
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	// 某些模型会把 JSON 前后加上解释，尝试截取第一个 { 到最后一个 }
	if i := strings.Index(s, "{"); i >= 0 {
		if j := strings.LastIndex(s, "}"); j > i {
			s = s[i : j+1]
		}
	}
	return s
}

func truncateForLog(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}

// CountWordOccurrences 按词库原词精确匹配统计出现次数（PRD：只统计原词，变形不计）。
// 匹配规则：作为单词边界匹配，大小写不敏感。
func CountWordOccurrences(text, word string) int {
	if word == "" {
		return 0
	}
	// 用正则按单词边界匹配，转义原词
	pattern := `(?i)\b` + regexp.QuoteMeta(word) + `\b`
	re, err := regexp.Compile(pattern)
	if err != nil {
		return 0
	}
	return len(re.FindAllStringIndex(text, -1))
}

// CountEnglishWords 统计英文单词数（用于校验长度 80~140）。
func CountEnglishWords(text string) int {
	n := 0
	inWord := false
	for _, r := range text {
		if unicode.IsLetter(r) || r == '\'' || r == '-' {
			if !inWord {
				n++
				inWord = true
			}
		} else {
			inWord = false
		}
	}
	return n
}

// ValidatePassage 校验短文：长度 80~140 英文词，每个覆盖词原词最多出现 2 次。
// 返回不合格原因（空串表示合格）。
func ValidatePassage(text string, covered []CandidateWord) string {
	wc := CountEnglishWords(text)
	if wc < readingMinWords || wc > readingMaxWords {
		return fmt.Sprintf("短文长度 %d 不在 %d~%d 之间", wc, readingMinWords, readingMaxWords)
	}
	for _, c := range covered {
		count := CountWordOccurrences(text, c.Word.Text)
		if count > readingAppear {
			return fmt.Sprintf("目标词 %s 出现 %d 次，不能超过 %d 次", c.Word.Text, count, readingAppear)
		}
	}
	return ""
}

// HighlightText 把覆盖词在正文中用 <mark> 包裹，返回 HTML 与每个词出现次数。
// 大小写不敏感匹配；保留原文大小写。
//
// 安全（防存储型 XSS）：对整段正文逐段做 HTML 转义，仅在命中覆盖词的位置插入
// <mark> 标签。早期实现用 ReplaceAllStringFunc 只转义了「命中的片段」，
// 命中词之间的正文原样透传，导致 LLM 生成内容中的 < > & 等字符会作为 HTML 执行。
// 现改为按命中位置分段拼接，确保所有未命中片段也被转义。
func HighlightText(text string, covered []CandidateWord) (htmlText string, counts map[string]int) {
	counts = map[string]int{}
	if len(covered) == 0 {
		return htmlEscapeForMark(text), counts
	}
	// 按词长度降序避免短词先替换破坏长词（例如 their vs the）
	sorted := make([]CandidateWord, len(covered))
	copy(sorted, covered)
	sort.SliceStable(sorted, func(i, j int) bool {
		return len(sorted[i].Word.Text) > len(sorted[j].Word.Text)
	})

	// 合并所有目标词为一个正则
	parts := make([]string, 0, len(sorted))
	for _, c := range sorted {
		parts = append(parts, regexp.QuoteMeta(c.Word.Text))
	}
	pattern := `(?i)\b(` + strings.Join(parts, "|") + `)\b`
	re, err := regexp.Compile(pattern)
	if err != nil {
		return htmlEscapeForMark(text), counts
	}

	matches := re.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return htmlEscapeForMark(text), counts
	}

	// 逐段拼接：未命中片段做 HTML 转义，命中覆盖词插入 <mark> 标签（内容同样转义）。
	var b strings.Builder
	last := 0
	for _, m := range matches {
		start, end := m[0], m[1]
		b.WriteString(htmlEscapeForMark(text[last:start])) // 转义命中前的未匹配片段
		match := text[start:end]
		marked := false
		for _, c := range sorted {
			if equalFoldASCII(match, c.Word.Text) {
				counts[c.Word.ID]++
				b.WriteString("<mark>")
				b.WriteString(htmlEscapeForMark(match))
				b.WriteString("</mark>")
				marked = true
				break
			}
		}
		if !marked {
			// 兜底：正则命中但未匹配到覆盖词（理论上不会发生），仅转义
			b.WriteString(htmlEscapeForMark(match))
		}
		last = end
	}
	b.WriteString(htmlEscapeForMark(text[last:])) // 转义末尾剩余片段
	return b.String(), counts
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func htmlEscapeForMark(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
