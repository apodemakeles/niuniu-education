package wordlibrary

import (
	"context"
	"database/sql"
	"fmt"
)

// Store 封装 words 表的数据访问。M1 最小子集，供导入流程与列表查询使用。
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// ListParams 列表查询参数。
type ListParams struct {
	Type   string // all/new/mistake
	Status string // all/...
	Q      string // 在 text/meaning_zh/phonetic 中模糊匹配
}

// List 返回未软删的单词列表（按创建时间倒序）。
func (s *Store) List(ctx context.Context, p ListParams) ([]Word, error) {
	q := `SELECT id, library_id, text, meaning_zh, phonetic, word_type, status,
	             created_at, updated_at, last_edited_at, source
	      FROM words
	      WHERE deleted_at IS NULL AND library_id = ?`
	args := []any{MainLibraryID}
	if p.Type == TypeNew || p.Type == TypeMistake {
		q += ` AND word_type = ?`
		args = append(args, p.Type)
	}
	if p.Status != "" && p.Status != "all" {
		q += ` AND status = ?`
		args = append(args, p.Status)
	}
	if p.Q != "" {
		q += ` AND (text LIKE ? OR meaning_zh LIKE ? OR phonetic LIKE ?)`
		like := "%" + p.Q + "%"
		args = append(args, like, like, like)
	}
	q += ` ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query words: %w", err)
	}
	defer rows.Close()

	var words []Word
	for rows.Next() {
		var w Word
		var lastEdited sql.NullString
		if err := rows.Scan(&w.ID, &w.LibraryID, &w.Text, &w.MeaningZh, &w.Phonetic,
			&w.WordType, &w.Status, &w.CreatedAt, &w.UpdatedAt, &lastEdited, &w.Source); err != nil {
			return nil, fmt.Errorf("scan word: %w", err)
		}
		w.LastEditedAt = lastEdited.String
		words = append(words, w)
	}
	return words, rows.Err()
}

// CreateWordParams 新增单词入参。
type CreateWordParams struct {
	Text      string
	MeaningZh string
	Phonetic  string
	WordType  string
	Status    string
	Source    string
}

// CreateWord 插入一条单词。ID 由 SQLite 生成并回填。
func (s *Store) CreateWord(ctx context.Context, p CreateWordParams) (Word, error) {
	if p.WordType == "" {
		p.WordType = TypeNew
	}
	if p.Status == "" {
		p.Status = DefaultStatusForType(p.WordType)
	}
	if p.Source == "" {
		p.Source = SourceManual
	}
	q := `INSERT INTO words (id, library_id, text, meaning_zh, phonetic, word_type, status, source)
	      VALUES (lower(hex(randomblob(8))), ?, ?, ?, ?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, q, MainLibraryID, p.Text, p.MeaningZh, p.Phonetic, p.WordType, p.Status, p.Source)
	if err != nil {
		return Word{}, fmt.Errorf("insert word: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Word{}, fmt.Errorf("insert word: 未写入")
	}
	// 查回插入行（含生成 id 与时间戳）
	return s.findByTextMeaning(ctx, p.Text, p.MeaningZh)
}

func (s *Store) findByTextMeaning(ctx context.Context, text, meaning string) (Word, error) {
	q := `SELECT id, library_id, text, meaning_zh, phonetic, word_type, status,
	             created_at, updated_at, last_edited_at, source
	      FROM words WHERE library_id=? AND text=? AND meaning_zh=? AND deleted_at IS NULL
	      ORDER BY created_at DESC LIMIT 1`
	var w Word
	var lastEdited sql.NullString
	err := s.db.QueryRowContext(ctx, q, MainLibraryID, text, meaning).
		Scan(&w.ID, &w.LibraryID, &w.Text, &w.MeaningZh, &w.Phonetic, &w.WordType, &w.Status,
			&w.CreatedAt, &w.UpdatedAt, &lastEdited, &w.Source)
	if err != nil {
		return Word{}, fmt.Errorf("find word after insert: %w", err)
	}
	w.LastEditedAt = lastEdited.String
	return w, nil
}

// ExistsByTextMeaning 判断词库内是否已存在 text+meaningZh 的单词（仅未软删）。
// 对应 PRD 的去重规则：按 text + meaning_zh 判定重复。
func (s *Store) ExistsByTextMeaning(ctx context.Context, text, meaning string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM words
		 WHERE library_id=? AND text=? AND meaning_zh=? AND deleted_at IS NULL`,
		MainLibraryID, text, meaning).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check duplicate: %w", err)
	}
	return count > 0, nil
}
