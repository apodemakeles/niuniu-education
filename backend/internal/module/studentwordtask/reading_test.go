package studentwordtask

import "testing"

// reading_test.go 验证覆盖词选择、短文校验、高亮统计。

func TestSelectCoveredWords_PriorityNewFirst(t *testing.T) {
	// 优先级：新词池 → 需强化 → 学习中 → 已掌握
	first := []CandidateWord{
		{Word: mkWord("n1", "apple", StatusUnlearned)},
		{Word: mkWord("n2", "desk", StatusUnlearned)},
	}
	review := []CandidateWord{
		{Word: mkWord("reinf", "read", StatusReinforce)},
		{Word: mkWord("learn", "write", StatusLearning)},
		{Word: mkWord("master", "book", StatusMastered)},
	}
	learnings := map[string]WordLearning{
		"reinf":  {LearningStatus: StatusReinforce},
		"learn":  {LearningStatus: StatusLearning},
		"master": {LearningStatus: StatusMastered},
	}
	got := SelectCoveredWords(first, review, learnings)
	if len(got) < 3 {
		t.Fatalf("覆盖词数量=%d，期望 ≥3", len(got))
	}
	// 前 2 个应是新词池
	if got[0].Word.ID != "n1" || got[1].Word.ID != "n2" {
		t.Fatalf("新词池应在最前，得 %s %s", got[0].Word.ID, got[1].Word.ID)
	}
	// 第 3 个应是需强化（优先于学习中和已掌握）
	if got[2].Word.ID != "reinf" {
		t.Fatalf("第3个应为需强化，得 %s", got[2].Word.ID)
	}
}

func TestSelectCoveredWords_MaxSix(t *testing.T) {
	first := []CandidateWord{}
	for i := 0; i < 4; i++ {
		first = append(first, CandidateWord{Word: mkWord("n"+string(rune('a'+i)), "n", StatusUnlearned)})
	}
	review := []CandidateWord{}
	for i := 0; i < 5; i++ {
		id := "r" + string(rune('a'+i))
		review = append(review, CandidateWord{Word: mkWord(id, id, StatusReinforce)})
	}
	got := SelectCoveredWords(first, review, map[string]WordLearning{})
	if len(got) > 6 {
		t.Fatalf("覆盖词上限 6，得 %d", len(got))
	}
}

func TestSelectCoveredWords_TooFewByActual(t *testing.T) {
	// 任务词不足 3 个：按实际数量
	first := []CandidateWord{{Word: mkWord("a", "a", StatusUnlearned)}}
	got := SelectCoveredWords(first, nil, nil)
	if len(got) != 1 {
		t.Fatalf("不足3个按实际，得 %d", len(got))
	}
}

func TestSelectCoveredWords_SkipMultiWordPhrase(t *testing.T) {
	// 含空格/标点的多词短语（如 "Nice to see you!"、"put away"）不应被选为覆盖词
	first := []CandidateWord{
		{Word: mkWord("w1", "tidy", StatusUnlearned)},
		{Word: mkWord("w2", "Nice to see you!", StatusUnlearned)},
		{Word: mkWord("w3", "put away", StatusUnlearned)},
		{Word: mkWord("w4", "tired", StatusUnlearned)},
	}
	got := SelectCoveredWords(first, nil, nil)
	for _, c := range got {
		if !isSingleWord(c.Word.Text) {
			t.Fatalf("多词短语不应被选中：%q", c.Word.Text)
		}
	}
	if len(got) != 2 {
		t.Fatalf("过滤后应剩 2 个单词，得 %d", len(got))
	}
}

func TestCountEnglishWords(t *testing.T) {
	cases := []struct {
		text string
		want int
	}{
		{"I eat an apple every day.", 6},
		{"hello", 1},
		{"", 0},
		{"it's a test", 3},     // it's 算 1 个（含撇号）
		{"well-known word", 2}, // well-known 算 1 个（含连字符）
	}
	for _, c := range cases {
		if got := CountEnglishWords(c.text); got != c.want {
			t.Errorf("CountEnglishWords(%q)=%d，期望 %d", c.text, got, c.want)
		}
	}
}

func TestValidatePassage_Length(t *testing.T) {
	covered := []CandidateWord{{Word: WordInfo{ID: "w", Text: "apple"}}}
	if reason := ValidatePassage("short text", covered); reason == "" {
		t.Fatal("短文过短应判不合格")
	}
}

func TestValidatePassage_OccurrenceZero(t *testing.T) {
	// 长度足够但目标词未出现（0次），应判合格（不要求一定出现）
	long := makeLongText(90, "filler")
	covered := []CandidateWord{{Word: WordInfo{ID: "w", Text: "apple"}}}
	if reason := ValidatePassage(long, covered); reason != "" {
		t.Fatalf("目标词未出现应判合格，被误判：%s", reason)
	}
}

func TestValidatePassage_OK(t *testing.T) {
	// 长度足够 + 每词最多 2 次
	text := "apple apple banana banana " + makeLongText(80, "filler")
	covered := []CandidateWord{
		{Word: WordInfo{ID: "a", Text: "apple"}},
		{Word: WordInfo{ID: "b", Text: "banana"}},
	}
	if reason := ValidatePassage(text, covered); reason != "" {
		t.Fatalf("合格短文被误判：%s", reason)
	}
}

func TestValidatePassage_RejectsMoreThanTwoOccurrences(t *testing.T) {
	// 出现 3 次应判不合格（最多 2 次）
	text := "apple apple apple " + makeLongText(80, "filler")
	covered := []CandidateWord{{Word: WordInfo{ID: "a", Text: "apple"}}}
	if reason := ValidatePassage(text, covered); reason == "" {
		t.Fatal("目标词超过2次应判不合格")
	}
}

func TestHighlightText_CountsAndMarks(t *testing.T) {
	text := "Apple and apple are the same. Banana is banana."
	covered := []CandidateWord{
		{Word: WordInfo{ID: "a", Text: "apple"}},
		{Word: WordInfo{ID: "b", Text: "banana"}},
	}
	htmlText, counts := HighlightText(text, covered)
	if counts["a"] != 2 {
		t.Fatalf("apple 出现次数=%d，期望 2", counts["a"])
	}
	if counts["b"] != 2 {
		t.Fatalf("banana 出现次数=%d，期望 2", counts["b"])
	}
	// 应包含 <mark> 标签
	if !containsStr(htmlText, "<mark>") {
		t.Fatalf("高亮 HTML 缺少 <mark>：%s", htmlText)
	}
}

func TestHighlightText_PreservesCase(t *testing.T) {
	text := "Apple is tasty."
	covered := []CandidateWord{{Word: WordInfo{ID: "a", Text: "apple"}}}
	htmlText, counts := HighlightText(text, covered)
	if counts["a"] != 1 {
		t.Fatalf("大小写不敏感匹配失败，得 %d", counts["a"])
	}
	if !containsStr(htmlText, "<mark>Apple</mark>") {
		t.Fatalf("应保留原大小写：%s", htmlText)
	}
}

func TestHighlightText_LongWordBeforeShort(t *testing.T) {
	// their 不应被 the 的替换破坏
	text := "the cat and their dog. the end. their home."
	covered := []CandidateWord{
		{Word: WordInfo{ID: "the", Text: "the"}},
		{Word: WordInfo{ID: "their", Text: "their"}},
	}
	_, counts := HighlightText(text, covered)
	// the 单独出现 2 次（the cat、the end）；their 中的 the 不算（单词边界）
	if counts["the"] != 2 {
		t.Fatalf("the 计数=%d，期望 2", counts["the"])
	}
	if counts["their"] != 2 {
		t.Fatalf("their 计数=%d，期望 2", counts["their"])
	}
}

func TestHighlightText_EscapesNonMatchedHTML(t *testing.T) {
	// XSS 防护：命中词之间的正文片段（含 < > &）必须被 HTML 转义，
	// 不能原样透传给前端 v-html。
	text := "apple <script>alert(1)</script> banana"
	covered := []CandidateWord{
		{Word: WordInfo{ID: "a", Text: "apple"}},
		{Word: WordInfo{ID: "b", Text: "banana"}},
	}
	htmlText, counts := HighlightText(text, covered)
	if counts["a"] != 1 || counts["b"] != 1 {
		t.Fatalf("计数错误：apple=%d banana=%d", counts["a"], counts["b"])
	}
	if containsStr(htmlText, "<script>") {
		t.Fatalf("未转义的 <script> 泄漏进 HTML：%s", htmlText)
	}
	if !containsStr(htmlText, "&lt;script&gt;") {
		t.Fatalf("正文应被转义为 &lt;script&gt;，得：%s", htmlText)
	}
	if !containsStr(htmlText, "<mark>apple</mark>") || !containsStr(htmlText, "<mark>banana</mark>") {
		t.Fatalf("命中词仍应被 <mark> 包裹：%s", htmlText)
	}
}

func TestHighlightText_EscapesAmpersand(t *testing.T) {
	// & 必须转义为 &amp;，避免与后续实体产生歧义
	text := "apple & banana"
	covered := []CandidateWord{{Word: WordInfo{ID: "a", Text: "apple"}}}
	htmlText, counts := HighlightText(text, covered)
	if counts["a"] != 1 {
		t.Fatalf("apple 计数=%d，期望 1", counts["a"])
	}
	if !containsStr(htmlText, "&amp;") {
		t.Fatalf("& 应被转义为 &amp;，得：%s", htmlText)
	}
}

func TestParsePassageJSON(t *testing.T) {
	raw := `{"title":"My Day","text":"I eat an apple.","sceneHint":"这是一个早晨"}`
	title, text, scene, err := parsePassageJSON(raw)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if title != "My Day" || text != "I eat an apple." || scene != "这是一个早晨" {
		t.Fatalf("解析值不对：title=%s text=%s scene=%s", title, text, scene)
	}
}

func TestParsePassageJSON_StripsCodeFence(t *testing.T) {
	raw := "```json\n{\"title\":\"T\",\"text\":\"hello world\",\"sceneHint\":\"场景\"}\n```"
	_, text, _, err := parsePassageJSON(raw)
	if err != nil || text != "hello world" {
		t.Fatalf("代码块围栏未剥离：err=%v text=%s", err, text)
	}
}

func TestParsePassageJSON_ExtractsFromNoise(t *testing.T) {
	// 模型可能前后加解释文字
	raw := `好的，这是短文：{"title":"T","text":"hello","sceneHint":"提示"} 希望你喜欢。`
	_, text, _, err := parsePassageJSON(raw)
	if err != nil || text != "hello" {
		t.Fatalf("应截取 JSON 部分：err=%v text=%s", err, text)
	}
}

// makeLongText 生成长度足够的填充文本。
func makeLongText(words int, word string) string {
	out := ""
	for i := 0; i < words; i++ {
		out += word + " "
	}
	return out
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
