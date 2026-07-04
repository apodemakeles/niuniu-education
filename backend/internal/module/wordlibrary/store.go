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

	// 初始化为空切片而非 nil，保证序列化为 [] 而非 null（前端容错）
	words := []Word{}
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

// ResetAll 物理清空所有单词（含软删行），仅供测试用。
// 通过专用测试端点调用，生产路由表不注册该端点。
func (s *Store) ResetAll(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM words`)
	return err
}

// Get 按 ID 查询单个单词（未软删）。
func (s *Store) Get(ctx context.Context, id string) (Word, error) {
	q := `SELECT id, library_id, text, meaning_zh, phonetic, word_type, status,
	             created_at, updated_at, last_edited_at, source
	      FROM words WHERE id=? AND deleted_at IS NULL`
	var w Word
	var lastEdited sql.NullString
	err := s.db.QueryRowContext(ctx, q, id).
		Scan(&w.ID, &w.LibraryID, &w.Text, &w.MeaningZh, &w.Phonetic, &w.WordType, &w.Status,
			&w.CreatedAt, &w.UpdatedAt, &lastEdited, &w.Source)
	if err != nil {
		if err == sql.ErrNoRows {
			return Word{}, ErrNotFound
		}
		return Word{}, fmt.Errorf("get word: %w", err)
	}
	w.LastEditedAt = lastEdited.String
	return w, nil
}

// UpdateParams 更新单词属性（不含 text，PRD：英文单词创建后不可改）。
type UpdateParams struct {
	ID         string
	MeaningZh  string
	Phonetic   string
	WordType   string
	Status     string
}

// Update 更新单词属性。普通字段修改不重置学习进度（不触碰 review_count 等）。
// 若 wordType 改为易错词，status 不自动重置（按原型行为，交由家长在表单里设）。
func (s *Store) Update(ctx context.Context, p UpdateParams) (Word, error) {
	q := `UPDATE words
	      SET meaning_zh=?, phonetic=?, word_type=?, status=?, last_edited_at=datetime('now'), updated_at=datetime('now')
	      WHERE id=? AND deleted_at IS NULL`
	res, err := s.db.ExecContext(ctx, q, p.MeaningZh, p.Phonetic, p.WordType, p.Status, p.ID)
	if err != nil {
		return Word{}, fmt.Errorf("update word: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Word{}, ErrNotFound
	}
	return s.Get(ctx, p.ID)
}

// ErrNotFound 表示单词不存在或已软删。
var ErrNotFound = fmt.Errorf("单词不存在")

// DeleteKind 表示删除方式，对应 PRD 删除策略。
type DeleteKind string

const (
	DeletePhysical DeleteKind = "physical" // 物理删除（未学单词）
	DeleteLogical  DeleteKind = "logical"  // 逻辑删除/软删（学习中/需强化/已掌握）
)

// DeleteResult 返回删除结果。
type DeleteResult struct {
	Kind DeleteKind `json:"kind"`
}

// Delete 按 PRD 删除策略删除单词：
//   - 未学（unlearned）：物理删除
//   - 学习中/需强化/已掌握：软删（置 deleted_at），保留学习记录
func (s *Store) Delete(ctx context.Context, id string) (*DeleteResult, error) {
	w, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if w.Status == StatusUnlearned {
		// 物理删除：先软删以释放部分唯一索引占位，再物理删除
		// （若直接 DELETE，唯一索引会随行消失，无需额外处理；这里直接 DELETE）
		if _, err := s.db.ExecContext(ctx, `DELETE FROM words WHERE id=?`, id); err != nil {
			return nil, fmt.Errorf("physical delete: %w", err)
		}
		return &DeleteResult{Kind: DeletePhysical}, nil
	}

	// 逻辑删除
	if _, err := s.db.ExecContext(ctx,
		`UPDATE words SET deleted_at=datetime('now'), updated_at=datetime('now') WHERE id=?`, id); err != nil {
		return nil, fmt.Errorf("logical delete: %w", err)
	}
	return &DeleteResult{Kind: DeleteLogical}, nil
}
