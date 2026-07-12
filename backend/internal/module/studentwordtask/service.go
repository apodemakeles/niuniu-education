package studentwordtask

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/apodemakeles/niuniu-education/backend/internal/platform/llm"
)

// Service 学生端任务前台编排层：串联 store + scheduler + review + reading。
type Service struct {
	store       *Store
	llm         llm.Provider
	logger      *slog.Logger
	grade       string // 孩子年级/难度提示（透传给 LLM）
	readingMu   sync.Mutex
	readingJobs map[string]bool
}

func NewService(store *Store, llmProvider llm.Provider, grade string, logger *slog.Logger) *Service {
	return &Service{store: store, llm: llmProvider, grade: grade, logger: logger, readingJobs: map[string]bool{}}
}

// --- 任务生成（幂等：当日已有 daily_tasks 则直接复用） ---

// generateDailyTasks 保证当天 daily_tasks 存在。返回当天任务明细列表。
// 若已存在则直接返回；否则按 scheduler 生成并写库。
func (s *Service) generateDailyTasks(ctx context.Context, date string) ([]DailyTask, error) {
	existing, err := s.store.ListDailyTasks(ctx, date)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return existing, nil
	}

	// 确保所有词都有学习记录（缺失补建）
	if _, err := s.store.EnsureLearnings(ctx); err != nil {
		return nil, fmt.Errorf("ensure learnings: %w", err)
	}

	words, err := s.store.ListAllWords(ctx)
	if err != nil {
		return nil, err
	}
	learnings, err := s.store.ListLearnings(ctx)
	if err != nil {
		return nil, err
	}
	strategy, err := s.store.GetStrategy(ctx)
	if err != nil {
		return nil, err
	}

	// 任务生成使用确定性随机源（按日期种子），保证同一天多次生成结果一致，
	// 避免刷新首页时新词池/抽查结果抖动。
	rng := newSeededRand(date)
	plan := BuildTodayPlan(date, "", strategy, words, learnings, rng.nextIntN)

	firstOrders, reviewOrders := AllocateDisplayOrder(plan.FirstPool, plan.ReviewPool)

	// 写入 daily_tasks
	now := time.Now().UTC().Format(time.RFC3339)
	tasks := make([]DailyTask, 0, len(plan.FirstPool)+len(plan.ReviewPool))
	for _, c := range plan.FirstPool {
		tasks = append(tasks, DailyTask{
			ID:           dailyTaskID(date, c.Word.ID),
			TaskDate:     date,
			WordID:       c.Word.ID,
			PoolType:     PoolFirst,
			DisplayOrder: firstOrders[c.Word.ID],
			ReadStatus:   ReadNotStarted,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}
	for _, c := range plan.ReviewPool {
		tasks = append(tasks, DailyTask{
			ID:           dailyTaskID(date, c.Word.ID),
			TaskDate:     date,
			WordID:       c.Word.ID,
			PoolType:     PoolReview,
			DisplayOrder: reviewOrders[c.Word.ID],
			ReadStatus:   ReadNotStarted,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}
	for _, t := range tasks {
		if err := s.store.UpsertDailyTask(ctx, t); err != nil {
			return nil, err
		}
	}
	return tasks, nil
}

// dailyTaskID 固定 id 格式：task_date + word_id，保证幂等。
func dailyTaskID(date, wordID string) string {
	return fmt.Sprintf("dt-%s-%s", date, wordID)
}

// --- 首页 ---

// GetToday 首页数据。
func (s *Service) GetToday(ctx context.Context) (TodayResponse, error) {
	date := Today()
	resp := TodayResponse{
		Date:          date,
		FirstPool:     make([]MissionWord, 0),
		ReviewPool:    make([]MissionWord, 0),
		NewPoolCount:  DefaultStrategy().NewPoolCount,
		ReviewPoolMax: DefaultStrategy().ReviewPoolMaxCount,
	}

	words, err := s.store.ListAllWords(ctx)
	if err != nil {
		return resp, err
	}
	if len(words) == 0 {
		resp.Empty = true
		resp.EmptyHint = "词库还是空的，先请家长在单词库添加单词吧。"
		return resp, nil
	}

	tasks, err := s.generateDailyTasks(ctx, date)
	if err != nil {
		return resp, err
	}
	resp.Completed = isDailyTaskCompleted(tasks)
	// 今日任务一旦存在就异步准备短文，不阻塞首页；同一进程每天只启动一个任务。
	s.ensureReadingAsync(date)
	learnings, _ := s.store.ListLearnings(ctx)
	learningByID := map[string]WordLearning{}
	for _, l := range learnings {
		learningByID[l.WordID] = l
	}
	wordByID := map[string]WordInfo{}
	for _, w := range words {
		wordByID[w.ID] = w
	}
	strategy, _ := s.store.GetStrategy(ctx)
	resp.NewPoolCount = strategy.NewPoolCount
	resp.ReviewPoolMax = strategy.ReviewPoolMaxCount

	for _, t := range tasks {
		w, ok := wordByID[t.WordID]
		if !ok {
			continue
		}
		mw := toMissionWord(w, t, learningByID[t.WordID])
		if t.PoolType == PoolFirst {
			resp.FirstPool = append(resp.FirstPool, mw)
		} else {
			resp.ReviewPool = append(resp.ReviewPool, mw)
		}
	}

	// 负载日类型：用 plan 算一次（不写库，仅展示）
	plan := BuildTodayPlan(date, "", strategy, words, learnings, newSeededRand(date).nextIntN)
	resp.ReviewPoolMax = plan.ReviewPoolCapacity
	resp.LoadDay = plan.LoadDay
	resp.LoadDayLabel, resp.LoadDayDesc = loadDayText(plan.LoadDay)
	resp.TotalCount = len(resp.FirstPool) + len(resp.ReviewPool)
	return resp, nil
}

// isDailyTaskCompleted 只有首次默写已锁定，且每个错词都已订正正确时才算完成。
func isDailyTaskCompleted(tasks []DailyTask) bool {
	if len(tasks) == 0 {
		return false
	}
	for _, task := range tasks {
		if !task.SubmissionLocked {
			return false
		}
		if (task.FirstDictationResult == DictWrong || task.FirstDictationResult == DictBlank) && task.CorrectionResult != CorrectCorrect {
			return false
		}
	}
	return true
}

// --- 单词卡 ---

// EnterCard 首次进入单词卡：未学→学习中（幂等）。
func (s *Service) EnterCard(ctx context.Context, wordID string) (CardDetailResponse, error) {
	date := Today()
	tasks, err := s.generateDailyTasks(ctx, date)
	if err != nil {
		return CardDetailResponse{}, err
	}
	// 更新学习记录：未学→学习中
	l, err := s.store.GetLearning(ctx, wordID)
	if err != nil {
		if err == ErrLearningNotFound {
			if _, e := s.store.EnsureLearnings(ctx); e != nil {
				return CardDetailResponse{}, e
			}
			l, err = s.store.GetLearning(ctx, wordID)
			if err != nil {
				return CardDetailResponse{}, err
			}
		} else {
			return CardDetailResponse{}, err
		}
	}
	updated := EnterCard(l, date)
	if updated.LearningStatus != l.LearningStatus || updated.LastStudiedDate != l.LastStudiedDate {
		if err := s.store.UpsertLearning(ctx, updated); err != nil {
			return CardDetailResponse{}, err
		}
	}
	return s.buildCardDetail(ctx, tasks, wordID)
}

// MarkListened 标记已听音。
func (s *Service) MarkListened(ctx context.Context, wordID string) (CardDetailResponse, error) {
	return s.updateCardReadStatus(ctx, wordID, func(cur string) string {
		if cur == ReadNotStarted {
			return ReadListened
		}
		return cur // 已读完/不会读不被覆盖
	})
}

// MarkReadDone 标记我读完了。
func (s *Service) MarkReadDone(ctx context.Context, wordID string) (CardDetailResponse, error) {
	return s.updateCardReadStatus(ctx, wordID, func(cur string) string {
		return ReadDone
	})
}

// MarkHard 单词卡阶段「不会读」（首次默写提交前有效）。
func (s *Service) MarkHard(ctx context.Context, wordID string) (CardDetailResponse, error) {
	date := Today()
	tasks, err := s.generateDailyTasks(ctx, date)
	if err != nil {
		return CardDetailResponse{}, err
	}
	// 找到该词的 daily_task，若已锁定（首次默写已提交）则拒绝
	for i, t := range tasks {
		if t.WordID == wordID && t.SubmissionLocked {
			return CardDetailResponse{}, ErrAlreadyLocked
		}
		_ = i
	}
	// 更新学习记录：需强化、清零、明天到期
	l, err := s.store.GetLearning(ctx, wordID)
	if err != nil {
		return CardDetailResponse{}, err
	}
	updated := MarkHardBeforeSubmit(l, date)
	if err := s.store.UpsertLearning(ctx, updated); err != nil {
		return CardDetailResponse{}, err
	}
	// 更新 daily_task read_status = hard
	return s.updateCardReadStatus(ctx, wordID, func(cur string) string { return ReadHard })
}

// ErrAlreadyLocked 首次默写已提交锁定，不能再改。
var ErrAlreadyLocked = fmt.Errorf("首次默写已提交，结果已锁定")

func (s *Service) updateCardReadStatus(ctx context.Context, wordID string, mutate func(cur string) string) (CardDetailResponse, error) {
	date := Today()
	tasks, err := s.generateDailyTasks(ctx, date)
	if err != nil {
		return CardDetailResponse{}, err
	}
	for i, t := range tasks {
		if t.WordID == wordID {
			t.ReadStatus = mutate(t.ReadStatus)
			if err := s.store.UpsertDailyTask(ctx, t); err != nil {
				return CardDetailResponse{}, err
			}
			tasks[i] = t
			break
		}
	}
	return s.buildCardDetail(ctx, tasks, wordID)
}

func (s *Service) buildCardDetail(ctx context.Context, tasks []DailyTask, wordID string) (CardDetailResponse, error) {
	idx := -1
	for i, t := range tasks {
		if t.WordID == wordID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return CardDetailResponse{}, ErrNotInTodayTask
	}
	t := tasks[idx]
	ws, err := s.store.GetWords(ctx, []string{t.WordID})
	if err != nil || len(ws) == 0 {
		return CardDetailResponse{}, fmt.Errorf("读取单词失败")
	}
	learnings, _ := s.store.ListLearnings(ctx)
	learningByID := map[string]WordLearning{}
	for _, l := range learnings {
		learningByID[l.WordID] = l
	}
	mw := toMissionWord(ws[0], t, learningByID[t.WordID])
	return CardDetailResponse{
		Word:           mw,
		CardIndex:      idx,
		Total:          len(tasks),
		Example:        systemExample(ws[0].Text),
		ExampleMissing: true,
		Listened:       t.ReadStatus == ReadListened || t.ReadStatus == ReadDone,
		ReadDone:       t.ReadStatus == ReadDone,
		IsFirstCard:    idx == 0,
		IsLastCard:     idx == len(tasks)-1,
	}, nil
}

// ErrNotInTodayTask 该词不在今日任务中。
var ErrNotInTodayTask = fmt.Errorf("该词不在今日任务中")

// --- 阅读短文 ---

// GetReading 取今日短文；无则触发生成（调 LLM + 校验 + 重试）。
func (s *Service) GetReading(ctx context.Context) (ReadingResponse, error) {
	date := Today()
	if _, err := s.generateDailyTasks(ctx, date); err != nil {
		return ReadingResponse{}, err
	}
	p, err := s.store.GetReading(ctx, date)
	if err == nil && p.AIGenerationStatus == ReadingSuccess && p.ReadingText != "" && s.passageValid(p) {
		if p.ReadingMinSeconds <= 0 {
			p.ReadingMinSeconds = s.readingMinSeconds()
			if err := s.store.UpsertReading(ctx, p); err != nil {
				return ReadingResponse{}, err
			}
		}
		if err := s.store.MarkReadingStarted(ctx, date); err != nil {
			return ReadingResponse{}, err
		}
		p, _ = s.store.GetReading(ctx, date)
		return s.buildReadingResponse(p), nil
	}
	if err == nil && p.AIGenerationStatus == ReadingPending {
		// pending 需判断是否真有后台任务在跑，避免进程重启/goroutine 异常退出后僵尸 pending 永久卡住前端。
		if s.isReadingRunning(date) {
			return s.buildReadingResponse(p), nil
		}
		if stale := pendingStaleMinutes(p); stale >= readingPendingStaleMinutes {
			s.logger.Warn("reading pending is stale, regenerate synchronously",
				slog.String("date", date), slog.Int("stale_minutes", stale))
			// 落到下方的生成/重新生成路径
		} else {
			// 刚启动还没占锁，或仍在 5 分钟窗口内，正常返回 pending
			return s.buildReadingResponse(p), nil
		}
	}
	// 生成或重新生成
	np, gerr := s.generateReading(ctx, date)
	if gerr != nil {
		// 返回失败态，允许前端重试
		s.logger.Warn("generate reading", slog.Any("err", gerr))
		return ReadingResponse{Date: date, Status: ReadingFailed}, nil
	}
	return s.buildReadingResponse(np), nil
}

// ensureReadingAsync 在创建今日任务后后台生成短文。失败状态保留给阅读页手动重试。
func (s *Service) ensureReadingAsync(date string) {
	if p, err := s.store.GetReading(context.Background(), date); err == nil &&
		p.AIGenerationStatus == ReadingSuccess && s.passageValid(p) {
		return
	}
	s.readingMu.Lock()
	if s.readingJobs[date] {
		s.readingMu.Unlock()
		return
	}
	s.readingJobs[date] = true
	s.readingMu.Unlock()
	_ = s.store.UpsertReading(context.Background(), ReadingPassage{TaskDate: date, AIGenerationStatus: ReadingPending, ReadingMinSeconds: s.readingMinSeconds()})
	s.logger.Info("async reading generation started", slog.String("date", date))
	go func() {
		start := time.Now()
		status := ReadingFailed
		defer func() {
			s.readingMu.Lock()
			delete(s.readingJobs, date)
			s.readingMu.Unlock()
			s.logger.Info("async reading generation finished",
				slog.String("date", date), slog.String("status", status),
				slog.Duration("dur", time.Since(start)))
		}()
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("async reading generation panic",
					slog.String("date", date), slog.Any("panic", r))
				// 把僵尸 pending 改写为 failed，避免前端无限转圈
				_ = s.store.UpsertReading(context.Background(), ReadingPassage{
					TaskDate:           date,
					AIGenerationStatus: ReadingFailed,
				})
			}
		}()
		p, err := s.generateReading(context.Background(), date)
		if err != nil {
			s.logger.Warn("async reading generation failed", slog.String("date", date), slog.Any("err", err))
			return
		}
		status = p.AIGenerationStatus
	}()
}

// readingPendingStaleMinutes pending 状态超过该分钟数且无后台任务在跑时，视为僵尸 pending。
const readingPendingStaleMinutes = 5

// isReadingRunning 检查指定日期是否有后台短文生成任务在跑。
func (s *Service) isReadingRunning(date string) bool {
	s.readingMu.Lock()
	defer s.readingMu.Unlock()
	return s.readingJobs[date]
}

// pendingStaleMinutes 返回 reading 的 updated_at 距今的分钟数（解析失败返回 0，按未过期处理）。
// updated_at 由 SQLite datetime('now') 写入，返回 UTC 时间的 "YYYY-MM-DD HH:MM:SS" 格式。
func pendingStaleMinutes(p ReadingPassage) int {
	t, ok := parseDBTime(p.UpdatedAt)
	if !ok {
		return 0
	}
	return int(time.Since(t).Minutes())
}

// parseDBTime 解析数据库里的时间字符串。
// SQLite datetime('now') 输出 UTC 时间的 "YYYY-MM-DD HH:MM:SS" 格式；
// 部分字段（如 completed_at）由 Go 代码写入 RFC3339 格式。两种都要兼容。
func parseDBTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	// 优先按 SQLite datetime('now') 格式解析（UTC）
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.UTC); err == nil {
		return t, true
	}
	// 兜底 RFC3339
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func (s *Service) readingMinSeconds() int {
	strategy, err := s.store.GetStrategy(context.Background())
	if err == nil && strategy.ReadingMinSeconds > 0 {
		return strategy.ReadingMinSeconds
	}
	return 120
}

func (s *Service) passageValid(p ReadingPassage) bool {
	if p.ReadingText == "" || len(p.CoveredWordIDs) == 0 {
		return false
	}
	return ValidatePassage(p.ReadingText, s.coveredFromIDs(p.CoveredWordIDs)) == ""
}

// RegenerateReading 手动重新生成（兜底）。
func (s *Service) RegenerateReading(ctx context.Context) (ReadingResponse, error) {
	date := Today()
	s.logger.Info("manual regenerate reading", slog.String("date", date))
	if _, err := s.generateDailyTasks(ctx, date); err != nil {
		return ReadingResponse{}, err
	}
	np, err := s.generateReading(ctx, date)
	if err != nil {
		return ReadingResponse{Date: date, Status: ReadingFailed}, nil
	}
	return s.buildReadingResponse(np), nil
}

// generateReading 生成短文：选覆盖词 → 调 LLM → 校验 → 重试最多 2 次 → 写库。
func (s *Service) generateReading(ctx context.Context, date string) (ReadingPassage, error) {
	start := time.Now()
	tasks, err := s.store.ListDailyTasks(ctx, date)
	if err != nil {
		return ReadingPassage{}, err
	}
	words, err := s.store.ListAllWords(ctx)
	if err != nil {
		return ReadingPassage{}, err
	}
	learnings, _ := s.store.ListLearnings(ctx)
	learningByID := map[string]WordLearning{}
	for _, l := range learnings {
		learningByID[l.WordID] = l
	}
	wordByID := map[string]WordInfo{}
	for _, w := range words {
		wordByID[w.ID] = w
	}

	// 组装新词池/非新词池候选（用于覆盖词选择）
	var firstPool, reviewPool []CandidateWord
	for _, t := range tasks {
		w, ok := wordByID[t.WordID]
		if !ok {
			continue
		}
		c := CandidateWord{Word: w, Learning: learningByID[t.WordID]}
		if t.PoolType == PoolFirst {
			firstPool = append(firstPool, c)
		} else {
			reviewPool = append(reviewPool, c)
		}
	}
	covered := SelectCoveredWords(firstPool, reviewPool, learningByID)
	if len(covered) == 0 {
		// 今日没有任务词，无法生成
		p := ReadingPassage{TaskDate: date, AIGenerationStatus: ReadingFailed}
		_ = s.store.UpsertReading(ctx, p)
		s.logger.Info("reading generation failed",
			slog.String("date", date), slog.String("final_err", "今日没有可覆盖的任务词"),
			slog.Duration("dur", time.Since(start)))
		return p, fmt.Errorf("今日没有可覆盖的任务词")
	}

	// 调 LLM，最多重试 readingMaxRetry 次
	var title, text, scene string
	var lastErr error
	for attempt := 0; attempt <= readingMaxRetry; attempt++ {
		s.logger.Info("reading generate attempt",
			slog.String("date", date), slog.Int("attempt", attempt+1), slog.Int("covered_count", len(covered)))
		t, x, sc, err := GeneratePassage(ctx, s.llm, covered, s.grade)
		if err != nil {
			s.logger.Warn("reading generate attempt failed",
				slog.Int("attempt", attempt+1), slog.Any("err", err))
			lastErr = err
			continue
		}
		wc := CountEnglishWords(x)
		if reason := ValidatePassage(x, covered); reason != "" {
			s.logger.Warn("reading validate failed",
				slog.Int("attempt", attempt+1), slog.Int("words", wc), slog.String("reason", reason))
			lastErr = fmt.Errorf("校验失败: %s", reason)
			continue
		}
		title, text, scene = t, x, sc
		lastErr = nil
		break
	}
	if lastErr != nil {
		p := ReadingPassage{TaskDate: date, AIGenerationStatus: ReadingFailed}
		_ = s.store.UpsertReading(ctx, p)
		s.logger.Info("reading generation failed",
			slog.String("date", date), slog.String("final_err", lastErr.Error()),
			slog.Duration("dur", time.Since(start)))
		return p, lastErr
	}

	_, counts := HighlightText(text, covered)
	coveredIDs := make([]string, 0, len(covered))
	for _, c := range covered {
		coveredIDs = append(coveredIDs, c.Word.ID)
	}
	p := ReadingPassage{
		TaskDate:             date,
		ReadingTitle:         title,
		ReadingText:          text,
		CoveredWordIDs:       coveredIDs,
		WordOccurrenceCounts: counts,
		ReadingSceneHint:     scene,
		AIGenerationStatus:   ReadingSuccess,
		ReadingMinSeconds:    s.readingMinSeconds(),
	}
	if err := s.store.UpsertReading(ctx, p); err != nil {
		return p, err
	}
	s.logger.Info("reading generation success",
		slog.String("date", date), slog.String("title", title),
		slog.Int("word_count", CountEnglishWords(text)), slog.Int("covered_count", len(covered)),
		slog.Duration("dur", time.Since(start)))
	return p, nil
}

func (s *Service) buildReadingResponse(p ReadingPassage) ReadingResponse {
	resp := ReadingResponse{
		Date:       p.TaskDate,
		Title:      p.ReadingTitle,
		Text:       p.ReadingText,
		RawText:    p.ReadingText,
		SceneHint:  p.ReadingSceneHint,
		Status:     p.AIGenerationStatus,
		MinSeconds: p.ReadingMinSeconds,
	}
	if p.ReadingStartedAt.Valid {
		resp.StartedAt = p.ReadingStartedAt.String
		if t, ok := parseDBTime(p.ReadingStartedAt.String); ok {
			resp.ElapsedSeconds = int(time.Since(t).Seconds())
			if resp.ElapsedSeconds < 0 {
				resp.ElapsedSeconds = 0
			}
		}
	}
	if p.ReadingCompletedAt.Valid {
		resp.CompletedAt = p.ReadingCompletedAt.String
	}
	resp.CanFinish = resp.ElapsedSeconds >= resp.MinSeconds || p.ReadingCompletedAt.Valid

	if p.AIGenerationStatus == ReadingSuccess {
		// 渲染高亮 HTML（用 DB 存的覆盖词 id 反查单词）
		covered := s.coveredFromIDs(p.CoveredWordIDs)
		htmlText, _ := HighlightText(p.ReadingText, covered)
		resp.Text = htmlText
		// 出现次数直接用 DB 存的（生成时已统计，避免重复算）
		byID := map[string]WordInfo{}
		for _, c := range covered {
			byID[c.Word.ID] = c.Word
			resp.CoveredWords = append(resp.CoveredWords, CoveredWord{
				ID: c.Word.ID, Text: c.Word.Text, MeaningZh: c.Word.MeaningZh,
			})
		}
		for id, cnt := range p.WordOccurrenceCounts {
			text := ""
			if w, ok := byID[id]; ok {
				text = w.Text
			}
			resp.Appearances = append(resp.Appearances, Appearance{ID: id, Text: text, Count: cnt})
		}
	}
	return resp
}

// coveredFromIDs 从 id 列表查词（用于 highlight）。查不到的忽略。
func (s *Service) coveredFromIDs(ids []string) []CandidateWord {
	out := make([]CandidateWord, 0, len(ids))
	// 注意：buildReadingResponse 在 GetReading 中调用，此时已读过 words，这里重读一次保证简洁。
	ws, _ := s.store.ListAllWords(context.Background())
	byID := map[string]WordInfo{}
	for _, w := range ws {
		byID[w.ID] = w
	}
	for _, id := range ids {
		if w, ok := byID[id]; ok {
			out = append(out, CandidateWord{Word: w})
		}
	}
	return out
}

// MarkReadingStarted 进入阅读页开始计时（幂等）。
func (s *Service) MarkReadingStarted(ctx context.Context) error {
	date := Today()
	// 确保短文存在
	if _, err := s.store.GetReading(ctx, date); err == ErrReadingNotFound {
		if _, e := s.GetReading(ctx); e != nil {
			return e
		}
	}
	return s.store.MarkReadingStarted(ctx, date)
}

// CompleteReading 完成阅读：校验停留≥minSeconds，写 completed_at。
func (s *Service) CompleteReading(ctx context.Context) error {
	date := Today()
	p, err := s.store.GetReading(ctx, date)
	if err != nil {
		return err
	}
	if p.AIGenerationStatus != ReadingSuccess {
		return ErrReadingNotReady
	}
	if p.ReadingCompletedAt.Valid {
		return nil // 已完成，幂等
	}
	if !p.ReadingStartedAt.Valid {
		return ErrReadingNotStarted
	}
	start, ok := parseDBTime(p.ReadingStartedAt.String)
	if !ok {
		return ErrReadingNotStarted
	}
	if int(time.Since(start).Seconds()) < p.ReadingMinSeconds {
		return ErrReadingTooShort
	}
	p.ReadingCompletedAt.Valid = true
	p.ReadingCompletedAt.String = time.Now().UTC().Format(time.RFC3339)
	return s.store.UpsertReading(ctx, p)
}

// ErrReadingNotReady 短文未就绪（未生成成功）。
var ErrReadingNotReady = fmt.Errorf("阅读短文未就绪")

// ErrReadingNotStarted 未开始计时。
var ErrReadingNotStarted = fmt.Errorf("请先开始阅读")

// ErrReadingTooShort 阅读停留时间不足。
var ErrReadingTooShort = fmt.Errorf("继续阅读一会儿再完成")

// --- 默写 ---

// GetDictation 取默写题面。
func (s *Service) GetDictation(ctx context.Context) (DictationResponse, error) {
	date := Today()
	tasks, err := s.generateDailyTasks(ctx, date)
	if err != nil {
		return DictationResponse{}, err
	}
	ws, err := s.store.GetWords(ctx, taskWordIDs(tasks))
	if err != nil {
		return DictationResponse{}, err
	}
	wordByID := map[string]WordInfo{}
	for _, w := range ws {
		wordByID[w.ID] = w
	}
	learnings, _ := s.store.ListLearnings(ctx)
	learningByID := map[string]WordLearning{}
	for _, l := range learnings {
		learningByID[l.WordID] = l
	}

	items := make([]DictationItem, 0, len(tasks))
	for _, t := range tasks {
		w, ok := wordByID[t.WordID]
		if !ok {
			continue
		}
		items = append(items, DictationItem{
			TaskID:           t.ID,
			WordID:           t.WordID,
			MeaningZh:        w.MeaningZh,
			Phonetic:         w.Phonetic,
			PoolType:         t.PoolType,
			FirstAnswer:      t.FirstDictationAnswer,
			FirstResult:      t.FirstDictationResult,
			Locked:           t.SubmissionLocked,
			CorrectionAnswer: t.CorrectionAnswer,
			CorrectionResult: t.CorrectionResult,
			BonusEligible:    isBonusEligible(t, learningByID[t.WordID]),
			BonusAwarded:     t.SubmissionLocked && t.FirstDictationResult == DictCorrect && isBonusEligible(t, learningByID[t.WordID]),
		})
	}
	return DictationResponse{Date: date, Items: items}, nil
}

// isBonusEligible 新词池未学词，首次正确可触发 bonus。
// PRD：bonus 只在新学单词首次提交默写且拼写正确时出现。
// 「新学单词」= 新词池 + 学习状态原为未学（首次进入学习）。
func isBonusEligible(t DailyTask, l WordLearning) bool {
	if t.PoolType != PoolFirst {
		return false
	}
	// 学期首日首次学习的词：learning_status 在提交时已从 unlearned 变 learning，
	// 用 first_dictation_result==correct 且 success_count==1 兜底判定。
	// 这里仅返回「是否符合新词池」条件，最终 bonus 由 submit 时按结果判定。
	return true
}

// SubmitDictation 首次默写提交：判定每词 → 锁定 → 跑 review 状态流转 → 更新学习记录。
// 已锁定的词跳过（幂等，避免重复处理）。
func (s *Service) SubmitDictation(ctx context.Context, req SubmitDictationRequest) (SubmitDictationResponse, error) {
	date := Today()
	tasks, err := s.generateDailyTasks(ctx, date)
	if err != nil {
		return SubmitDictationResponse{}, err
	}
	ws, err := s.store.GetWords(ctx, taskWordIDs(tasks))
	if err != nil {
		return SubmitDictationResponse{}, err
	}
	wordByID := map[string]WordInfo{}
	for _, w := range ws {
		wordByID[w.ID] = w
	}
	answerByTaskID := map[string]string{}
	for _, a := range req.Answers {
		answerByTaskID[a.TaskID] = a.Answer
	}

	learnings, _ := s.store.ListLearnings(ctx)
	learningByID := map[string]WordLearning{}
	for _, l := range learnings {
		learningByID[l.WordID] = l
	}

	resp := SubmitDictationResponse{}
	updatedTasks := make([]DailyTask, 0, len(tasks))
	for _, t := range tasks {
		if t.SubmissionLocked {
			// 已锁定：保留原结果，仅汇总
			appendSubmitResult(&resp, t)
			updatedTasks = append(updatedTasks, t)
			continue
		}
		w, ok := wordByID[t.WordID]
		if !ok {
			updatedTasks = append(updatedTasks, t)
			continue
		}
		answer := answerByTaskID[t.ID]
		result := JudgeAnswer(answer, w.Text)
		t.FirstDictationAnswer = answer
		t.FirstDictationResult = result
		t.SubmissionLocked = true
		if t.ReadStatus == ReadHard {
			t.UsedHint = true
		}

		// 跑状态流转（首次提交）
		l := learningByID[t.WordID]
		oc := DictationOutcome{
			WordID:   t.WordID,
			Correct:  result == DictCorrect,
			Blank:    result == DictBlank,
			PoolType: t.PoolType,
		}
		newL := ApplyDictation(l, oc, date, t.ReadStatus == ReadHard)
		if err := s.store.UpsertLearning(ctx, newL); err != nil {
			return resp, err
		}
		learningByID[t.WordID] = newL

		// bonus：新词池未学词首次学习且首次默写正确。
		// 「首次学习」判定：提交前 success_count==0 且今天之前未学过（studied_dates 不含历史日期）。
		// 新词池只放未学词，复习词不会进新词池，故 pool=first + success_count==0 即等价新学词。
		if result == DictCorrect && t.PoolType == PoolFirst && l.SuccessCount == 0 && len(l.StudiedDates) <= 1 {
			resp.BonusIDs = append(resp.BonusIDs, t.WordID)
		}
		appendSubmitResult(&resp, t)
		if err := s.store.UpsertDailyTask(ctx, t); err != nil {
			return resp, err
		}
		updatedTasks = append(updatedTasks, t)
	}
	return resp, nil
}

func appendSubmitResult(resp *SubmitDictationResponse, t DailyTask) {
	switch t.FirstDictationResult {
	case DictCorrect:
		resp.Correct++
	case DictWrong:
		resp.Wrong++
	case DictBlank:
		resp.Blank++
	}
}

// CorrectDictation 订正提交：只写 correction_*，不改首次结果与 success_count。
func (s *Service) CorrectDictation(ctx context.Context, req CorrectDictationRequest) (SubmitDictationResponse, error) {
	date := Today()
	tasks, err := s.store.ListDailyTasks(ctx, date)
	if err != nil {
		return SubmitDictationResponse{}, err
	}
	ws, err := s.store.GetWords(ctx, taskWordIDs(tasks))
	if err != nil {
		return SubmitDictationResponse{}, err
	}
	wordByID := map[string]WordInfo{}
	for _, w := range ws {
		wordByID[w.ID] = w
	}
	answerByTaskID := map[string]string{}
	for _, a := range req.Answers {
		answerByTaskID[a.TaskID] = a.Answer
	}

	resp := SubmitDictationResponse{}
	for _, t := range tasks {
		if !t.SubmissionLocked {
			continue // 未提交首次，不能订正
		}
		if t.FirstDictationResult != DictWrong && t.FirstDictationResult != DictBlank {
			continue // 仅错词/未填写词开放订正
		}
		answer, submitted := answerByTaskID[t.ID]
		if !submitted {
			continue // 本轮未提交的词保留已有订正结果，不能被当作空答案覆盖
		}
		w, ok := wordByID[t.WordID]
		if !ok {
			continue
		}
		result := JudgeAnswer(answer, w.Text)
		t.CorrectionAnswer = answer
		switch result {
		case DictCorrect:
			t.CorrectionResult = CorrectCorrect
		case DictWrong:
			t.CorrectionResult = CorrectWrong
		case DictBlank:
			t.CorrectionResult = CorrectBlank
		}
		if err := s.store.UpsertDailyTask(ctx, t); err != nil {
			return resp, err
		}
	}
	// 返回最新题面汇总
	full, err := s.GetDictation(ctx)
	if err != nil {
		return resp, err
	}
	resp.Items = full.Items
	return resp, nil
}

// --- 完成页 ---

func (s *Service) GetDone(ctx context.Context) (DoneResponse, error) {
	date := Today()
	tasks, err := s.store.ListDailyTasks(ctx, date)
	if err != nil {
		return DoneResponse{}, err
	}
	ws, err := s.store.GetWords(ctx, taskWordIDs(tasks))
	if err != nil {
		return DoneResponse{}, err
	}
	wordByID := map[string]WordInfo{}
	for _, w := range ws {
		wordByID[w.ID] = w
	}
	learnings, _ := s.store.ListLearnings(ctx)
	learningByID := map[string]WordLearning{}
	for _, l := range learnings {
		learningByID[l.WordID] = l
	}

	resp := DoneResponse{Date: date, Total: len(tasks)}
	hasCorrection := false
	var tomorrow, reinforce []MissionWord
	for _, t := range tasks {
		w, ok := wordByID[t.WordID]
		if !ok {
			continue
		}
		mw := toMissionWord(w, t, learningByID[t.WordID])
		switch t.FirstDictationResult {
		case DictCorrect:
			resp.CorrectCount++
		case DictWrong, DictBlank:
			resp.WrongCount++
			reinforce = append(reinforce, mw)
			tomorrow = append(tomorrow, mw)
		default:
			tomorrow = append(tomorrow, mw)
		}
		if t.UsedHint {
			resp.UsedHintCount++
		}
		if t.CorrectionResult != CorrectNotSubmitted {
			hasCorrection = true
		}
		resp.DictationLocked = resp.DictationLocked || t.SubmissionLocked
	}
	if resp.WrongCount > 0 && len(tomorrow) == 0 {
		tomorrow = reinforce
	}
	resp.TomorrowReview = tomorrow
	resp.NeedReinforce = reinforce
	resp.HasCorrection = hasCorrection

	// 打卡概要
	resp.Checkin = s.buildCheckinSummary(ctx, date, false)
	return resp, nil
}

// --- 打卡 ---

// Checkin 打卡（幂等）。若形成连续 7 天，返回 bonus7Day=true。
func (s *Service) Checkin(ctx context.Context) (CheckinResponse, error) {
	date := Today()
	had, err := s.store.HasCheckin(ctx, date)
	if err != nil {
		return CheckinResponse{}, err
	}
	if err := s.store.InsertCheckin(ctx, date); err != nil {
		return CheckinResponse{}, err
	}
	summary := s.buildCheckinSummary(ctx, date, !had)
	return CheckinResponse{Done: true, Summary: summary}, nil
}

// buildCheckinSummary 计算本月打卡 + 连续天数。triggerBonus 控制是否在本次新打卡时判定 7 天 bonus。
func (s *Service) buildCheckinSummary(ctx context.Context, date string, triggerBonus bool) CheckinSummary {
	now := time.Now()
	year, month, day := now.Date()
	yearMonth := fmt.Sprintf("%04d-%02d", year, int(month))
	monthCheckins, _ := s.store.ListCheckins(ctx, yearMonth)
	checked := map[string]bool{}
	for _, c := range monthCheckins {
		checked[c.TaskDate] = true
	}
	checkedDays := []int{}
	for d := 1; d <= daysInMonth(year, int(month)); d++ {
		key := fmt.Sprintf("%s-%02d", yearMonth, d)
		if checked[key] {
			checkedDays = append(checkedDays, d)
		}
	}
	// 连续天数：从今天倒推。用全量打卡记录（跨月连续不断）。
	allCheckins, _ := s.store.ListAllCheckins(ctx)
	allChecked := map[string]bool{}
	for _, c := range allCheckins {
		allChecked[c.TaskDate] = true
	}
	streak := 0
	cursor := now
	for {
		key := fmt.Sprintf("%04d-%02d-%02d", cursor.Year(), int(cursor.Month()), cursor.Day())
		if !allChecked[key] {
			break
		}
		streak++
		cursor = cursor.AddDate(0, 0, -1)
	}
	bonus := false
	if triggerBonus && streak >= 7 && streak%7 == 0 {
		bonus = true
	}
	return CheckinSummary{
		MonthLabel:  fmt.Sprintf("%d 月", int(month)),
		CheckedDays: checkedDays,
		TodayDay:    day,
		Streak:      streak,
		Bonus7Day:   bonus,
	}
}

func daysInMonth(year, month int) int {
	return time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC).Day()
}

// --- 工具 ---

func taskWordIDs(tasks []DailyTask) []string {
	ids := make([]string, 0, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.WordID)
	}
	return ids
}

func toMissionWord(w WordInfo, t DailyTask, l WordLearning) MissionWord {
	status := w.Status
	if l.WordID != "" {
		status = l.LearningStatus
	}
	return MissionWord{
		ID:            w.ID,
		Text:          w.Text,
		MeaningZh:     w.MeaningZh,
		Phonetic:      w.Phonetic,
		WordType:      w.WordType,
		TypeLabel:     typeLabelText(w.WordType),
		LearningLabel: statusLabelText(status),
		Stage:         stageText(l, status),
		PoolType:      t.PoolType,
		DisplayOrder:  t.DisplayOrder,
		ReadStatus:    t.ReadStatus,
	}
}

func typeLabelText(t string) string {
	switch t {
	case "mistake":
		return "易错词"
	default:
		return "新词"
	}
}

func statusLabelText(s string) string {
	switch s {
	case StatusLearning:
		return "学习中"
	case StatusReinforce:
		return "需强化"
	case StatusMastered:
		return "已掌握"
	default:
		return "未学"
	}
}

func stageText(l WordLearning, status string) string {
	switch status {
	case StatusUnlearned:
		return "初学"
	case StatusLearning:
		switch l.SuccessCount {
		case 0, 1:
			return "1天复习"
		case 2:
			return "3天复习"
		case 3:
			return "7天复习"
		case 4:
			return "14天复习"
		default:
			return "巩固中"
		}
	case StatusReinforce:
		switch l.SuccessCount {
		case 0:
			return "明天再巩固"
		case 1:
			return "1天巩固"
		case 2:
			return "3天巩固"
		case 3:
			return "7天巩固"
		default:
			return "14天巩固"
		}
	case StatusMastered:
		idx := l.MasteredReviewIndex
		if idx >= 0 && idx < len(masteredReviewDays) {
			return fmt.Sprintf("%d天抽查", masteredReviewDays[idx])
		}
		return "已掌握抽查"
	}
	return ""
}

func loadDayText(d string) (label, desc string) {
	switch d {
	case LoadDayLight:
		return "轻松日", "今天复习词不多，可以轻松完成。"
	case LoadDayPressure:
		return "压力日", "今天到期词偏多，系统只挑优先级最高的一批。"
	default:
		return "普通日", "今天复习量适中，按顺序完成就好。"
	}
}

// systemExample 系统占位例句（PRD：缺例句时使用占位）。
func systemExample(word string) string {
	if word == "" {
		return "例句待补充。"
	}
	return fmt.Sprintf("Let's learn the word \"%s\".", word)
}
