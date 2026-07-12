package studentwordtask

import (
	"fmt"
	"testing"
)

// scheduler_test.go 验证任务生成算法：新词公平排队、复习优先级与负载日。

func mkWord(id, text, status string) WordInfo {
	return WordInfo{ID: id, Text: text, Status: status, WordType: "new"}
}

func mkLearning(id, status string, successCount int, due string) WordLearning {
	return WordLearning{WordID: id, LearningStatus: status, SuccessCount: successCount, NextDueDate: due}
}

func TestBuildTodayPlan_NewPoolOldestFirstUpTo3(t *testing.T) {
	// 5 个未学词，新词池上限 3，最早入库的必须先被安排。
	words := []WordInfo{
		{ID: "w5", Text: "e", Status: StatusUnlearned, CreatedAt: "2026-01-05"},
		{ID: "w3", Text: "c", Status: StatusUnlearned, CreatedAt: "2026-01-03"},
		{ID: "w1", Text: "a", Status: StatusUnlearned, CreatedAt: "2026-01-01"},
		{ID: "w4", Text: "d", Status: StatusUnlearned, CreatedAt: "2026-01-04"},
		{ID: "w2", Text: "b", Status: StatusUnlearned, CreatedAt: "2026-01-02"},
	}
	st := DefaultStrategy()
	plan := BuildTodayPlan("2026-01-01", "", st, words, nil, func(int) int { return 0 })
	if len(plan.FirstPool) != 3 {
		t.Fatalf("新词池数量=%d，期望 3", len(plan.FirstPool))
	}
	if len(plan.ReviewPool) != 0 {
		t.Fatalf("无非新词候选，得 %d", len(plan.ReviewPool))
	}
	got := []string{plan.FirstPool[0].Word.ID, plan.FirstPool[1].Word.ID, plan.FirstPool[2].Word.ID}
	if want := []string{"w1", "w2", "w3"}; got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("新词应按等待时间排队，got %v want %v", got, want)
	}
}

func TestBuildTodayPlan_NewPoolFewerThanMax(t *testing.T) {
	// 仅 2 个未学词，新词池按实际数量
	words := []WordInfo{
		mkWord("w1", "a", StatusUnlearned),
		mkWord("w2", "b", StatusUnlearned),
	}
	plan := BuildTodayPlan("2026-01-01", "", DefaultStrategy(), words, nil, nil)
	if len(plan.FirstPool) != 2 {
		t.Fatalf("新词池数量=%d，期望 2", len(plan.FirstPool))
	}
}

func TestBuildTodayPlan_ReviewPoolScoreOrdering(t *testing.T) {
	// 需强化词分数最高，应排在非新词池最前
	today := "2026-01-10"
	words := []WordInfo{
		mkWord("learn", "learn", StatusLearning),   // base 70
		mkWord("reinf", "reinf", StatusReinforce),  // base 90
		mkWord("master", "master", StatusMastered), // base 10
	}
	learnings := []WordLearning{
		mkLearning("learn", StatusLearning, 1, today),  // 70 + 40 = 110
		mkLearning("reinf", StatusReinforce, 0, today), // 90 + 40 = 130
		mkLearning("master", StatusMastered, 3, today), // 已掌握，需命中抽查概率
	}
	// 已掌握抽查概率=1%，用固定 randFn 返回 0 让其命中
	plan := BuildTodayPlan(today, "", DefaultStrategy(), words, learnings, func(max int) int { return 0 })
	if len(plan.ReviewPool) < 2 {
		t.Fatalf("非新词池数量=%d，期望 ≥2", len(plan.ReviewPool))
	}
	// reinforce(130) 应排在 learning(110) 前
	if plan.ReviewPool[0].Word.ID != "reinf" {
		t.Fatalf("分数最高者应在首位，得 %s", plan.ReviewPool[0].Word.ID)
	}
}

func TestBuildTodayPlan_ReinforceScoreHigherThanLearning(t *testing.T) {
	today := "2026-01-10"
	words := []WordInfo{
		mkWord("learn", "learn", StatusLearning),
		mkWord("reinf", "reinf", StatusReinforce),
	}
	learnings := []WordLearning{
		mkLearning("learn", StatusLearning, 1, today),
		mkLearning("reinf", StatusReinforce, 0, today),
	}
	plan := BuildTodayPlan(today, "", DefaultStrategy(), words, learnings, nil)
	got := map[string]int{}
	for _, c := range plan.ReviewPool {
		got[c.Word.ID] = computeReviewScore(c.Learning.LearningStatus, c.Learning.NextDueDate, today, DefaultStrategy())
	}
	if got["reinf"] <= got["learn"] {
		t.Fatalf("需强化分 %d 应高于学习中 %d", got["reinf"], got["learn"])
	}
}

func TestComputeReviewScore_OverdueCap(t *testing.T) {
	st := DefaultStrategy()
	// 学习中，逾期很多天：base 70 + 到期 40 + 逾期封顶 56 = 166
	score := computeReviewScore(StatusLearning, "2025-01-01", "2026-01-01", st)
	if score != 70+40+56 {
		t.Fatalf("逾期封顶分=%d，期望 %d", score, 70+40+56)
	}
}

func TestComputeReviewScore_NotDue(t *testing.T) {
	st := DefaultStrategy()
	// 未到期：只有 base，无到期/逾期加分
	score := computeReviewScore(StatusReinforce, "2026-02-01", "2026-01-01", st)
	if score != 90 {
		t.Fatalf("未到期分=%d，期望 90", score)
	}
}

func TestClassifyLoadDay(t *testing.T) {
	st := DefaultStrategy()
	cases := []struct {
		qualified int
		want      string
	}{
		{0, LoadDayLight},
		{2, LoadDayLight},
		{3, LoadDayNormal},
		{9, LoadDayNormal},
		{10, LoadDayPressure},
	}
	for _, c := range cases {
		if got := classifyLoadDay(c.qualified, st.ReviewPoolMaxCount); got != c.want {
			t.Errorf("classifyLoadDay(%d)=%s，期望 %s", c.qualified, got, c.want)
		}
	}
}

func TestBuildTodayPlan_OverdueBacklogPausesNewAndTransfersSlots(t *testing.T) {
	today := "2026-01-10"
	words := []WordInfo{
		mkWord("new1", "new1", StatusUnlearned),
		mkWord("new2", "new2", StatusUnlearned),
		mkWord("new3", "new3", StatusUnlearned),
	}
	learnings := []WordLearning{}
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("r%02d", i)
		words = append(words, mkWord(id, id, StatusReinforce))
		learnings = append(learnings, mkLearning(id, StatusReinforce, 0, "2026-01-08"))
	}
	plan := BuildTodayPlan(today, "", DefaultStrategy(), words, learnings, func(int) int { return 0 })
	if len(plan.FirstPool) != 0 {
		t.Fatalf("逾期复习达到暂停阈值时不应加入新词，got %d", len(plan.FirstPool))
	}
	if plan.ReviewPoolCapacity != 12 || len(plan.ReviewPool) != 12 {
		t.Fatalf("空出的3个新词名额应转给需强化词，capacity=%d count=%d", plan.ReviewPoolCapacity, len(plan.ReviewPool))
	}
}

func TestBuildTodayPlan_NotDueExcluded(t *testing.T) {
	// 学习中词未到期，不进入候选
	today := "2026-01-10"
	words := []WordInfo{mkWord("w", "w", StatusLearning)}
	learnings := []WordLearning{mkLearning("w", StatusLearning, 1, "2026-02-01")}
	plan := BuildTodayPlan(today, "", DefaultStrategy(), words, learnings, nil)
	if len(plan.ReviewPool) != 0 {
		t.Fatalf("未到期词不应进入候选，得 %d", len(plan.ReviewPool))
	}
}

func TestBuildTodayPlan_MasteredSamplingStoppedAfterStopDay(t *testing.T) {
	// 学期第 130 天（>120），已掌握词停止抽查
	today := "2026-06-01"
	termStart := "2026-01-22" // 到 today 约 130 天
	words := []WordInfo{mkWord("m", "m", StatusMastered)}
	learnings := []WordLearning{mkLearning("m", StatusMastered, 3, today)}
	plan := BuildTodayPlan(today, termStart, DefaultStrategy(), words, learnings, func(int) int { return 0 })
	if len(plan.ReviewPool) != 0 {
		t.Fatalf("收口期已掌握词不应抽查，得 %d", len(plan.ReviewPool))
	}
}

func TestAllocateDisplayOrder(t *testing.T) {
	first := []CandidateWord{{Word: mkWord("a", "a", "")}, {Word: mkWord("b", "b", "")}}
	review := []CandidateWord{{Word: mkWord("c", "c", "")}}
	fo, ro := AllocateDisplayOrder(first, review)
	if fo["a"] != 0 || fo["b"] != 1 {
		t.Fatalf("新词池 order=%v", fo)
	}
	if ro["c"] != 2 {
		t.Fatalf("非新词池 order=%v，期望 c=2", ro)
	}
}
