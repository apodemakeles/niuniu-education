package wordlibrary

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	// 纯 Go SQLite 驱动
	_ "modernc.org/sqlite"
)

// newTestDB 返回一个内存 SQLite 并执行建表与预置词库，每个测试独立、无文件残留。
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open memory sqlite: %v", err)
	}
	schema := `
	CREATE TABLE libraries (
		id TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '默认词库',
		created_at TEXT NOT NULL DEFAULT (datetime('now')), updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);
	CREATE TABLE words (
		id TEXT PRIMARY KEY, library_id TEXT NOT NULL REFERENCES libraries(id),
		text TEXT NOT NULL, meaning_zh TEXT NOT NULL DEFAULT '', phonetic TEXT NOT NULL DEFAULT '',
		word_type TEXT NOT NULL DEFAULT 'new' CHECK (word_type IN ('new','mistake')),
		status TEXT NOT NULL DEFAULT 'unlearned' CHECK (status IN ('unlearned','learning','reinforce','mastered')),
		first_learned_at TEXT, last_reviewed_at TEXT, review_count INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now')), updated_at TEXT NOT NULL DEFAULT (datetime('now')),
		last_edited_at TEXT, source TEXT NOT NULL DEFAULT 'manual', deleted_at TEXT
	);
	CREATE UNIQUE INDEX uniq_word_text_meaning ON words(library_id, text, meaning_zh) WHERE deleted_at IS NULL;
	CREATE INDEX idx_words_type_status ON words(library_id, word_type, status) WHERE deleted_at IS NULL;
	CREATE TABLE import_images (
		id TEXT PRIMARY KEY, library_id TEXT NOT NULL, file_path TEXT NOT NULL, mime_type TEXT NOT NULL,
		uploaded_at TEXT NOT NULL DEFAULT (datetime('now')), ocr_raw_text TEXT, provider TEXT,
		status TEXT NOT NULL DEFAULT 'pending'
	);
	INSERT INTO libraries(id, name) VALUES ('main-library', '默认词库');
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

// TestCreateWord_DefaultStatus 验证按原型规则：新词默认 unlearned，易错词默认 reinforce。
func TestCreateWord_DefaultStatus(t *testing.T) {
	store := NewStore(newTestDB(t))
	ctx := context.Background()

	cases := []struct {
		name     string
		wordType string
		wantStat string
	}{
		{"新词默认未学", TypeNew, StatusUnlearned},
		{"易错词默认需强化", TypeMistake, StatusReinforce},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w, err := store.CreateWord(ctx, CreateWordParams{
				Text: "test" + c.wordType, MeaningZh: "测试", WordType: c.wordType,
			})
			if err != nil {
				t.Fatalf("CreateWord: %v", err)
			}
			if w.Status != c.wantStat {
				t.Errorf("status = %q, want %q", w.Status, c.wantStat)
			}
			if w.ID == "" {
				t.Error("ID 未生成")
			}
		})
	}
}

// TestCreateWord_EmptyTypeDefaultsToNew wordType 为空时默认 new。
func TestCreateWord_EmptyTypeDefaultsToNew(t *testing.T) {
	store := NewStore(newTestDB(t))
	w, err := store.CreateWord(context.Background(), CreateWordParams{Text: "x", MeaningZh: "y"})
	if err != nil {
		t.Fatalf("CreateWord: %v", err)
	}
	if w.WordType != TypeNew || w.Status != StatusUnlearned {
		t.Errorf("got type=%q status=%q, want new/unlearned", w.WordType, w.Status)
	}
}

// TestExistsByTextMeaning 验证 PRD 去重规则：text+meaningZh 命中才算重复。
func TestExistsByTextMeaning(t *testing.T) {
	store := NewStore(newTestDB(t))
	ctx := context.Background()
	if _, err := store.CreateWord(ctx, CreateWordParams{Text: "apple", MeaningZh: "苹果"}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name      string
		text      string
		meaning   string
		wantExist bool
	}{
		{"完全命中", "apple", "苹果", true},
		{"仅 text 相同释义不同不判重", "apple", "苹果公司", false}, // PRD：仅 text 相同只提示不强拦
		{"text 不同", "banana", "苹果", false},
		{"都不同", "x", "y", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := store.ExistsByTextMeaning(ctx, c.text, c.meaning)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.wantExist {
				t.Errorf("Exists(%q,%q) = %v, want %v", c.text, c.meaning, got, c.wantExist)
			}
		})
	}
}

// TestList_Filters 验证按 type/status/q 筛选，且不返回软删行。
func TestList_Filters(t *testing.T) {
	store := NewStore(newTestDB(t))
	ctx := context.Background()
	seed := []CreateWordParams{
		{Text: "apple", MeaningZh: "苹果", Phonetic: "/ˈæpl/", WordType: TypeNew},
		{Text: "read", MeaningZh: "阅读", Phonetic: "/riːd/", WordType: TypeMistake},
		{Text: "desk", MeaningZh: "书桌", WordType: TypeNew},
	}
	for _, p := range seed {
		if _, err := store.CreateWord(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	// 全部
	all, err := store.List(ctx, ListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Words) != 3 {
		t.Errorf("List() = %d, want 3", len(all.Words))
	}

	// type 筛选
	newOnly, _ := store.List(ctx, ListParams{Type: TypeNew})
	if got := len(newOnly.Words); got != 2 {
		t.Errorf("type=new = %d, want 2", got)
	}

	// 搜索 q（前缀匹配 text）
	apple, _ := store.List(ctx, ListParams{Q: "app"})
	if len(apple.Words) != 1 || apple.Words[0].Text != "apple" {
		t.Errorf("q=app = %+v", apple.Words)
	}
	appMid, _ := store.List(ctx, ListParams{Q: "ple"})
	if len(appMid.Words) != 0 {
		t.Errorf("q=ple 中间匹配应不命中, got %+v", appMid.Words)
	}

	// 搜索 q（前缀匹配 meaning_zh 中文）
	zh, _ := store.List(ctx, ListParams{Q: "书桌"})
	if len(zh.Words) != 1 || zh.Words[0].Text != "desk" {
		t.Errorf("q=书桌 = %+v", zh.Words)
	}
	zhMid, _ := store.List(ctx, ListParams{Q: "桌"})
	if len(zhMid.Words) != 0 {
		t.Errorf("q=桌 中间匹配应不命中, got %+v", zhMid.Words)
	}
}

// TestUpdate_ChangesFieldsNotText 验证更新属性（不含 text），并刷新 last_edited_at。
func TestUpdate_ChangesFieldsNotText(t *testing.T) {
	store := NewStore(newTestDB(t))
	ctx := context.Background()
	w, _ := store.CreateWord(ctx, CreateWordParams{Text: "apple", MeaningZh: "苹果", WordType: TypeNew})

	updated, err := store.Update(ctx, UpdateParams{
		ID: w.ID, MeaningZh: "苹果果", Phonetic: "/ˈæpl/", WordType: TypeMistake, Status: StatusReinforce,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.MeaningZh != "苹果果" || updated.WordType != TypeMistake || updated.Status != StatusReinforce {
		t.Errorf("更新后字段不符: %+v", updated)
	}
	if updated.Text != "apple" {
		t.Errorf("text 不应变化: got %q", updated.Text)
	}
	if updated.LastEditedAt == "" {
		t.Error("last_edited_at 应被刷新")
	}
}

// TestUpdate_NotFound 更新不存在的 ID 返回 ErrNotFound。
func TestUpdate_NotFound(t *testing.T) {
	store := NewStore(newTestDB(t))
	_, err := store.Update(context.Background(), UpdateParams{ID: "nope", MeaningZh: "x"})
	if err != ErrNotFound {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

// TestDelete_PhysicalForUnlearned 未学单词物理删除（列表查不到，可重新插入）。
func TestDelete_PhysicalForUnlearned(t *testing.T) {
	store := NewStore(newTestDB(t))
	ctx := context.Background()
	w, _ := store.CreateWord(ctx, CreateWordParams{Text: "apple", MeaningZh: "苹果"}) // 默认 unlearned

	res, err := store.Delete(ctx, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != DeletePhysical {
		t.Errorf("kind = %s, want physical", res.Kind)
	}
	// 列表应为空
	all, _ := store.List(ctx, ListParams{})
	if len(all.Words) != 0 {
		t.Errorf("物理删后列表 = %d, want 0", len(all.Words))
	}
	// 可重新插入（物理删后唯一索引不占位）
	exist, _ := store.ExistsByTextMeaning(ctx, "apple", "苹果")
	if exist {
		t.Error("物理删后应可重新插入")
	}
}

// TestDelete_LogicalForLearned 学习中的单词软删（仍占唯一索引位，列表查不到）。
func TestDelete_LogicalForLearned(t *testing.T) {
	store := NewStore(newTestDB(t))
	ctx := context.Background()
	w, _ := store.CreateWord(ctx, CreateWordParams{Text: "apple", MeaningZh: "苹果"})
	// 改为学习中
	_, _ = store.Update(ctx, UpdateParams{ID: w.ID, MeaningZh: "苹果", WordType: TypeNew, Status: StatusLearning})

	res, err := store.Delete(ctx, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != DeleteLogical {
		t.Errorf("kind = %s, want logical", res.Kind)
	}
	// 列表查不到
	all, _ := store.List(ctx, ListParams{})
	if len(all.Words) != 0 {
		t.Errorf("软删后列表 = %d, want 0", len(all.Words))
	}
}

// TestDelete_NotFound 删除不存在的 ID 返回 ErrNotFound。
func TestDelete_NotFound(t *testing.T) {
	store := NewStore(newTestDB(t))
	_, err := store.Delete(context.Background(), "nope")
	if err != ErrNotFound {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

// TestList_ExcludesSoftDeleted 验证软删行不出现在列表。
func TestList_ExcludesSoftDeleted(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	ctx := context.Background()
	w, _ := store.CreateWord(ctx, CreateWordParams{Text: "apple", MeaningZh: "苹果"})

	// 软删
	if _, err := db.ExecContext(ctx, `UPDATE words SET deleted_at=datetime('now') WHERE id=?`, w.ID); err != nil {
		t.Fatal(err)
	}
	rows, _ := store.List(ctx, ListParams{})
	if len(rows.Words) != 0 {
		t.Errorf("软删后 List() = %d, want 0", len(rows.Words))
	}

	// 软删的词不再算重复（部分唯一索引仅覆盖未删行）
	exist, _ := store.ExistsByTextMeaning(ctx, "apple", "苹果")
	if exist {
		t.Error("软删后不应判重，应允许重新插入")
	}
}

// TestList_Pagination 验证分页与 total 计数。
func TestList_Pagination(t *testing.T) {
	store := NewStore(newTestDB(t))
	ctx := context.Background()
	for i := 1; i <= 5; i++ {
		if _, err := store.CreateWord(ctx, CreateWordParams{
			Text: fmt.Sprintf("word%d", i), MeaningZh: fmt.Sprintf("词%d", i),
		}); err != nil {
			t.Fatal(err)
		}
	}

	page1, err := store.List(ctx, ListParams{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page1.Words) != 2 || page1.Total != 5 {
		t.Errorf("page1 = %+v, want 2 items total 5", page1)
	}
	page3, err := store.List(ctx, ListParams{Page: 3, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page3.Words) != 1 || page3.Total != 5 {
		t.Errorf("page3 = %+v, want 1 item total 5", page3)
	}
}

// TestCountStats 验证词库统计。
func TestCountStats(t *testing.T) {
	store := NewStore(newTestDB(t))
	ctx := context.Background()
	if _, err := store.CreateWord(ctx, CreateWordParams{Text: "a", MeaningZh: "甲", WordType: TypeNew}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateWord(ctx, CreateWordParams{Text: "b", MeaningZh: "乙", WordType: TypeMistake}); err != nil {
		t.Fatal(err)
	}
	stats, err := store.CountStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Total != 2 || stats.NewWords != 1 || stats.MistakeWords != 1 {
		t.Errorf("stats = %+v, want total=2 new=1 mistake=1", stats)
	}
}
