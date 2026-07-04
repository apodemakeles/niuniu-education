package ocr

import (
	"regexp"
	"strings"
	"unicode"
)

// ParseOCRText 把 OCR 返回的原始文本解析为草稿行。
//
// DeepSeek-OCR 这类模型只忠实转录图片文字，不按指令重组结构，
// 因此需要后端用启发式把文本解析成 [{text, meaningZh, phonetic}]。
//
// 解析策略按优先级尝试：
//  1. markdown 表格（| a | b | c |）
//  2. 空格/制表符分隔行
//  3. 中英混排单行（ASCII 词与 CJK 释义分离）
//
// 解析不必完美：解析失败或字段缺失的行仍返回（text 为空），
// 交由前端草稿预览让家长修正——契合 PRD "OCR 必须经家长确认" 的设计。
func ParseOCRText(raw string) []DraftRow {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	// 清理 OCR 常见前缀噪音，如 ".# Title"、"# Title"
	lines := cleanLines(strings.Split(raw, "\n"))

	if rows, ok := tryParseTable(lines); ok {
		return rows
	}
	if rows, ok := tryParseDelimited(lines); ok {
		return rows
	}
	return tryParseMixed(lines)
}

func cleanLines(lines []string) []string {
	// 第一遍：保留缩进信息，识别"续行"（缩进的纯 CJK 行，多为上一词条的释义）。
	// OCR 常把表格里的中文释义拆成独立的缩进行，如：
	//   1. apple /ˈæpl/
	//      苹果          ← 此行应合并到上一行
	merged := make([]string, 0, len(lines))
	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		// 续行特征：原始行有前导空格/制表符，且去序号后是纯中文（无英文单词）
		if (strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\t")) && isContinuationLine(trimmed) {
			if len(merged) > 0 {
				merged[len(merged)-1] = merged[len(merged)-1] + " " + trimmed
				continue
			}
		}
		merged = append(merged, trimmed)
	}
	// 第二遍：去噪音
	out := make([]string, 0, len(merged))
	for _, l := range merged {
		l = strings.TrimLeft(l, ".#")
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// isContinuationLine 判断是否为释义续行：去序号后不含英文单词（仅 CJK 或 CJK+标点）。
func isContinuationLine(s string) bool {
	s = stripLeadingIndex(s)
	if s == "" {
		return false
	}
	// 含英文单词（连续2个以上字母）则不是纯释义续行
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return false
		}
	}
	return containsCJK(s)
}

// --- 策略 1：markdown 表格 ---
// 形如：
//   | 单词 | 中文 | 英文 |
//   |---|---|---|
//   | 1. apple | /ˈæpl/ | 苹果 |
func tryParseTable(lines []string) ([]DraftRow, bool) {
	var tableLines []string
	for _, l := range lines {
		if strings.HasPrefix(l, "|") {
			tableLines = append(tableLines, l)
		}
	}
	// 至少要有 2 行有效数据行（排除表头与分隔行）
	if len(tableLines) < 2 {
		return nil, false
	}

	var rows []DraftRow
	for _, l := range tableLines {
		// 跳过分隔行 |---|---|
		if isTableSeparator(l) {
			continue
		}
		cells := splitTableCells(l)
		// 跳过表头（含"单词"/"中文"/"英文"等标题词，或全列都是中文标题）
		if isHeaderRow(cells) {
			continue
		}
		row, ok := buildRowFromCells(cells)
		if ok {
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		return nil, false
	}
	return rows, true
}

func isTableSeparator(l string) bool {
	inner := strings.Trim(l, "|")
	inner = strings.ReplaceAll(inner, "-", "")
	inner = strings.ReplaceAll(inner, " ", "")
	inner = strings.ReplaceAll(inner, ":", "")
	return inner == ""
}

func splitTableCells(l string) []string {
	inner := strings.Trim(l, "|")
	parts := strings.Split(inner, "|")
	cells := make([]string, 0, len(parts))
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

func isHeaderRow(cells []string) bool {
	// 表头特征：出现"单词/中文/英文/音标/释义/word/meaning"等词
	for _, c := range cells {
		lc := strings.ToLower(c)
		if strings.ContainsAny(c, "单词中文英文音标释义序号") ||
			lc == "word" || lc == "meaning" || lc == "phonetic" || lc == "chinese" {
			return true
		}
	}
	return false
}

// --- 策略 2：空格/制表符分隔 ---
// 形如：apple,苹果,/ˈæpl/   或   apple\t苹果
// 注意：含完整音标 /.../ 或中英混排的行不在此处理（音标会被空格拆碎），
// 交给 tryParseMixed 按音标/CJK 边界分离。
func tryParseDelimited(lines []string) ([]DraftRow, bool) {
	var rows []DraftRow
	matched := 0
	for _, l := range lines {
		isCommaTab := strings.Contains(l, ",") || strings.Contains(l, "\t")
		// 空格分隔时：含完整音标 /.../ 或 CJK 的行跳过（音标/中文会被空格拆碎），交给 mixed
		// 逗号/制表符分隔时：音标和中文是完整 cell，正常处理
		if !isCommaTab && (hasCompletePhonetic(l) || containsCJK(l)) {
			continue
		}
		var parts []string
		if isCommaTab {
			parts = splitFields(l, []string{",", "\t"})
		} else {
			parts = strings.Fields(l)
		}
		row, ok := buildRowFromCells(parts)
		if ok && len(parts) >= 2 {
			rows = append(rows, row)
			matched++
		}
	}
	// 仅当大部分行（>50%）匹配时才认为分隔符策略成立，
	// 否则可能是误判（如 "Unit 4" 这种偶发匹配），应让位给 mixed 策略。
	if matched == 0 || len(lines) > 0 && matched*2 < len(lines) {
		return nil, false
	}
	return rows, true
}

func splitFields(s string, seps []string) []string {
	for _, sep := range seps {
		s = strings.ReplaceAll(s, sep, "\x00")
	}
	raw := strings.Split(s, "\x00")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// --- 策略 3：中英混排单行 ---
// 形如：1. apple /ˈæpl/ 苹果  →  分离 ASCII（英文+音标）与 CJK（释义）
func tryParseMixed(lines []string) []DraftRow {
	var rows []DraftRow
	for _, l := range lines {
		row, ok := parseMixedLine(l)
		if ok {
			rows = append(rows, row)
		}
	}
	return rows
}

var (
	indexPrefixRe = regexp.MustCompile(`^\d+[\.\)、]\s*`)
	pageSuffixRe  = regexp.MustCompile(`\s*p\.\s*\d+\s*$`) // 行尾页码 p. 30 / p.44
)

func parseMixedLine(l string) (DraftRow, bool) {
	l = stripLeadingIndex(l)            // 去掉 "1. " 序号
	l = strings.TrimSpace(l)
	l = strings.TrimLeft(l, "*")        // 去掉行首星号标记（教材重点词常带 *）
	l = strings.TrimSpace(l)
	l = pageSuffixRe.ReplaceAllString(l, "") // 去掉行尾页码 p. 30
	l = strings.TrimSpace(l)

	// 找第一段 CJK（中文释义）的起始位置
	cjkStart := indexOfFirstCJK(l)
	if cjkStart < 0 {
		// 整行无中文：可能是纯英文单词行，释义缺失
		enPart := strings.TrimSpace(l)
		enPart = strings.Trim(enPart, "|")
		if enPart == "" {
			return DraftRow{}, false
		}
		text, phonetic := splitTextAndPhonetic(enPart)
		return DraftRow{Text: text, Phonetic: phonetic, WordType: "new", Confidence: 1.0}, text != ""
	}
	enPart := strings.TrimSpace(l[:cjkStart])
	zhPart := strings.TrimSpace(l[cjkStart:])
	// 中文释义尾部可能也带页码（如"消防站 p. 30"在 CJK 之后混了 ASCII）
	zhPart = pageSuffixRe.ReplaceAllString(zhPart, "")
	zhPart = strings.TrimSpace(zhPart)
	enPart = strings.Trim(enPart, "|")
	text, phonetic := splitTextAndPhonetic(enPart)
	if text == "" {
		return DraftRow{}, false
	}
	return DraftRow{Text: text, MeaningZh: zhPart, Phonetic: phonetic, WordType: "new", Confidence: 1.0}, true
}

// buildRowFromCells 把若干单元格按内容启发式分配到 text/phonetic/meaningZh。
func buildRowFromCells(cells []string) (DraftRow, bool) {
	if len(cells) == 0 {
		return DraftRow{}, false
	}
	var text, phonetic, meaning string
	meaningSet := false
	phoneticSet := false
	textSet := false
	for _, c := range cells {
		c = stripLeadingIndex(c)
		if c == "" {
			continue
		}
		switch {
		case !phoneticSet && isPhonetic(c):
			phonetic = c
			phoneticSet = true
		case !meaningSet && containsCJK(c):
			meaning = c
			meaningSet = true
		case !textSet && isWordLike(c):
			text = c
			textSet = true
		}
	}
	if text == "" {
		return DraftRow{}, false
	}
	return DraftRow{Text: text, MeaningZh: meaning, Phonetic: phonetic, WordType: "new", Confidence: 1.0}, true
}

// isPhonetic 判断是否为音标：以 / 开头并以 / 结尾（如 /ˈæpl/）。
func isPhonetic(s string) bool {
	s = strings.TrimSpace(s)
	return len(s) >= 2 && strings.HasPrefix(s, "/") && strings.HasSuffix(s, "/")
}

// hasCompletePhonetic 判断整行是否含完整的音标片段（/.../）。
// 用于让 tryParseDelimited 跳过含音标的行，交给 mixed 策略处理。
func hasCompletePhonetic(l string) bool {
	i := strings.Index(l, "/")
	if i < 0 {
		return false
	}
	j := strings.Index(l[i+1:], "/")
	return j >= 0 // 存在第二个 /
}

// containsCJK 判断是否含 CJK 统一表意文字（汉字）。
func containsCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func indexOfFirstCJK(s string) int {
	for i, r := range s {
		if unicode.Is(unicode.Han, r) {
			return i
		}
	}
	return -1
}

// isWordLike 判断是否像英文单词：去掉标点后全为 ASCII 字母。
func isWordLike(s string) bool {
	s = stripLeadingIndex(s)
	s = strings.Trim(s, ".,;:!?\"'()|")
	if s == "" {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return true
}

// splitTextAndPhonetic 从一段 ASCII 文本里分离单词与音标。
// 如 "apple /ˈæpl/" → ("apple", "/ˈæpl/")
func splitTextAndPhonetic(s string) (text, phonetic string) {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "/"); i >= 0 {
		if j := strings.LastIndex(s, "/"); j > i {
			return strings.TrimSpace(s[:i]), s[i : j+1]
		}
	}
	return s, ""
}

// stripLeadingIndex 去掉行首序号前缀，如 "1. "、"2) "、"3、 "。
func stripLeadingIndex(s string) string {
	return strings.TrimSpace(indexPrefixRe.ReplaceAllString(strings.TrimSpace(s), ""))
}
