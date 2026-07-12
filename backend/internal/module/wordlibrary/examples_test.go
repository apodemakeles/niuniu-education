package wordlibrary

import (
	"context"
	"log/slog"
	"testing"

	"github.com/apodemakeles/niuniu-education/backend/internal/platform/llm"
	"github.com/apodemakeles/niuniu-education/backend/internal/platform/ocr"
)

func TestGenerateExamplesForWord_StoresThreeValidatedSentences(t *testing.T) {
	store := NewStore(newTestDB(t))
	w, err := store.CreateWord(context.Background(), CreateWordParams{Text: "kitchen", MeaningZh: "厨房"})
	if err != nil {
		t.Fatal(err)
	}
	provider := llm.NewMockProvider(`{"sentences":["I eat in the kitchen.","Mom cooks in the kitchen.","The cat sleeps in the kitchen."]}`)
	svc := NewService(store, ocr.NewMockProvider(), provider, slog.Default())
	examples, err := svc.examples.GenerateForWord(context.Background(), w.ID)
	if err != nil {
		t.Fatalf("GenerateForWord: %v", err)
	}
	if len(examples) != 3 {
		t.Fatalf("例句数=%d，期望 3", len(examples))
	}
	for _, example := range examples {
		if !containsTarget(example.Sentence, "kitchen") {
			t.Errorf("例句缺目标词：%q", example.Sentence)
		}
	}
}

func TestGenerateExamplesForWord_InvalidOutputDoesNotWrite(t *testing.T) {
	store := NewStore(newTestDB(t))
	w, err := store.CreateWord(context.Background(), CreateWordParams{Text: "kitchen", MeaningZh: "厨房"})
	if err != nil {
		t.Fatal(err)
	}
	provider := llm.NewMockProvider(`{"sentences":["A very long sentence without the target word.","Another sentence without it.","Third sentence without it."]}`)
	svc := NewService(store, ocr.NewMockProvider(), provider, slog.Default())
	if _, err := svc.examples.GenerateForWord(context.Background(), w.ID); err == nil {
		t.Fatal("不合格输出应返回错误")
	}
	examples, err := store.ListExamples(context.Background(), w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(examples) != 0 {
		t.Fatalf("不合格输出不应写库，got %+v", examples)
	}
}

func TestContainsTarget_RequiresWordBoundary(t *testing.T) {
	if containsTarget("The dog is here.", "he") {
		t.Fatal("he 不应在 The 中被误判为目标词")
	}
	if !containsTarget("He has a red ball.", "he") {
		t.Fatal("完整目标词应被识别")
	}
}
