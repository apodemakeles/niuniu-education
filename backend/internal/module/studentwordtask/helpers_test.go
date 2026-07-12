package studentwordtask

import (
	"context"
	"database/sql"
	"log/slog"
	"testing"
	"time"

	"github.com/apodemakeles/niuniu-education/backend/internal/platform/llm"

	_ "modernc.org/sqlite"
)

// newTestDB 返回内存 SQLite 并建好学生端模块所需的全部表 + 默认策略行。
// 每个 case 独立，无文件残留。
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
	INSERT INTO libraries(id, name) VALUES ('main-library', '默认词库');

	CREATE TABLE word_learning (
		word_id TEXT PRIMARY KEY REFERENCES words(id) ON DELETE CASCADE,
		library_id TEXT NOT NULL DEFAULT 'main-library',
		learning_status TEXT NOT NULL DEFAULT 'unlearned',
		success_count INTEGER NOT NULL DEFAULT 0,
		studied_dates TEXT NOT NULL DEFAULT '[]',
		success_dates TEXT NOT NULL DEFAULT '[]',
		last_studied_date TEXT, last_success_date TEXT, next_due_date TEXT,
		mastered_review_index INTEGER NOT NULL DEFAULT 0, first_mastered_date TEXT,
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);
	CREATE TABLE daily_tasks (
		id TEXT PRIMARY KEY, task_date TEXT NOT NULL, word_id TEXT NOT NULL REFERENCES words(id) ON DELETE CASCADE,
		library_id TEXT NOT NULL DEFAULT 'main-library', pool_type TEXT NOT NULL CHECK (pool_type IN ('first','review')),
		display_order INTEGER NOT NULL DEFAULT 0, read_status TEXT NOT NULL DEFAULT 'not_started',
		first_dictation_answer TEXT NOT NULL DEFAULT '', first_dictation_result TEXT NOT NULL DEFAULT 'pending',
		used_hint INTEGER NOT NULL DEFAULT 0, submission_locked INTEGER NOT NULL DEFAULT 0,
		correction_answer TEXT NOT NULL DEFAULT '', correction_result TEXT NOT NULL DEFAULT 'not_submitted',
		created_at TEXT NOT NULL DEFAULT (datetime('now')), updated_at TEXT NOT NULL DEFAULT (datetime('now')),
		UNIQUE (task_date, word_id)
	);
	CREATE TABLE reading_passages (
		task_date TEXT PRIMARY KEY, library_id TEXT NOT NULL DEFAULT 'main-library',
		reading_title TEXT NOT NULL DEFAULT '', reading_text TEXT NOT NULL DEFAULT '',
		covered_word_ids TEXT NOT NULL DEFAULT '[]', word_occurrence_counts TEXT NOT NULL DEFAULT '{}',
		reading_scene_hint TEXT NOT NULL DEFAULT '', ai_generation_status TEXT NOT NULL DEFAULT 'pending',
		reading_started_at TEXT, reading_completed_at TEXT, reading_min_seconds INTEGER NOT NULL DEFAULT 120,
		created_at TEXT NOT NULL DEFAULT (datetime('now')), updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);
	CREATE TABLE checkins (
		task_date TEXT PRIMARY KEY, library_id TEXT NOT NULL DEFAULT 'main-library',
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	);
	CREATE TABLE review_strategy (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		new_pool_count INTEGER NOT NULL DEFAULT 3, review_pool_max_count INTEGER NOT NULL DEFAULT 9,
		review_pool_min_score INTEGER NOT NULL DEFAULT 70, reinforce_base_score INTEGER NOT NULL DEFAULT 90,
		learning_base_score INTEGER NOT NULL DEFAULT 70, mastered_base_score INTEGER NOT NULL DEFAULT 10,
		due_today_bonus INTEGER NOT NULL DEFAULT 40, overdue_daily_bonus INTEGER NOT NULL DEFAULT 8,
		overdue_bonus_cap INTEGER NOT NULL DEFAULT 56, mastered_due_sample_ratio INTEGER NOT NULL DEFAULT 1,
		mastered_sample_stop_day INTEGER NOT NULL DEFAULT 120,
		review_overdue_reduce_new INTEGER NOT NULL DEFAULT 3, review_overdue_pause_new INTEGER NOT NULL DEFAULT 6,
		reading_min_seconds INTEGER NOT NULL DEFAULT 120,
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);
	INSERT INTO review_strategy(id) VALUES (1);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	// PRAGMA foreign_keys 在内存 SQLite 中需要每个连接开启；测试用单连接。
	db.SetMaxOpenConns(1)
	return db
}

// newTestService 用内存 DB + mock LLM 构造 service。
func newTestService(t *testing.T) (*Service, *Store) {
	t.Helper()
	db := newTestDB(t)
	store := NewStore(db)
	svc := NewService(store, llm.NewMockProvider(""), "", testLogger())
	return svc, store
}

func testLogger() *slog.Logger { return slog.Default() }

// seedWord 插入一个单词（不写 word_learning，由 EnsureLearnings 补）。
func seedWord(t *testing.T, db *sql.DB, id, text, meaning, phonetic, wordType, status string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO words(id, library_id, text, meaning_zh, phonetic, word_type, status, source) VALUES (?, 'main-library', ?, ?, ?, ?, ?, 'manual')`,
		id, text, meaning, phonetic, wordType, status)
	if err != nil {
		t.Fatalf("seed word %s: %v", id, err)
	}
}

// seedLearning 直接写入一条学习记录（覆盖 EnsureLearnings 的默认值）。
func seedLearning(t *testing.T, db *sql.DB, wordID, status string, successCount int, nextDue string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO word_learning(word_id, learning_status, success_count, next_due_date) VALUES (?, ?, ?, ?)
		ON CONFLICT(word_id) DO UPDATE SET learning_status=excluded.learning_status, success_count=excluded.success_count, next_due_date=excluded.next_due_date`,
		wordID, status, successCount, nextDue)
	if err != nil {
		t.Fatalf("seed learning %s: %v", wordID, err)
	}
}

// mustParseDate 测试用固定日期解析。
func mustParseDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func ctxbg() context.Context { return context.Background() }
