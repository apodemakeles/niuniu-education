package wordlibrary

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Store 封装 words 表的数据访问。M1 最小子集，供导入流程与列表查询使用。
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// ListParams 列表查询参数。
type ListParams struct {
	Type     string // all/new/mistake
	Status   string // all/...
	Q        string // 在 text/meaning_zh/phonetic 中前缀匹配
	Page     int    // 从 1 起；0 表示不分页
	PageSize int    // <=0 表示不分页（导出等内部调用）
}

// ListResult 是分页列表查询结果。
type ListResult struct {
	Words []Word
	Total int
}

// 学习状态只从 word_learning 读取；尚未建立学习记录的词按“未学”展示。
// words.status 保留仅为旧库兼容字段，不能再作为业务判断依据。
const effectiveStatusExpr = `COALESCE(wl.learning_status, 'unlearned')`

const wordSelectColumns = `w.id, w.library_id, w.text, w.meaning_zh, w.phonetic, w.word_type, ` + effectiveStatusExpr + `,
	w.created_at, w.updated_at, w.last_edited_at, w.source`

const wordLearningJoin = ` LEFT JOIN word_learning wl ON wl.word_id = w.id AND wl.library_id = w.library_id`

type rowQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *Store) buildListWhere(p ListParams) (string, []any) {
	clause := `w.deleted_at IS NULL AND w.library_id = ?`
	args := []any{MainLibraryID}
	if p.Type == TypeNew || p.Type == TypeMistake {
		clause += ` AND w.word_type = ?`
		args = append(args, p.Type)
	}
	if p.Status != "" && p.Status != "all" {
		clause += ` AND ` + effectiveStatusExpr + ` = ?`
		args = append(args, p.Status)
	}
	if p.Q != "" {
		clause += ` AND (w.text LIKE ? OR w.meaning_zh LIKE ? OR w.phonetic LIKE ?)`
		like := p.Q + "%"
		args = append(args, like, like, like)
	}
	return clause, args
}

// CountStats 返回词库全局统计（不受列表筛选影响）。
func (s *Store) CountStats(ctx context.Context) (LibraryStats, error) {
	var stats LibraryStats
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(1),
		       COALESCE(SUM(CASE WHEN word_type = ? THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN word_type = ? THEN 1 ELSE 0 END), 0)
		FROM words
		WHERE deleted_at IS NULL AND library_id = ?`,
		TypeNew, TypeMistake, MainLibraryID).
		Scan(&stats.Total, &stats.NewWords, &stats.MistakeWords)
	if err != nil {
		return LibraryStats{}, fmt.Errorf("count stats: %w", err)
	}
	return stats, nil
}

// List 返回未软删的单词列表（按创建时间倒序），支持筛选与分页。
func (s *Store) List(ctx context.Context, p ListParams) (ListResult, error) {
	clause, args := s.buildListWhere(p)

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM words w`+wordLearningJoin+` WHERE `+clause, args...).
		Scan(&total); err != nil {
		return ListResult{}, fmt.Errorf("count words: %w", err)
	}

	q := `SELECT ` + wordSelectColumns + ` FROM words w` + wordLearningJoin + `
	      WHERE ` + clause + ` ORDER BY w.created_at DESC`

	queryArgs := append([]any{}, args...)
	if p.PageSize > 0 {
		page := p.Page
		if page < 1 {
			page = 1
		}
		offset := (page - 1) * p.PageSize
		q += ` LIMIT ? OFFSET ?`
		queryArgs = append(queryArgs, p.PageSize, offset)
	}

	rows, err := s.db.QueryContext(ctx, q, queryArgs...)
	if err != nil {
		return ListResult{}, fmt.Errorf("query words: %w", err)
	}
	defer rows.Close()

	// 初始化为空切片而非 nil，保证序列化为 [] 而非 null（前端容错）
	words := []Word{}
	for rows.Next() {
		var w Word
		var lastEdited sql.NullString
		if err := scanWord(rows, &w, &lastEdited); err != nil {
			return ListResult{}, fmt.Errorf("scan word: %w", err)
		}
		w.LastEditedAt = lastEdited.String
		words = append(words, w)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, err
	}
	return ListResult{Words: words, Total: total}, nil
}

// CreateWordParams 新增单词入参。
type CreateWordParams struct {
	Text      string
	MeaningZh string
	Phonetic  string
	WordType  string
	Source    string
}

// CreateWord 插入一条单词。ID 由 SQLite 生成并回填。
func (s *Store) CreateWord(ctx context.Context, p CreateWordParams) (Word, error) {
	if p.WordType == "" {
		p.WordType = TypeNew
	}
	if p.Source == "" {
		p.Source = SourceManual
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Word{}, fmt.Errorf("begin create word: %w", err)
	}
	defer tx.Rollback()

	q := `INSERT INTO words (id, library_id, text, meaning_zh, phonetic, word_type, status, source)
	      VALUES (lower(hex(randomblob(8))), ?, ?, ?, ?, ?, ?, ?)`
	// 新的学习状态写入 word_learning；此处旧字段固定为未学，仅兼容历史表结构。
	res, err := tx.ExecContext(ctx, q, MainLibraryID, p.Text, p.MeaningZh, p.Phonetic, p.WordType, StatusUnlearned, p.Source)
	if err != nil {
		return Word{}, fmt.Errorf("insert word: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Word{}, fmt.Errorf("insert word: 未写入")
	}
	w, err := findByTextMeaning(ctx, tx, p.Text, p.MeaningZh)
	if err != nil {
		return Word{}, err
	}
	if p.WordType == TypeMistake {
		// 逐个录入/导入时主动选择“易错词”，即明确要求从“需强化”开始复习。
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO word_learning(word_id, library_id, learning_status, next_due_date)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(word_id) DO UPDATE SET
				learning_status=excluded.learning_status,
				next_due_date=excluded.next_due_date,
				updated_at=datetime('now')`,
			w.ID, MainLibraryID, StatusReinforce, time.Now().Format("2006-01-02")); err != nil {
			return Word{}, fmt.Errorf("mark manually entered mistake word: %w", err)
		}
	}
	w, err = getWord(ctx, tx, w.ID)
	if err != nil {
		return Word{}, err
	}
	if err := tx.Commit(); err != nil {
		return Word{}, fmt.Errorf("commit create word: %w", err)
	}
	return w, nil
}

func (s *Store) findByTextMeaning(ctx context.Context, text, meaning string) (Word, error) {
	return findByTextMeaning(ctx, s.db, text, meaning)
}

func findByTextMeaning(ctx context.Context, q rowQuerier, text, meaning string) (Word, error) {
	query := `SELECT ` + wordSelectColumns + ` FROM words w` + wordLearningJoin + `
		WHERE w.library_id=? AND w.text=? AND w.meaning_zh=? AND w.deleted_at IS NULL
		ORDER BY w.created_at DESC LIMIT 1`
	var w Word
	var lastEdited sql.NullString
	err := scanWord(q.QueryRowContext(ctx, query, MainLibraryID, text, meaning), &w, &lastEdited)
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
	return getWord(ctx, s.db, id)
}

// ListExamples 返回一个单词的例句（按展示顺序）。
func (s *Store) ListExamples(ctx context.Context, wordID string) ([]WordExample, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, word_id, sentence, display_order, created_at, updated_at
		FROM word_examples WHERE word_id=? ORDER BY display_order ASC`, wordID)
	if err != nil {
		return nil, fmt.Errorf("list word examples: %w", err)
	}
	defer rows.Close()
	examples := []WordExample{}
	for rows.Next() {
		var e WordExample
		if err := rows.Scan(&e.ID, &e.WordID, &e.Sentence, &e.DisplayOrder, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan word example: %w", err)
		}
		examples = append(examples, e)
	}
	return examples, rows.Err()
}

// RandomExample 随机取一条例句，学生端每次进入单词卡可看到不同语境。
func (s *Store) RandomExample(ctx context.Context, wordID string) (WordExample, error) {
	var e WordExample
	err := s.db.QueryRowContext(ctx, `
		SELECT id, word_id, sentence, display_order, created_at, updated_at
		FROM word_examples WHERE word_id=? ORDER BY random() LIMIT 1`, wordID).
		Scan(&e.ID, &e.WordID, &e.Sentence, &e.DisplayOrder, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return WordExample{}, err
	}
	return e, nil
}

// ReplaceExamples 用新生成的三条例句原子替换旧例句。
func (s *Store) ReplaceExamples(ctx context.Context, wordID string, sentences []string) ([]WordExample, error) {
	if len(sentences) != 3 {
		return nil, fmt.Errorf("例句数量必须为 3")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin replace examples: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM word_examples WHERE word_id=?`, wordID); err != nil {
		return nil, fmt.Errorf("delete old examples: %w", err)
	}
	for i, sentence := range sentences {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO word_examples(id, word_id, sentence, display_order)
			VALUES (lower(hex(randomblob(8))), ?, ?, ?)`, wordID, sentence, i); err != nil {
			return nil, fmt.Errorf("insert word example: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit replace examples: %w", err)
	}
	return s.ListExamples(ctx, wordID)
}

// UpdateExample 更新一条例句，供家长单句重新生成使用。
func (s *Store) UpdateExample(ctx context.Context, wordID, exampleID, sentence string) (WordExample, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE word_examples SET sentence=?, updated_at=datetime('now')
		WHERE id=? AND word_id=?`, sentence, exampleID, wordID)
	if err != nil {
		return WordExample{}, fmt.Errorf("update word example: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return WordExample{}, ErrNotFound
	}
	var e WordExample
	if err := s.db.QueryRowContext(ctx, `
		SELECT id, word_id, sentence, display_order, created_at, updated_at FROM word_examples WHERE id=?`, exampleID).
		Scan(&e.ID, &e.WordID, &e.Sentence, &e.DisplayOrder, &e.CreatedAt, &e.UpdatedAt); err != nil {
		return WordExample{}, fmt.Errorf("get updated example: %w", err)
	}
	return e, nil
}

// ListMissingExampleTargets 返回例句不足 3 条的历史单词，供家长主动补全。
func (s *Store) ListMissingExampleTargets(ctx context.Context) ([]ExampleTarget, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT w.id, w.text, w.meaning_zh
		FROM words w
		LEFT JOIN word_examples e ON e.word_id=w.id
		WHERE w.deleted_at IS NULL AND w.library_id=?
		GROUP BY w.id, w.text, w.meaning_zh
		HAVING COUNT(e.id) < 3
		ORDER BY w.created_at ASC, w.id ASC`, MainLibraryID)
	if err != nil {
		return nil, fmt.Errorf("list missing example targets: %w", err)
	}
	defer rows.Close()
	out := []ExampleTarget{}
	for rows.Next() {
		var target ExampleTarget
		if err := rows.Scan(&target.WordID, &target.Text, &target.MeaningZh); err != nil {
			return nil, fmt.Errorf("scan missing example target: %w", err)
		}
		out = append(out, target)
	}
	return out, rows.Err()
}

func getWord(ctx context.Context, q rowQuerier, id string) (Word, error) {
	query := `SELECT ` + wordSelectColumns + ` FROM words w` + wordLearningJoin + ` WHERE w.id=? AND w.deleted_at IS NULL`
	var w Word
	var lastEdited sql.NullString
	err := scanWord(q.QueryRowContext(ctx, query, id), &w, &lastEdited)
	if err != nil {
		if err == sql.ErrNoRows {
			return Word{}, ErrNotFound
		}
		return Word{}, fmt.Errorf("get word: %w", err)
	}
	w.LastEditedAt = lastEdited.String
	return w, nil
}

func scanWord(row interface{ Scan(...any) error }, w *Word, lastEdited *sql.NullString) error {
	return row.Scan(&w.ID, &w.LibraryID, &w.Text, &w.MeaningZh, &w.Phonetic,
		&w.WordType, &w.Status, &w.CreatedAt, &w.UpdatedAt, lastEdited, &w.Source)
}

// UpdateParams 更新单词属性（不含 text，PRD：英文单词创建后不可改）。
type UpdateParams struct {
	ID        string
	MeaningZh string
	Phonetic  string
	WordType  string
}

// Update 只更新词条属性；学习状态只能由学习任务或创建时的“易错词”意图写入。
func (s *Store) Update(ctx context.Context, p UpdateParams) (Word, error) {
	q := `UPDATE words
	      SET meaning_zh=?, phonetic=?, word_type=?, last_edited_at=datetime('now'), updated_at=datetime('now')
	      WHERE id=? AND deleted_at IS NULL`
	res, err := s.db.ExecContext(ctx, q, p.MeaningZh, p.Phonetic, p.WordType, p.ID)
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
