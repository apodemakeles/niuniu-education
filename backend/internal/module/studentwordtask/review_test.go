package studentwordtask

import "testing"

// review_test.go 穷举 PRD「学习状态流转规则」与「艾宾浩斯排程规则」矩阵。

func TestApplyDictation_UnlearnedFirstCorrect(t *testing.T) {
	// 新词池未学词首次学习正确：+1，学习中，1 天后
	in := WordLearning{LearningStatus: StatusUnlearned, SuccessCount: 0}
	out := ApplyDictation(in, DictationOutcome{Correct: true, PoolType: PoolFirst}, "2026-01-01", false)
	if out.LearningStatus != StatusLearning || out.SuccessCount != 1 {
		t.Fatalf("未学首次正确：状态=%s 连对=%d，期望 learning/1", out.LearningStatus, out.SuccessCount)
	}
	if out.NextDueDate != "2026-01-02" {
		t.Fatalf("未学首次正确 next_due=%s，期望 2026-01-02", out.NextDueDate)
	}
	if !contains(out.StudiedDates, "2026-01-01") || out.LastStudiedDate != "2026-01-01" {
		t.Fatalf("studied_dates 未记录当天")
	}
	if !contains(out.SuccessDates, "2026-01-01") || out.LastSuccessDate != "2026-01-01" {
		t.Fatalf("success_dates 未记录当天")
	}
}

func TestApplyDictation_LearningToMastered(t *testing.T) {
	today := "2026-01-10"
	// 完成第 5 次首次正确 → 已掌握，30 天后
	in := WordLearning{LearningStatus: StatusLearning, SuccessCount: 4, NextDueDate: today}
	out := ApplyDictation(in, DictationOutcome{Correct: true}, today, false)
	if out.LearningStatus != StatusMastered {
		t.Fatalf("学习中第5次正确应已掌握，得 %s", out.LearningStatus)
	}
	if out.SuccessCount != 5 {
		t.Fatalf("success_count=%d，期望 5", out.SuccessCount)
	}
	if out.NextDueDate != "2026-02-09" { // 30 天后
		t.Fatalf("已掌握 next_due=%s，期望 2026-02-09", out.NextDueDate)
	}
	if out.FirstMasteredDate != today {
		t.Fatalf("first_mastered_date=%s，期望 %s", out.FirstMasteredDate, today)
	}
	if out.MasteredReviewIndex != 0 {
		t.Fatalf("mastered_review_index=%d，期望 0", out.MasteredReviewIndex)
	}
}

func TestApplyDictation_LearningSecondCorrect(t *testing.T) {
	// 学习中第 2 次正确：3 天后
	in := WordLearning{LearningStatus: StatusLearning, SuccessCount: 1, NextDueDate: "2026-01-01"}
	out := ApplyDictation(in, DictationOutcome{Correct: true}, "2026-01-01", false)
	if out.SuccessCount != 2 || out.LearningStatus != StatusLearning {
		t.Fatalf("学习中第2次正确：状态=%s 连对=%d，期望 learning/2", out.LearningStatus, out.SuccessCount)
	}
	if out.NextDueDate != "2026-01-04" {
		t.Fatalf("next_due=%s，期望 2026-01-04", out.NextDueDate)
	}
}

func TestApplyDictation_LearningThirdAndFourthCorrect(t *testing.T) {
	third := ApplyDictation(WordLearning{LearningStatus: StatusLearning, SuccessCount: 2}, DictationOutcome{Correct: true}, "2026-01-01", false)
	if third.LearningStatus != StatusLearning || third.SuccessCount != 3 || third.NextDueDate != "2026-01-08" {
		t.Fatalf("第3次正确应7天后巩固，got %+v", third)
	}
	fourth := ApplyDictation(WordLearning{LearningStatus: StatusLearning, SuccessCount: 3}, DictationOutcome{Correct: true}, "2026-01-01", false)
	if fourth.LearningStatus != StatusLearning || fourth.SuccessCount != 4 || fourth.NextDueDate != "2026-01-15" {
		t.Fatalf("第4次正确应14天后巩固，got %+v", fourth)
	}
}

func TestApplyDictation_ReinforceCorrectBelowThreshold(t *testing.T) {
	// 需强化第 2 次正确：3 天后巩固
	in := WordLearning{LearningStatus: StatusReinforce, SuccessCount: 1, NextDueDate: "2026-01-01"}
	out := ApplyDictation(in, DictationOutcome{Correct: true}, "2026-01-01", false)
	if out.LearningStatus != StatusReinforce || out.SuccessCount != 2 {
		t.Fatalf("需强化正确未满3次：状态=%s 连对=%d，期望 reinforce/2", out.LearningStatus, out.SuccessCount)
	}
	if out.NextDueDate != "2026-01-04" {
		t.Fatalf("next_due=%s，期望 2026-01-04", out.NextDueDate)
	}
}

func TestApplyDictation_ReinforceToMastered(t *testing.T) {
	// 需强化第 5 次正确 → 已掌握，30 天后
	in := WordLearning{LearningStatus: StatusReinforce, SuccessCount: 4, NextDueDate: "2026-01-01"}
	out := ApplyDictation(in, DictationOutcome{Correct: true}, "2026-01-01", false)
	if out.LearningStatus != StatusMastered || out.SuccessCount != 5 {
		t.Fatalf("需强化第5次正确应已掌握：状态=%s 连对=%d", out.LearningStatus, out.SuccessCount)
	}
	if out.NextDueDate != "2026-01-31" {
		t.Fatalf("next_due=%s，期望 2026-01-31", out.NextDueDate)
	}
}

func TestApplyDictation_WrongClearsAndReinforce(t *testing.T) {
	// 任意非已掌握词首次错误：清零，需强化，明天
	in := WordLearning{LearningStatus: StatusLearning, SuccessCount: 2, NextDueDate: "2026-01-01"}
	out := ApplyDictation(in, DictationOutcome{Correct: false}, "2026-01-01", false)
	if out.LearningStatus != StatusReinforce || out.SuccessCount != 0 {
		t.Fatalf("错误应清零进需强化：状态=%s 连对=%d", out.LearningStatus, out.SuccessCount)
	}
	if out.NextDueDate != "2026-01-02" {
		t.Fatalf("next_due=%s，期望 2026-01-02", out.NextDueDate)
	}
	// 不应记录 success_dates
	if contains(out.SuccessDates, "2026-01-01") {
		t.Fatalf("错误不应记录 success_date")
	}
}

func TestApplyDictation_BlankTreatedAsWrong(t *testing.T) {
	in := WordLearning{LearningStatus: StatusLearning, SuccessCount: 2}
	out := ApplyDictation(in, DictationOutcome{Correct: false, Blank: true}, "2026-01-01", false)
	if out.LearningStatus != StatusReinforce || out.SuccessCount != 0 {
		t.Fatalf("未填写应按错误处理：状态=%s 连对=%d", out.LearningStatus, out.SuccessCount)
	}
}

func TestApplyDictation_HardBeforeSubmit(t *testing.T) {
	// 首次提交前点过「不会读」：进需强化，清零，明天
	in := WordLearning{LearningStatus: StatusLearning, SuccessCount: 2}
	out := ApplyDictation(in, DictationOutcome{Correct: true}, "2026-01-01", true)
	if out.LearningStatus != StatusReinforce || out.SuccessCount != 0 {
		t.Fatalf("不会读应清零进需强化：状态=%s 连对=%d", out.LearningStatus, out.SuccessCount)
	}
	if out.NextDueDate != "2026-01-02" {
		t.Fatalf("next_due=%s，期望 2026-01-02", out.NextDueDate)
	}
}

func TestApplyDictation_MasteredQuizCorrect(t *testing.T) {
	// 已掌握抽查正确：保持已掌握，阶段推进，间隔拉长
	in := WordLearning{LearningStatus: StatusMastered, SuccessCount: 5, MasteredReviewIndex: 0}
	out := ApplyDictation(in, DictationOutcome{Correct: true}, "2026-01-01", false)
	if out.LearningStatus != StatusMastered {
		t.Fatalf("已掌握抽查正确应保持已掌握：状态=%s", out.LearningStatus)
	}
	// 阶段 0 抽查正确 → 用阶段 1 的 60 天
	if out.NextDueDate != "2026-03-02" {
		t.Fatalf("已掌握抽查 next_due=%s，期望 2026-03-02(60天)", out.NextDueDate)
	}
	if out.MasteredReviewIndex != 1 {
		t.Fatalf("阶段=%d，期望 1", out.MasteredReviewIndex)
	}
}

func TestApplyDictation_MasteredQuizWrong(t *testing.T) {
	// 已掌握抽查错误：清零，需强化，明天
	in := WordLearning{LearningStatus: StatusMastered, SuccessCount: 3, MasteredReviewIndex: 2}
	out := ApplyDictation(in, DictationOutcome{Correct: false}, "2026-01-01", false)
	if out.LearningStatus != StatusReinforce || out.SuccessCount != 0 {
		t.Fatalf("已掌握抽查错误应回到需强化：状态=%s 连对=%d", out.LearningStatus, out.SuccessCount)
	}
	if out.MasteredReviewIndex != 2 {
		t.Fatalf("抽查失败不应重置 mastered_review_index，得 %d", out.MasteredReviewIndex)
	}
}

func TestMarkHardBeforeSubmit(t *testing.T) {
	in := WordLearning{LearningStatus: StatusMastered, SuccessCount: 3}
	out := MarkHardBeforeSubmit(in, "2026-01-01")
	if out.LearningStatus != StatusReinforce || out.SuccessCount != 0 || out.NextDueDate != "2026-01-02" {
		t.Fatalf("MarkHard 结果：状态=%s 连对=%d next=%s", out.LearningStatus, out.SuccessCount, out.NextDueDate)
	}
}

func TestEnterCard_UnlearnedBecomesLearning(t *testing.T) {
	in := WordLearning{LearningStatus: StatusUnlearned}
	out := EnterCard(in, "2026-01-01")
	if out.LearningStatus != StatusLearning {
		t.Fatalf("未学进入卡片应变学习中，得 %s", out.LearningStatus)
	}
	if !contains(out.StudiedDates, "2026-01-01") {
		t.Fatalf("应记录 studied_date")
	}
}

func TestEnterCard_IdempotentForLearning(t *testing.T) {
	// 已学习的词再进入卡片不改变状态
	in := WordLearning{LearningStatus: StatusMastered, SuccessCount: 3, MasteredReviewIndex: 1}
	out := EnterCard(in, "2026-01-01")
	if out.LearningStatus != StatusMastered || out.MasteredReviewIndex != 1 {
		t.Fatalf("已掌握进入卡片不应改变状态")
	}
}

func TestJudgeAnswer(t *testing.T) {
	cases := []struct {
		answer, expected, want string
	}{
		{"apple", "apple", DictCorrect},
		{"Apple", "apple", DictCorrect},
		{"  apple  ", "apple", DictCorrect},
		{"APPLE", "apple", DictCorrect},
		{"appel", "apple", DictWrong},
		{"", "apple", DictBlank},
		{"   ", "apple", DictBlank},
	}
	for _, c := range cases {
		if got := JudgeAnswer(c.answer, c.expected); got != c.want {
			t.Errorf("JudgeAnswer(%q,%q)=%s，期望 %s", c.answer, c.expected, got, c.want)
		}
	}
}
