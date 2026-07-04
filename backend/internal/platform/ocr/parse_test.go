package ocr

import (
	"strings"
	"testing"
)

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
		p := &OpenAICompatProvider{model: c.model}
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
