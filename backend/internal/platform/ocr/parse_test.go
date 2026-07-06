package ocr

import (
	"strings"
	"testing"
)

func TestParseFilterTitle(t *testing.T) {
	// Unit 4 / Lesson 5 这类标题行不应被识别为单词
	raw := "fire station /ˈfaɪər/ 消防站\nUnit 4\nfactory /ˈfæktri/ 工厂\nLesson 5"
	rows := ParseOCRText(raw)
	for _, r := range rows {
		if strings.Contains(strings.ToLower(r.Text), "unit") || strings.Contains(strings.ToLower(r.Text), "lesson") {
			t.Errorf("标题行被误识别为单词: %+v", r)
		}
	}
	// 应只有 2 个有效单词
	valid := 0
	for _, r := range rows {
		if r.Text != "" && (r.MeaningZh != "" || r.Phonetic != "") {
			valid++
		}
	}
	if valid < 2 {
		t.Errorf("有效单词数 %d, want >= 2", valid)
	}
}

func TestIsLikelyTitle(t *testing.T) {
	cases := map[string]bool{
		"Unit 4":   true,
		"Unit4":    true,
		"Lesson 5": true,
		"Chapter 3": true,
		"apple":    false,
		"factory":  false,
		"Unit 4 词汇": false, // 含中文，不是纯标题
	}
	for in, want := range cases {
		if got := isLikelyTitle(in); got != want {
			t.Errorf("isLikelyTitle(%q)=%v, want %v", in, got, want)
		}
	}
}

func TestParseQwenVLOutputWithPlaceholderSlash(t *testing.T) {
	// 来自真实 Qwen3-VL 对教材截图的输出（import_images.ocr_raw_text）
	raw := `fire station / 消防站
factory / 'fæktri/ / 工厂
* farm /'fa:m/ / 农场；牧场；饲养场
middle school / 中学
take care of / 保管；照顾
living room / 客厅；起居室
do the dishes / 洗餐具
What a mess! / 真是一团糟啊！
Unit 4`
	rows := ParseOCRText(raw)
	byText := map[string]DraftRow{}
	for _, r := range rows {
		if r.Text != "" {
			byText[r.Text] = r
		}
	}

	noPhonetic := []struct {
		text    string
		meaning string
	}{
		{"fire station", "消防站"},
		{"middle school", "中学"},
		{"take care of", "保管；照顾"},
		{"living room", "客厅；起居室"},
		{"do the dishes", "洗餐具"},
		{"What a mess!", "真是一团糟啊！"},
	}
	for _, c := range noPhonetic {
		r, ok := byText[c.text]
		if !ok {
			t.Errorf("未识别 %q", c.text)
			continue
		}
		if strings.HasSuffix(r.Text, "/") {
			t.Errorf("%q 英文不应以斜杠结尾: %q", c.text, r.Text)
		}
		if r.Phonetic != "" {
			t.Errorf("%q 不应有音标, got %q", c.text, r.Phonetic)
		}
		if r.MeaningZh != c.meaning {
			t.Errorf("%q 释义: got %q, want %q", c.text, r.MeaningZh, c.meaning)
		}
	}

	r, ok := byText["factory"]
	if !ok {
		t.Fatal("未识别 factory")
	}
	if strings.HasSuffix(r.Phonetic, " /") || strings.HasSuffix(r.Phonetic, "/ /") {
		t.Errorf("factory 音标尾部有多余斜杠: %q", r.Phonetic)
	}
	if !strings.HasPrefix(r.Phonetic, "/") || !strings.HasSuffix(r.Phonetic, "/") {
		t.Errorf("factory 音标格式异常: %q", r.Phonetic)
	}
	if r.MeaningZh != "工厂" {
		t.Errorf("factory 释义: got %q", r.MeaningZh)
	}
}

func TestSplitTextAndPhonetic(t *testing.T) {
	cases := []struct {
		in          string
		wantText    string
		wantPhonetic string
	}{
		{"middle school /", "middle school", ""},
		{"factory / 'fæktri/ /", "factory", "/'fæktri/"},
		{"factory /ˈfæktri/", "factory", "/ˈfæktri/"},
		{"apple /ˈæpl/", "apple", "/ˈæpl/"},
		{"fire station /ˈfaɪər ˌsteɪʃən/", "fire station", "/ˈfaɪər ˌsteɪʃən/"},
	}
	for _, c := range cases {
		text, phonetic := splitTextAndPhonetic(c.in)
		if text != c.wantText || phonetic != c.wantPhonetic {
			t.Errorf("splitTextAndPhonetic(%q) = (%q, %q), want (%q, %q)",
				c.in, text, phonetic, c.wantText, c.wantPhonetic)
		}
	}
}

func TestParseParentheticalMeaning(t *testing.T) {
	// 来自真实 OCR 原文（import_images），OCR 本身括号位置正确
	raw := `Ms /mɪz/ （用于女子的姓氏或姓名前，不指明婚否）女士
Nice to see you! （以前见过面的人之间用）见到你很高兴！
him /hɪm/ （he 的宾格）他
hospital /'hɒspɪt(ə)l/ 医院`
	rows := ParseOCRText(raw)
	byText := map[string]DraftRow{}
	for _, r := range rows {
		byText[r.Text] = r
	}

	ms := byText["Ms"]
	if ms.MeaningZh != "（用于女子的姓氏或姓名前，不指明婚否）女士" {
		t.Errorf("Ms 释义: got %q", ms.MeaningZh)
	}
	if strings.Contains(ms.Text, "（") {
		t.Errorf("Ms 英文不应含括号: %q", ms.Text)
	}

	nice := byText["Nice to see you!"]
	if nice.MeaningZh != "（以前见过面的人之间用）见到你很高兴！" {
		t.Errorf("Nice to see you! 释义: got %q", nice.MeaningZh)
	}
	if strings.Contains(nice.Text, "（") {
		t.Errorf("Nice to see you! 英文不应含括号: %q", nice.Text)
	}

	him := byText["him"]
	if him.MeaningZh != "（he 的宾格）他" {
		t.Errorf("him 释义: got %q", him.MeaningZh)
	}

	hosp := byText["hospital"]
	if hosp.MeaningZh != "医院" {
		t.Errorf("hospital 释义不应误含音标内括号: got %q", hosp.MeaningZh)
	}
}

func TestIndexOfMeaningStart(t *testing.T) {
	cases := []struct {
		in   string
		want int // -1 表示期望从该 rune 起为释义；用子串校验更直观
		at   string
	}{
		{"Ms /mɪz/ （用于", len("Ms /mɪz/ "), "（"},
		{"Nice to see you! （以前", len("Nice to see you! "), "（"},
		{"factory /ˈfæktri/ 工厂", strings.Index("factory /ˈfæktri/ 工厂", "工"), "工"},
	}
	for _, c := range cases {
		got := indexOfMeaningStart(c.in)
		if got < 0 || c.in[got:] != c.in[c.want:] && !strings.HasPrefix(c.in[got:], c.at) {
			t.Errorf("indexOfMeaningStart(%q)=%d, want prefix %q, got %q", c.in, got, c.at, c.in[got:])
		}
	}
}

func TestParseQwen3VLOutput(t *testing.T) {
	// Qwen3-VL-32B-Instruct 对教材截图的真实输出（来自实测）。
	// 格式：单词(可多词) 音标 中文 p.页码，部分行带 * 标记、Unit 标题。
	raw := `fire station /ˈfaɪər ˌsteɪʃən/ 消防站 p. 30
factory /ˈfæktri/ 工厂 p. 30
farm /fɑːm/ 农场；牧场；饲养场 p. 30
hospital /ˈhɒspɪt(ə)l/ 医院 p. 30
Ms /mɪz/ （用于女子的姓氏或姓名前，不指明婚否）女士 p. 28
* its /ɪts/ 它的 p. 44
* their /ðeə(r)/ 他们的 p. 44
mine /maɪn/ 我的 p. 44
Unit 4`
	rows := ParseOCRText(raw)

	// 应识别出有效单词行（Unit 4 这种纯标题行无中文释义，应被过滤或标记）
	texts := map[string]DraftRow{}
	for _, r := range rows {
		if r.Text != "" {
			texts[r.Text] = r
		}
	}

	// 核心断言：关键单词的音标和中文正确
	if r, ok := texts["factory"]; !ok {
		t.Errorf("未识别出 factory")
	} else {
		if r.MeaningZh != "工厂" {
			t.Errorf("factory 释义: got %q, want 工厂", r.MeaningZh)
		}
		if r.Phonetic != "/ˈfæktri/" {
			t.Errorf("factory 音标: got %q, want /ˈfæktri/", r.Phonetic)
		}
	}

	// 多词英文：fire station
	if r, ok := texts["fire station"]; !ok {
		t.Errorf("未识别出 fire station（多词英文）")
	} else if r.MeaningZh != "消防站" {
		t.Errorf("fire station 释义: got %q, want 消防站", r.MeaningZh)
	}

	// 带 * 标记的行应被剥离星号
	if _, ok := texts["its"]; !ok {
		t.Errorf("未识别出 its（应剥离行首 * 标记）")
	}

	// 页码 p. 30 不应出现在释义里
	if r, ok := texts["farm"]; ok {
		if strings.Contains(r.MeaningZh, "p.") || strings.Contains(r.MeaningZh, "30") {
			t.Errorf("farm 释义不应含页码: got %q", r.MeaningZh)
		}
	}
}

func TestPromptForModel(t *testing.T) {
	cases := []struct {
		model    string
		wantFree bool // true=期望 "Free OCR."（DeepSeek-OCR）
	}{
		{"deepseek-ai/DeepSeek-OCR", true},
		{"deepseek-ai/DeepSeek-OCR-2", true},
		{"Qwen/Qwen3-VL-32B-Instruct", false},
		{"PaddlePaddle/PaddleOCR-VL-1.5", false},
	}
	for _, c := range cases {
		p := &SiliconFlowProvider{model: c.model}
		got := p.promptForModel()
		isFree := got == "Free OCR."
		if isFree != c.wantFree {
			t.Errorf("model=%q: prompt=%q, wantFree=%v", c.model, got, c.wantFree)
		}
	}
}

func TestParseRealDeepSeekOutput(t *testing.T) {
	// 真实 DeepSeek-OCR 对 /tmp/wordlist_test.png 的输出：缩进续行结构。
	// 释义"苹果"等被拆到独立的缩进行，需先合并续行再解析。
	raw := ".# Unit 3 英语单词表\n\nName: ______ Date: ______\n\n" +
		"1. apple /ˈæpl/  \n   苹果  \n\n" +
		"2. banana /bəˈnɑːnə/  \n   香蕉  \n\n" +
		"3. climb /klɑːm/  \n   攀爬  \n\n" +
		"4. window /ˈwɪndoʊ/  \n   窗户  \n\n" +
		"5. their /ðeɪr/  \n   他们的  \n\n" +
		"6. read /riːd/  \n   阅读"
	rows := ParseOCRText(raw)
	// 应识别出 6 个单词，且释义不丢失
	if len(rows) < 6 {
		t.Fatalf("want >=6 rows, got %d: %+v", len(rows), rows)
	}
	// 至少 apple/climb/read 的释义应正确
	found := map[string]string{}
	for _, r := range rows {
		if r.Text != "" && r.MeaningZh != "" {
			found[r.Text] = r.MeaningZh
		}
	}
	for _, w := range []string{"apple", "climb", "read", "banana"} {
		if _, ok := found[w]; !ok {
			t.Errorf("单词 %s 的释义缺失", w)
		}
	}
	if found["apple"] != "苹果" {
		t.Errorf("apple 释义: got %q, want 苹果", found["apple"])
	}
}

func TestParseMarkdownTable(t *testing.T) {
	// 来自 /tmp/wordlist_test.png 的真实 DeepSeek-OCR 输出（表格格式）
	raw := `.# Unit 3 英语单词表

Name: ______ Date: ______

| 单词    | 中文    | 英文    |
|---|---|---|
| 1. apple   | /ˈæpl/   | 苹果    |
| 2. banana  | /bəˈnɑːnə/| 香蕉    |
| 3. climb   | /klɑːm/   | 攀爬    |
| 4. window  | /ˈwɪndoʊ/ | 窗户    |
| 5. their   | /ðeər/    | 他们的   |
| 6. read    | /riːd/    | 阅读    |`
	rows := ParseOCRText(raw)
	if len(rows) != 6 {
		t.Fatalf("want 6 rows, got %d: %+v", len(rows), rows)
	}
	want := DraftRow{Text: "apple", MeaningZh: "苹果", Phonetic: "/ˈæpl/", WordType: "new", Confidence: 1.0}
	if rows[0] != want {
		t.Errorf("row0: got %+v, want %+v", rows[0], want)
	}
	if rows[5].Text != "read" || rows[5].MeaningZh != "阅读" {
		t.Errorf("row5: got %+v", rows[5])
	}
}

func TestParseDelimitedSpace(t *testing.T) {
	raw := "apple /ˈæpl/ 苹果\nbanana /bəˈnɑːnə/ 香蕉\nread /riːd/ 阅读"
	rows := ParseOCRText(raw)
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	if rows[0].Text != "apple" || rows[0].MeaningZh != "苹果" || rows[0].Phonetic != "/ˈæpl/" {
		t.Errorf("row0: %+v", rows[0])
	}
}

func TestParseDelimitedComma(t *testing.T) {
	raw := "apple,苹果,/ˈæpl/\nbanana,香蕉,/bəˈnɑːnə/"
	rows := ParseOCRText(raw)
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	if rows[0].Text != "apple" || rows[0].MeaningZh != "苹果" {
		t.Errorf("row0: %+v", rows[0])
	}
}

func TestParseMixedLine(t *testing.T) {
	// 无音标的中英混排
	raw := "1. apple 苹果\n2. banana 香蕉"
	rows := ParseOCRText(raw)
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	if rows[0].Text != "apple" || rows[0].MeaningZh != "苹果" || rows[0].Phonetic != "" {
		t.Errorf("row0: %+v", rows[0])
	}
}

func TestParseIndexPrefix(t *testing.T) {
	// 序号应被剥离
	raw := "1. climb 攀爬\n2. window 窗户"
	rows := ParseOCRText(raw)
	for _, r := range rows {
		if r.Text == "" {
			t.Errorf("text empty: %+v", r)
		}
	}
}

func TestParseEmpty(t *testing.T) {
	if rows := ParseOCRText(""); rows != nil {
		t.Errorf("want nil for empty, got %v", rows)
	}
	if rows := ParseOCRText("   \n  \n"); rows != nil {
		t.Errorf("want nil for blank, got %v", rows)
	}
}

func TestStripLeadingIndex(t *testing.T) {
	cases := map[string]string{
		"1. apple":   "apple",
		"2) banana":  "banana",
		"3、cherry":  "cherry",
		"apple":      "apple",
		"10. water":  "water",
	}
	for in, want := range cases {
		if got := stripLeadingIndex(in); got != want {
			t.Errorf("stripLeadingIndex(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestIsPhonetic(t *testing.T) {
	if !isPhonetic("/ˈæpl/") {
		t.Error("want true for /ˈæpl/")
	}
	if isPhonetic("apple") {
		t.Error("want false for apple")
	}
	if isPhonetic("/incomplete") {
		t.Error("want false for /incomplete")
	}
}
