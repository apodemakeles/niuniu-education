package studentwordtask

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/apodemakeles/niuniu-education/backend/internal/platform/llm"
)

// service_test.go 端到端集成：store + scheduler + review + reading 通过 service 协作。

// TestE2E_TodayGeneration 验证任务生成幂等性与负载日。
func TestE2E_TodayGeneration(t *testing.T) {
	svc, store := newTestService(t)
	ctx := ctxbg()
	seedWord(t, store.db, "w1", "apple", "苹果", "/æpl/", "new", StatusUnlearned)
	seedWord(t, store.db, "w2", "desk", "书桌", "/desk/", "new", StatusUnlearned)
	seedWord(t, store.db, "w3", "read", "阅读", "/ri:d/", "mistake", StatusReinforce)
	seedLearning(t, store.db, "w3", StatusReinforce, 0, Today())

	resp, err := svc.GetToday(ctx)
	if err != nil {
		t.Fatalf("GetToday: %v", err)
	}
	if resp.Empty {
		t.Fatal("词库非空，不应是 empty")
	}
	if len(resp.FirstPool) != 2 {
		t.Fatalf("新词池=%d，期望 2（2 个未学词）", len(resp.FirstPool))
	}
	if len(resp.ReviewPool) != 1 {
		t.Fatalf("非新词池=%d，期望 1（1 个到期需强化词）", len(resp.ReviewPool))
	}
	if resp.TotalCount != 3 {
		t.Fatalf("总数=%d，期望 3", resp.TotalCount)
	}
	// 非新词候选 < 3 → 轻松日
	if resp.LoadDay != LoadDayLight {
		t.Fatalf("负载日=%s，期望 light", resp.LoadDay)
	}

	// 再次调用应幂等（不重复创建任务）
	resp2, _ := svc.GetToday(ctx)
	if len(resp2.FirstPool) != len(resp.FirstPool) {
		t.Fatal("重复 GetToday 不应改变任务")
	}
}

// TestE2E_TodayResponseContract 验证空任务池仍编码为 []，且字段名与前端契约一致。
func TestE2E_TodayResponseContract(t *testing.T) {
	svc, store := newTestService(t)
	seedWord(t, store.db, "w1", "apple", "苹果", "/æpl/", "new", StatusUnlearned)

	resp, err := svc.GetToday(ctxbg())
	if err != nil {
		t.Fatalf("GetToday: %v", err)
	}
	if resp.FirstPool == nil || resp.ReviewPool == nil {
		t.Fatalf("任务池必须是空数组而不是 null：first=%v review=%v", resp.FirstPool, resp.ReviewPool)
	}

	body, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	jsonText := string(body)
	if !strings.Contains(jsonText, `"reviewPool":[]`) {
		t.Fatalf("reviewPool 应编码为 []：%s", jsonText)
	}
	if !strings.Contains(jsonText, `"readStatus":"not_started"`) {
		t.Fatalf("readStatus 字段应使用 camelCase：%s", jsonText)
	}
	if strings.Contains(jsonText, `"read_status"`) {
		t.Fatalf("不应再返回 snake_case read_status：%s", jsonText)
	}
}

func TestE2E_TodayCompletedAfterFirstCorrect(t *testing.T) {
	svc, store := newTestService(t)
	ctx := ctxbg()
	seedWord(t, store.db, "w1", "apple", "苹果", "/æpl/", "new", StatusUnlearned)

	initial, err := svc.GetToday(ctx)
	if err != nil {
		t.Fatalf("GetToday before submit: %v", err)
	}
	if initial.Completed {
		t.Fatal("首次默写前不能标记为完成")
	}

	dictation, err := svc.GetDictation(ctx)
	if err != nil {
		t.Fatalf("GetDictation: %v", err)
	}
	item := dictation.Items[0]
	if _, err := svc.SubmitDictation(ctx, SubmitDictationRequest{Answers: []SubmitAnswer{{
		TaskID: item.TaskID, WordID: item.WordID, Answer: "apple",
	}}}); err != nil {
		t.Fatalf("SubmitDictation: %v", err)
	}

	completed, err := svc.GetToday(ctx)
	if err != nil {
		t.Fatalf("GetToday after submit: %v", err)
	}
	if !completed.Completed {
		t.Fatal("首次全对后应标记为完成")
	}
}

// TestE2E_EmptyLibrary 空库应返回 empty 状态。
func TestE2E_EmptyLibrary(t *testing.T) {
	svc, _ := newTestService(t)
	resp, err := svc.GetToday(ctxbg())
	if err != nil {
		t.Fatalf("GetToday: %v", err)
	}
	if !resp.Empty {
		t.Fatal("空库应是 empty")
	}
	if resp.EmptyHint == "" {
		t.Fatal("应有空状态提示")
	}
	if resp.FirstPool == nil || resp.ReviewPool == nil {
		t.Fatal("空词库的任务池也必须返回 []，不能返回 null")
	}
}

// TestE2E_EnterCardUnlearnedToLearning 进入单词卡后未学变学习中。
func TestE2E_EnterCardUnlearnedToLearning(t *testing.T) {
	svc, store := newTestService(t)
	ctx := ctxbg()
	seedWord(t, store.db, "w1", "apple", "苹果", "/æpl/", "new", StatusUnlearned)
	seedWord(t, store.db, "w2", "desk", "书桌", "/desk/", "new", StatusUnlearned)
	seedWord(t, store.db, "w3", "read", "阅读", "/ri:d/", "mistake", StatusReinforce)
	seedLearning(t, store.db, "w3", StatusReinforce, 0, Today())

	_, err := svc.EnterCard(ctx, "w1")
	if err != nil {
		t.Fatalf("EnterCard: %v", err)
	}
	l, _ := store.GetLearning(ctx, "w1")
	if l.LearningStatus != StatusLearning {
		t.Fatalf("进入卡片后状态=%s，期望 learning", l.LearningStatus)
	}
	if !contains(l.StudiedDates, Today()) {
		t.Fatal("应记录 studied_date")
	}
}

// TestE2E_DictationSubmit 新词池未学词首次正确 → bonus + 状态流转。
func TestE2E_DictationSubmit(t *testing.T) {
	svc, store := newTestService(t)
	ctx := ctxbg()
	seedWord(t, store.db, "w1", "apple", "苹果", "/æpl/", "new", StatusUnlearned)
	seedWord(t, store.db, "w2", "desk", "书桌", "/desk/", "new", StatusUnlearned)
	seedWord(t, store.db, "w3", "read", "阅读", "/ri:d/", "mistake", StatusReinforce)
	seedLearning(t, store.db, "w3", StatusReinforce, 0, Today())

	// 先 EnterCard 触发任务生成 + w1 转学习中
	if _, err := svc.EnterCard(ctx, "w1"); err != nil {
		t.Fatal(err)
	}

	// 取默写题面
	dr, err := svc.GetDictation(ctx)
	if err != nil {
		t.Fatalf("GetDictation: %v", err)
	}
	if len(dr.Items) != 3 {
		t.Fatalf("默写题数=%d，期望 3", len(dr.Items))
	}

	// 提交：w1 正确、w2 留空（未填写）、w3 错误
	var answers []SubmitAnswer
	for _, it := range dr.Items {
		ans := ""
		switch it.WordID {
		case "w1":
			ans = "apple"
		case "w3":
			ans = "wrong"
		}
		answers = append(answers, SubmitAnswer{TaskID: it.TaskID, WordID: it.WordID, Answer: ans})
	}
	resp, err := svc.SubmitDictation(ctx, SubmitDictationRequest{Answers: answers})
	if err != nil {
		t.Fatalf("SubmitDictation: %v", err)
	}
	if resp.Correct != 1 {
		t.Fatalf("正确数=%d，期望 1", resp.Correct)
	}
	if resp.Blank != 1 {
		t.Fatalf("未填写数=%d，期望 1", resp.Blank)
	}
	if resp.Wrong != 1 {
		t.Fatalf("错误数=%d，期望 1", resp.Wrong)
	}
	// w1 应触发 bonus（新词池未学词首次正确）
	if len(resp.BonusIDs) != 1 || resp.BonusIDs[0] != "w1" {
		t.Fatalf("bonus=%v，期望 [w1]", resp.BonusIDs)
	}

	// w1 学习记录：学习中，success_count=1，明天到期
	l1, _ := store.GetLearning(ctx, "w1")
	if l1.LearningStatus != StatusLearning || l1.SuccessCount != 1 {
		t.Fatalf("w1 流转：状态=%s 连对=%d，期望 learning/1", l1.LearningStatus, l1.SuccessCount)
	}
	// w3 错误：需强化，清零
	l3, _ := store.GetLearning(ctx, "w3")
	if l3.LearningStatus != StatusReinforce || l3.SuccessCount != 0 {
		t.Fatalf("w3 流转：状态=%s 连对=%d，期望 reinforce/0", l3.LearningStatus, l3.SuccessCount)
	}

	// 再次提交应幂等（已锁定，不改结果）
	resp2, _ := svc.SubmitDictation(ctx, SubmitDictationRequest{Answers: answers})
	if resp2.Correct != 1 {
		t.Fatal("重复提交不应改变结果")
	}
}

// TestE2E_DictationLockedAfterSubmit 提交后 first_dictation 锁定。
func TestE2E_DictationLockedAfterSubmit(t *testing.T) {
	svc, store := newTestService(t)
	ctx := ctxbg()
	seedWord(t, store.db, "w1", "apple", "苹果", "", "new", StatusUnlearned)
	seedWord(t, store.db, "w2", "desk", "书桌", "", "new", StatusUnlearned)
	seedWord(t, store.db, "w3", "read", "阅读", "", "mistake", StatusReinforce)
	seedLearning(t, store.db, "w3", StatusReinforce, 0, Today())
	svc.EnterCard(ctx, "w1")

	dr, _ := svc.GetDictation(ctx)
	answers := []SubmitAnswer{{TaskID: dr.Items[0].TaskID, WordID: "w1", Answer: "apple"}}
	svc.SubmitDictation(ctx, SubmitDictationRequest{Answers: answers})

	// w1 应锁定
	tasks, _ := store.ListDailyTasks(ctx, Today())
	for _, tk := range tasks {
		if tk.WordID == "w1" && !tk.SubmissionLocked {
			t.Fatal("提交后 w1 应锁定")
		}
	}
}

// TestE2E_CorrectionDoesNotChangeFirst 订正不改首次结果与 success_count。
func TestE2E_CorrectionDoesNotChangeFirst(t *testing.T) {
	svc, store := newTestService(t)
	ctx := ctxbg()
	seedWord(t, store.db, "w1", "apple", "苹果", "", "new", StatusUnlearned)
	seedWord(t, store.db, "w2", "desk", "书桌", "", "new", StatusUnlearned)
	seedWord(t, store.db, "w3", "read", "阅读", "", "mistake", StatusReinforce)
	seedLearning(t, store.db, "w3", StatusReinforce, 0, Today())
	svc.EnterCard(ctx, "w1")

	dr, _ := svc.GetDictation(ctx)
	// w3 首次错误
	var w1TaskID, w3TaskID string
	var answers []SubmitAnswer
	for _, it := range dr.Items {
		if it.WordID == "w1" {
			w1TaskID = it.TaskID
		}
		if it.WordID == "w3" {
			w3TaskID = it.TaskID
			answers = append(answers, SubmitAnswer{TaskID: it.TaskID, WordID: it.WordID, Answer: "wrong"})
		} else {
			answers = append(answers, SubmitAnswer{TaskID: it.TaskID, WordID: it.WordID, Answer: ""})
		}
	}
	svc.SubmitDictation(ctx, SubmitDictationRequest{Answers: answers})

	// 订正 w3 正确
	svc.CorrectDictation(ctx, CorrectDictationRequest{Answers: []SubmitAnswer{{TaskID: w3TaskID, WordID: "w3", Answer: "read"}}})
	// 下一轮只提交 w1；不能把本轮未提交的 w3 覆盖成空答案。
	svc.CorrectDictation(ctx, CorrectDictationRequest{Answers: []SubmitAnswer{{TaskID: w1TaskID, WordID: "w1", Answer: "apple"}}})

	// w3 首次结果仍是 wrong，success_count 仍 0
	l3, _ := store.GetLearning(ctx, "w3")
	if l3.SuccessCount != 0 {
		t.Fatalf("订正不应改 success_count，得 %d", l3.SuccessCount)
	}
	tasks, _ := store.ListDailyTasks(ctx, Today())
	for _, tk := range tasks {
		if tk.WordID == "w3" {
			if tk.FirstDictationResult != DictWrong {
				t.Fatalf("首次结果应仍是 wrong，得 %s", tk.FirstDictationResult)
			}
			if tk.CorrectionResult != CorrectCorrect {
				t.Fatalf("后续订正其他词不能覆盖 w3；订正结果应为 correct，得 %s", tk.CorrectionResult)
			}
		}
	}
}

// TestE2E_CheckinStreak 连续打卡统计。
func TestE2E_CheckinStreak(t *testing.T) {
	svc, store := newTestService(t)
	ctx := ctxbg()
	// 预置近 6 天打卡
	for i := 6; i >= 1; i-- {
		d := AddDays(Today(), -i)
		store.InsertCheckin(ctx, d)
	}
	// 今天打卡 → 连续 7 天，应触发 bonus
	resp, err := svc.Checkin(ctx)
	if err != nil {
		t.Fatalf("Checkin: %v", err)
	}
	if !resp.Done {
		t.Fatal("应打卡成功")
	}
	if resp.Summary.Streak != 7 {
		t.Fatalf("连续天数=%d，期望 7", resp.Summary.Streak)
	}
	if !resp.Summary.Bonus7Day {
		t.Fatal("连续 7 天应触发 bonus")
	}
}

func TestBuildCardDetail_UsesGeneratedExample(t *testing.T) {
	svc, store := newTestService(t)
	ctx := ctxbg()
	seedWord(t, store.db, "w-example", "kitchen", "厨房", "", "new", StatusUnlearned)
	if _, err := store.db.Exec(`INSERT INTO word_examples(id, word_id, sentence, display_order) VALUES ('ex-1', 'w-example', 'I eat in the kitchen.', 0)`); err != nil {
		t.Fatal(err)
	}
	tasks := []DailyTask{{ID: "task-example", WordID: "w-example", TaskDate: Today(), PoolType: PoolFirst}}
	card, err := svc.buildCardDetail(ctx, tasks, "w-example")
	if err != nil {
		t.Fatal(err)
	}
	if card.Example != "I eat in the kitchen." || card.ExampleMissing {
		t.Fatalf("应展示已生成例句，got %+v", card)
	}
}

// TestE2E_ReadingFlowWithMockLLM 阅读生成（mock LLM 返回合格短文）。
func TestE2E_ReadingFlowWithMockLLM(t *testing.T) {
	db := newTestDB(t)
	// 任务词：w1(apple 新词池)、w3(read 非新词池)。w2(desk) 也是未学进新词池。
	// SelectCoveredWords 优先选新词池(apple, desk)再选需强化(read)，最多 6 个。
	// mock 短文需让所有被选覆盖词各恰好出现 2 次。
	passage := ""
	for _, w := range []string{"apple", "desk", "read"} {
		passage += w + " " + w + " "
	}
	// 填充到 ≥80 词
	fillers := []string{"the", "cat", "sits", "on", "mat", "and", "looks", "at", "sun", "sky"}
	for CountEnglishWords(passage) < 90 {
		for _, f := range fillers {
			passage += f + " "
			if CountEnglishWords(passage) >= 90 {
				break
			}
		}
	}
	mockText := `{"title":"School Day","text":"` + passage + `","sceneHint":"一个上学的日子"}`
	store := NewStore(db)
	svc := NewService(store, llm.NewMockProvider(mockText), "", testLogger())

	seedWord(t, db, "w1", "apple", "苹果", "/æpl/", "new", StatusUnlearned)
	seedWord(t, db, "w2", "desk", "书桌", "/desk/", "new", StatusUnlearned)
	seedWord(t, db, "w3", "read", "阅读", "/ri:d/", "mistake", StatusReinforce)
	seedLearning(t, db, "w3", StatusReinforce, 0, Today())

	rr, err := svc.GetReading(context.Background())
	if err != nil {
		t.Fatalf("GetReading: %v", err)
	}
	if rr.Status != ReadingSuccess {
		t.Fatalf("短文状态=%s，期望 success", rr.Status)
	}
	if rr.Title != "School Day" {
		t.Fatalf("标题=%s", rr.Title)
	}
	if len(rr.Appearances) == 0 {
		t.Fatal("应有出现次数统计")
	}
	// 每个覆盖词应恰好出现 2 次
	for _, ap := range rr.Appearances {
		if ap.Count != 2 {
			t.Fatalf("词 %s 出现 %d 次，应恰好为2", ap.ID, ap.Count)
		}
	}
}

// TestDebugMode_SkipReadingWait 调试模式下可立即完成阅读，无需等满 minSeconds。
func TestDebugMode_SkipReadingWait(t *testing.T) {
	svc, store := newTestService(t)
	date := Today()
	now := time.Now().UTC().Format(time.RFC3339)
	p := ReadingPassage{
		TaskDate:           date,
		ReadingTitle:       "Debug Story",
		ReadingText:        "hello world",
		AIGenerationStatus: ReadingSuccess,
		ReadingMinSeconds:  120,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	p.ReadingStartedAt.Valid = true
	p.ReadingStartedAt.String = now
	if err := store.UpsertReading(context.Background(), p); err != nil {
		t.Fatalf("seed reading: %v", err)
	}

	if err := svc.CompleteReading(context.Background()); err != ErrReadingTooShort {
		t.Fatalf("未开调试时期望 ErrReadingTooShort，got %v", err)
	}

	svc.SetDebugMode(true)
	rr := svc.buildReadingResponse(p)
	if !rr.DebugMode || !rr.CanFinish {
		t.Fatalf("调试模式响应应可完成: debug=%v canFinish=%v", rr.DebugMode, rr.CanFinish)
	}
	if err := svc.CompleteReading(context.Background()); err != nil {
		t.Fatalf("调试模式应允许立即完成阅读: %v", err)
	}
}

// makePassageWithOccurrences 生成包含目标词 n 次的、长度足够的短文。
func makePassageWithOccurrences(word string, times, targetWords int) string {
	s := ""
	for i := 0; i < times; i++ {
		s += word + " "
	}
	// 用不同 filler 词填充到 ≥ targetWords 个英文词，避免重复词被误统计
	fillers := []string{"the", "cat", "sits", "on", "the", "mat", "and", "looks", "at", "the", "sun", "in", "the", "sky"}
	for CountEnglishWords(s) < targetWords {
		for _, f := range fillers {
			if CountEnglishWords(s) >= targetWords {
				break
			}
			s += f + " "
		}
	}
	return s
}
