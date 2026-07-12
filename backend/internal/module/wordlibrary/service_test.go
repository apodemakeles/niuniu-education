package wordlibrary

import (
	"context"
	"log/slog"
	"testing"

	"github.com/apodemakeles/niuniu-education/backend/internal/platform/llm"
	"github.com/apodemakeles/niuniu-education/backend/internal/platform/ocr"
)

// newTestService 用内存 DB + mock OCR provider 构造 service。
func newTestService(t *testing.T) (*Service, *Store) {
	t.Helper()
	store := NewStore(newTestDB(t))
	svc := NewService(store, ocr.NewMockProvider(), llm.NewMockProvider(""), slog.Default())
	return svc, store
}

// TestConfirmImport_AddsAndSetsDefaultStatus 验证入库并按类型设默认状态。
func TestConfirmImport_AddsAndSetsDefaultStatus(t *testing.T) {
	svc, store := newTestService(t)
	ctx := context.Background()

	resp, err := svc.ConfirmImport(ctx, ConfirmImportRequest{Rows: []ConfirmRow{
		{Text: "apple", MeaningZh: "苹果", WordType: TypeNew},
		{Text: "climb", MeaningZh: "攀爬", WordType: TypeMistake},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Added != 2 {
		t.Errorf("added = %d, want 2", resp.Added)
	}

	// 验证默认状态落库
	all, _ := store.List(ctx, ListParams{})
	byText := map[string]Word{}
	for _, w := range all.Words {
		byText[w.Text] = w
	}
	if byText["apple"].Status != StatusUnlearned {
		t.Errorf("apple status = %q, want unlearned", byText["apple"].Status)
	}
	if byText["climb"].Status != StatusReinforce {
		t.Errorf("climb status = %q, want reinforce", byText["climb"].Status)
	}
}

// TestConfirmImport_DuplicateSkipped 已存在的 text+meaningZh 被跳过。
func TestConfirmImport_DuplicateSkipped(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	// 第一次插入
	_, _ = svc.ConfirmImport(ctx, ConfirmImportRequest{Rows: []ConfirmRow{
		{Text: "apple", MeaningZh: "苹果"},
	}})
	// 第二次同样的应跳过
	resp, err := svc.ConfirmImport(ctx, ConfirmImportRequest{Rows: []ConfirmRow{
		{Text: "apple", MeaningZh: "苹果"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Skipped != 1 || resp.Added != 0 {
		t.Errorf("got added=%d skipped=%d, want added=0 skipped=1", resp.Added, resp.Skipped)
	}
}

// TestConfirmImport_EmptyTextInvalid 英文单词为空的行标记 invalid，不入库。
func TestConfirmImport_EmptyTextInvalid(t *testing.T) {
	svc, store := newTestService(t)
	ctx := context.Background()

	resp, err := svc.ConfirmImport(ctx, ConfirmImportRequest{Rows: []ConfirmRow{
		{Text: "", MeaningZh: "空英文"},
		{Text: "apple", MeaningZh: "苹果"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Invalid != 1 || resp.Added != 1 {
		t.Errorf("got invalid=%d added=%d, want invalid=1 added=1", resp.Invalid, resp.Added)
	}
	all, _ := store.List(ctx, ListParams{})
	if len(all.Words) != 1 {
		t.Errorf("DB 应只有 1 行，got %d", len(all.Words))
	}
}

// TestConfirmImport_DuplicateButDifferentMeaningAdds PRD：仅 text 相同释义不同允许入库。
func TestConfirmImport_DuplicateButDifferentMeaningAdds(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	_, _ = svc.ConfirmImport(ctx, ConfirmImportRequest{Rows: []ConfirmRow{
		{Text: "apple", MeaningZh: "苹果"},
	}})
	// 同 text 不同 meaning 应入库（同形词/多义词）
	resp, _ := svc.ConfirmImport(ctx, ConfirmImportRequest{Rows: []ConfirmRow{
		{Text: "apple", MeaningZh: "苹果公司"},
	}})
	if resp.Added != 1 {
		t.Errorf("同形异义应入库：added=%d, want 1", resp.Added)
	}
}

// TestParsePasteForDraft 验证粘贴文本解析为草稿行（不写库）。
func TestParsePasteForDraft(t *testing.T) {
	svc, _ := newTestService(t)

	cases := []struct {
		name string
		text string
		want []struct{ text, meaning string }
	}{
		{
			"空格分隔含音标",
			"apple /ˈæpl/ 苹果\nbanana /bəˈnɑːnə/ 香蕉",
			[]struct{ text, meaning string }{{"apple", "苹果"}, {"banana", "香蕉"}},
		},
		{
			"逗号分隔",
			"apple,苹果,/ˈæpl/\nread,阅读",
			[]struct{ text, meaning string }{{"apple", "苹果"}, {"read", "阅读"}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rows := svc.ParsePasteForDraft(c.text)
			if len(rows) != len(c.want) {
				t.Fatalf("got %d rows, want %d: %+v", len(rows), len(c.want), rows)
			}
			for i, w := range c.want {
				if rows[i].Text != w.text || rows[i].MeaningZh != w.meaning {
					t.Errorf("row%d = %+v, want text=%q meaning=%q", i, rows[i], w.text, w.meaning)
				}
			}
		})
	}
}

// TestRecognizeForDraft 用 mock provider 验证草稿生成（不写库）。
func TestRecognizeForDraft(t *testing.T) {
	svc, store := newTestService(t)
	ctx := context.Background()

	rows, rawText, err := svc.RecognizeForDraft(ctx, []byte("fake-image"), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 mock rows, got %d", len(rows))
	}
	if rows[0].Text != "clirnb" {
		t.Errorf("first mock row = %q, want clirnb", rows[0].Text)
	}
	if rawText == "" {
		t.Error("rawText 不应为空")
	}
	// 验证不写库
	all, _ := store.List(ctx, ListParams{})
	if len(all.Words) != 0 {
		t.Errorf("草稿阶段不应写库，got %d 行", len(all.Words))
	}
	// 低置信度行应被标记 low_confidence
	hasFlag := false
	for _, r := range rows {
		for _, iss := range r.Issues {
			if iss == "low_confidence" {
				hasFlag = true
			}
		}
	}
	if !hasFlag {
		t.Error("低置信度 mock 行(clirnb 0.62)未被标记 low_confidence")
	}
}
