package studentwordtask

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/apodemakeles/niuniu-education/backend/internal/module/wordlibrary"
)

// MainLibraryID 复用家长端固定词库 ID。
const MainLibraryID = wordlibrary.MainLibraryID

// WordInfo 单词展示信息（从 words 表读，供任务/默写/阅读渲染）。
type WordInfo struct {
	ID        string
	Text      string
	MeaningZh string
	Phonetic  string
	WordType  string // new / mistake（展示标签）
	Status    string // 仅作为缺失学习记录时的兼容回退
	CreatedAt string // 新词公平排队使用
}

// Store 学生端任务前台数据访问层。
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// wordColumns 与 WordInfo 读取字段对齐。
const wordSelectColumns = `id, text, meaning_zh, phonetic, word_type, status, created_at`

func scanWordInfo(rows interface{ Scan(...any) error }, prefix ...string) (WordInfo, error) {
	var w WordInfo
	if err := rows.Scan(&w.ID, &w.Text, &w.MeaningZh, &w.Phonetic, &w.WordType, &w.Status, &w.CreatedAt); err != nil {
		return WordInfo{}, err
	}
	_ = prefix
	return w, nil
}

// ListAllWords 读取词库全部未软删单词（任务生成用）。
func (s *Store) ListAllWords(ctx context.Context) ([]WordInfo, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+wordSelectColumns+` FROM words WHERE deleted_at IS NULL AND library_id=? ORDER BY created_at ASC`,
		MainLibraryID)
	if err != nil {
		return nil, fmt.Errorf("list words: %w", err)
	}
	defer rows.Close()
	out := []WordInfo{}
	for rows.Next() {
		w, err := scanWordInfo(rows)
		if err != nil {
			return nil, fmt.Errorf("scan word: %w", err)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// GetWords 按 id 批量读取（保持入参顺序）。
func (s *Store) GetWords(ctx context.Context, ids []string) ([]WordInfo, error) {
	if len(ids) == 0 {
		return []WordInfo{}, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, 0, len(ids)+1)
	args = append(args, MainLibraryID)
	for _, id := range ids {
		args = append(args, id)
	}
	q := `SELECT ` + wordSelectColumns + ` FROM words WHERE deleted_at IS NULL AND library_id=? AND id IN (` + placeholders + `)`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("get words: %w", err)
	}
	defer rows.Close()
	byID := map[string]WordInfo{}
	for rows.Next() {
		w, err := scanWordInfo(rows)
		if err != nil {
			return nil, err
		}
		byID[w.ID] = w
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]WordInfo, 0, len(ids))
	for _, id := range ids {
		if w, ok := byID[id]; ok {
			out = append(out, w)
		}
	}
	return out, nil
}

// ListLearningsByStatus 按状态读取学习记录（任务生成用）。
func (s *Store) ListLearnings(ctx context.Context) ([]WordLearning, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT word_id, library_id, learning_status, success_count, studied_dates, success_dates,
		       last_studied_date, last_success_date, next_due_date, mastered_review_index, first_mastered_date, updated_at
		FROM word_learning WHERE library_id=?`, MainLibraryID)
	if err != nil {
		return nil, fmt.Errorf("list learnings: %w", err)
	}
	defer rows.Close()
	out := []WordLearning{}
	for rows.Next() {
		l, err := scanLearning(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) GetLearning(ctx context.Context, wordID string) (WordLearning, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT word_id, library_id, learning_status, success_count, studied_dates, success_dates,
		       last_studied_date, last_success_date, next_due_date, mastered_review_index, first_mastered_date, updated_at
		FROM word_learning WHERE word_id=?`, wordID)
	l, err := scanLearning(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return WordLearning{}, ErrLearningNotFound
		}
		return WordLearning{}, err
	}
	return l, nil
}

func scanLearning(r interface{ Scan(...any) error }) (WordLearning, error) {
	var l WordLearning
	var studiedDates, successDates sql.NullString
	var lastStudied, lastSuccess, nextDue, firstMastered sql.NullString
	if err := r.Scan(
		&l.WordID, &l.LibraryID, &l.LearningStatus, &l.SuccessCount,
		&studiedDates, &successDates, &lastStudied, &lastSuccess,
		&nextDue, &l.MasteredReviewIndex, &firstMastered, &l.UpdatedAt,
	); err != nil {
		return WordLearning{}, err
	}
	l.StudiedDates = decodeStringSlice(studiedDates.String)
	l.SuccessDates = decodeStringSlice(successDates.String)
	l.LastStudiedDate = lastStudied.String
	l.LastSuccessDate = lastSuccess.String
	l.NextDueDate = nextDue.String
	l.FirstMasteredDate = firstMastered.String
	return l, nil
}

// ErrLearningNotFound 学习记录不存在。
var ErrLearningNotFound = fmt.Errorf("学习记录不存在")

// UpsertLearning 插入或更新一条学习记录（全字段写入）。
func (s *Store) UpsertLearning(ctx context.Context, l WordLearning) error {
	if l.LibraryID == "" {
		l.LibraryID = MainLibraryID
	}
	if l.StudiedDates == nil {
		l.StudiedDates = []string{}
	}
	if l.SuccessDates == nil {
		l.SuccessDates = []string{}
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO word_learning(word_id, library_id, learning_status, success_count, studied_dates, success_dates,
			last_studied_date, last_success_date, next_due_date, mastered_review_index, first_mastered_date, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,datetime('now'))
		ON CONFLICT(word_id) DO UPDATE SET
			learning_status=excluded.learning_status,
			success_count=excluded.success_count,
			studied_dates=excluded.studied_dates,
			success_dates=excluded.success_dates,
			last_studied_date=excluded.last_studied_date,
			last_success_date=excluded.last_success_date,
			next_due_date=excluded.next_due_date,
			mastered_review_index=excluded.mastered_review_index,
			first_mastered_date=excluded.first_mastered_date,
			updated_at=datetime('now')`,
		l.WordID, l.LibraryID, l.LearningStatus, l.SuccessCount,
		encodeJSON(l.StudiedDates), encodeJSON(l.SuccessDates),
		l.LastStudiedDate, l.LastSuccessDate, l.NextDueDate,
		l.MasteredReviewIndex, l.FirstMasteredDate)
	if err != nil {
		return fmt.Errorf("upsert learning: %w", err)
	}
	return nil
}

// EnsureLearnings 为缺失学习记录的单词补建默认行。
// words.status 是历史兼容列，不能再反向影响实际学习状态。
func (s *Store) EnsureLearnings(ctx context.Context) (int, error) {
	// 用 INSERT ... SELECT 把 words 中存在、word_learning 中缺失的词补为未学记录。
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO word_learning(word_id, library_id, learning_status, next_due_date)
		SELECT id, library_id, 'unlearned', NULL
		FROM words
		WHERE deleted_at IS NULL
		  AND id NOT IN (SELECT word_id FROM word_learning)`)
	if err != nil {
		return 0, fmt.Errorf("ensure learnings: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// --- daily_tasks ---

// ListDailyTasks 读取某天全部任务明细（按 display_order 排序）。
func (s *Store) ListDailyTasks(ctx context.Context, date string) ([]DailyTask, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, task_date, word_id, library_id, pool_type, display_order, read_status,
		       first_dictation_answer, first_dictation_result, used_hint, submission_locked,
		       correction_answer, correction_result, created_at, updated_at
		FROM daily_tasks WHERE library_id=? AND task_date=? ORDER BY display_order ASC`, MainLibraryID, date)
	if err != nil {
		return nil, fmt.Errorf("list daily tasks: %w", err)
	}
	defer rows.Close()
	out := []DailyTask{}
	for rows.Next() {
		t, err := scanDailyTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func scanDailyTask(r interface{ Scan(...any) error }) (DailyTask, error) {
	var t DailyTask
	var usedHint, locked int
	if err := r.Scan(&t.ID, &t.TaskDate, &t.WordID, &t.LibraryID, &t.PoolType, &t.DisplayOrder,
		&t.ReadStatus, &t.FirstDictationAnswer, &t.FirstDictationResult, &usedHint, &locked,
		&t.CorrectionAnswer, &t.CorrectionResult, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return DailyTask{}, err
	}
	t.UsedHint = usedHint == 1
	t.SubmissionLocked = locked == 1
	return t, nil
}

// UpsertDailyTask 插入或更新任务明细。display_order 在插入时固定，更新时不改。
func (s *Store) UpsertDailyTask(ctx context.Context, t DailyTask) error {
	if t.ID == "" {
		return fmt.Errorf("daily task id 不能为空")
	}
	if t.LibraryID == "" {
		t.LibraryID = MainLibraryID
	}
	if t.TaskDate == "" {
		t.TaskDate = Today()
	}
	// 防御性默认值：避免空字符串违反 CHECK 约束
	if t.ReadStatus == "" {
		t.ReadStatus = ReadNotStarted
	}
	if t.FirstDictationResult == "" {
		t.FirstDictationResult = DictPending
	}
	if t.CorrectionResult == "" {
		t.CorrectionResult = CorrectNotSubmitted
	}
	locked := 0
	if t.SubmissionLocked {
		locked = 1
	}
	usedHint := 0
	if t.UsedHint {
		usedHint = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO daily_tasks(id, task_date, word_id, library_id, pool_type, display_order, read_status,
		    first_dictation_answer, first_dictation_result, used_hint, submission_locked,
		    correction_answer, correction_result, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,datetime('now'),datetime('now'))
		ON CONFLICT(id) DO UPDATE SET
			read_status=excluded.read_status,
			first_dictation_answer=excluded.first_dictation_answer,
			first_dictation_result=excluded.first_dictation_result,
			used_hint=excluded.used_hint,
			submission_locked=excluded.submission_locked,
			correction_answer=excluded.correction_answer,
			correction_result=excluded.correction_result,
			updated_at=datetime('now')`,
		t.ID, t.TaskDate, t.WordID, t.LibraryID, t.PoolType, t.DisplayOrder, t.ReadStatus,
		t.FirstDictationAnswer, t.FirstDictationResult, usedHint, locked,
		t.CorrectionAnswer, t.CorrectionResult)
	if err != nil {
		return fmt.Errorf("upsert daily task: %w", err)
	}
	return nil
}

// --- reading_passages ---

func (s *Store) GetReading(ctx context.Context, date string) (ReadingPassage, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT task_date, library_id, reading_title, reading_text, covered_word_ids, word_occurrence_counts,
		       reading_scene_hint, ai_generation_status, reading_started_at, reading_completed_at,
		       reading_min_seconds, created_at, updated_at
		FROM reading_passages WHERE task_date=?`, date)
	var p ReadingPassage
	var covered, occurrences, startedAt, completedAt sql.NullString
	if err := row.Scan(&p.TaskDate, &p.LibraryID, &p.ReadingTitle, &p.ReadingText,
		&covered, &occurrences, &p.ReadingSceneHint, &p.AIGenerationStatus,
		&startedAt, &completedAt, &p.ReadingMinSeconds, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return ReadingPassage{}, ErrReadingNotFound
		}
		return ReadingPassage{}, fmt.Errorf("get reading: %w", err)
	}
	p.CoveredWordIDs = decodeStringSlice(covered.String)
	p.WordOccurrenceCounts = decodeIntMap(occurrences.String)
	p.ReadingStartedAt = startedAt
	p.ReadingCompletedAt = completedAt
	return p, nil
}

// ErrReadingNotFound 当天阅读短文不存在。
var ErrReadingNotFound = fmt.Errorf("阅读短文不存在")

func (s *Store) UpsertReading(ctx context.Context, p ReadingPassage) error {
	if p.LibraryID == "" {
		p.LibraryID = MainLibraryID
	}
	if p.CoveredWordIDs == nil {
		p.CoveredWordIDs = []string{}
	}
	if p.WordOccurrenceCounts == nil {
		p.WordOccurrenceCounts = map[string]int{}
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO reading_passages(task_date, library_id, reading_title, reading_text, covered_word_ids,
		    word_occurrence_counts, reading_scene_hint, ai_generation_status, reading_started_at,
		    reading_completed_at, reading_min_seconds, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,datetime('now'),datetime('now'))
		ON CONFLICT(task_date) DO UPDATE SET
			library_id=excluded.library_id,
			reading_title=excluded.reading_title,
			reading_text=excluded.reading_text,
			covered_word_ids=excluded.covered_word_ids,
			word_occurrence_counts=excluded.word_occurrence_counts,
			reading_scene_hint=excluded.reading_scene_hint,
			ai_generation_status=excluded.ai_generation_status,
			reading_started_at=COALESCE(excluded.reading_started_at, reading_passages.reading_started_at),
			reading_completed_at=COALESCE(excluded.reading_completed_at, reading_passages.reading_completed_at),
			reading_min_seconds=excluded.reading_min_seconds,
			updated_at=datetime('now')`,
		p.TaskDate, p.LibraryID, p.ReadingTitle, p.ReadingText, encodeJSON(p.CoveredWordIDs),
		encodeJSON(p.WordOccurrenceCounts), p.ReadingSceneHint, p.AIGenerationStatus,
		nullable(p.ReadingStartedAt), nullable(p.ReadingCompletedAt), p.ReadingMinSeconds)
	if err != nil {
		return fmt.Errorf("upsert reading: %w", err)
	}
	return nil
}

// MarkReadingStarted 若 reading_started_at 为空则记为 now（进入阅读页开始计时）。
func (s *Store) MarkReadingStarted(ctx context.Context, date string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE reading_passages SET reading_started_at=COALESCE(reading_started_at, datetime('now')), updated_at=datetime('now')
		 WHERE task_date=? AND reading_started_at IS NULL`, date)
	return err
}

func nullable(n sql.NullString) any {
	if !n.Valid {
		return nil
	}
	return n.String
}

// --- checkins ---

func (s *Store) ListCheckins(ctx context.Context, yearMonth string) ([]Checkin, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT task_date, library_id, created_at FROM checkins WHERE library_id=? AND task_date LIKE ?`,
		MainLibraryID, yearMonth+"-%")
	if err != nil {
		return nil, fmt.Errorf("list checkins: %w", err)
	}
	defer rows.Close()
	out := []Checkin{}
	for rows.Next() {
		var c Checkin
		if err := rows.Scan(&c.TaskDate, &c.LibraryID, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListAllCheckins 读取全部打卡记录（连续天数计算用，跨月）。
func (s *Store) ListAllCheckins(ctx context.Context) ([]Checkin, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT task_date, library_id, created_at FROM checkins WHERE library_id=?`, MainLibraryID)
	if err != nil {
		return nil, fmt.Errorf("list all checkins: %w", err)
	}
	defer rows.Close()
	out := []Checkin{}
	for rows.Next() {
		var c Checkin
		if err := rows.Scan(&c.TaskDate, &c.LibraryID, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// HasCheckin 当天是否已打卡。
func (s *Store) HasCheckin(ctx context.Context, date string) (bool, error) {
	var c int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM checkins WHERE task_date=?`, date).Scan(&c)
	if err != nil {
		return false, err
	}
	return c > 0, nil
}

// InsertCheckin 打卡（幂等，INSERT OR IGNORE）。
func (s *Store) InsertCheckin(ctx context.Context, date string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO checkins(task_date, library_id) VALUES (?, ?)`, date, MainLibraryID)
	return err
}

// --- strategy ---

func (s *Store) GetStrategy(ctx context.Context) (Strategy, error) {
	var st Strategy
	var id int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, new_pool_count, review_pool_max_count, review_pool_min_score, reinforce_base_score,
		       learning_base_score, mastered_base_score, due_today_bonus, overdue_daily_bonus, overdue_bonus_cap,
		       mastered_due_sample_ratio, mastered_sample_stop_day, review_overdue_reduce_new,
		       review_overdue_pause_new, reading_min_seconds
		FROM review_strategy WHERE id=1`).Scan(
		&id, &st.NewPoolCount, &st.ReviewPoolMaxCount, &st.ReviewPoolMinScore, &st.ReinforceBaseScore,
		&st.LearningBaseScore, &st.MasteredBaseScore, &st.DueTodayBonus, &st.OverdueDailyBonus, &st.OverdueBonusCap,
		&st.MasteredDueSampleRatio, &st.MasteredSampleStopDay, &st.ReviewOverdueReduceNew,
		&st.ReviewOverduePauseNew, &st.ReadingMinSeconds)
	if err != nil {
		if err == sql.ErrNoRows {
			return DefaultStrategy(), nil
		}
		return Strategy{}, fmt.Errorf("get strategy: %w", err)
	}
	return st, nil
}

// ResetAll 物理清空学生端数据（仅测试用，经专用测试端点）。
func (s *Store) ResetAll(ctx context.Context) error {
	for _, t := range []string{"daily_tasks", "reading_passages", "checkins"} {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM `+t); err != nil {
			return err
		}
	}
	// word_learning 与 words 一并清掉，便于测试重置后再 EnsureLearnings。
	for _, t := range []string{"word_learning"} {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM `+t); err != nil {
			return err
		}
	}
	return nil
}
