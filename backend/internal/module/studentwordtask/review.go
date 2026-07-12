package studentwordtask

// review.go 实现学习状态流转 + 艾宾浩斯排程（对齐 PRD「学习状态流转规则」「艾宾浩斯排程规则」）。
// 全部为纯函数（输入 WordLearning + 事件 + 当日，输出新的 WordLearning），
// 与 store 解耦，便于在 *_test.go 中穷举矩阵。

// DictationOutcome 一次默写的判定结果（不含答案原文，只看正误）。
type DictationOutcome struct {
	WordID  string
	Correct bool
	Blank   bool // 未填写（按错误处理）
	// PoolType 仅用于判定「新词池未学词首次正确」bonus 触发条件，不影响状态流转。
	PoolType string
}

// ApplyDictation 把一次首次默写结果应用到学习记录。
// isHardBeforeSubmit 表示该词在首次提交前是否点过「不会读」。
//
// 返回更新后的 WordLearning。规则严格对齐 PRD「学习状态流转规则」表。
//
// 注意：本函数只处理「首次默写」语义。订正（correctDictation）不调用本函数，
// 订正不改 success_count / next_due_date（PRD 明确）。
func ApplyDictation(in WordLearning, oc DictationOutcome, today string, isHardBeforeSubmit bool) WordLearning {
	out := in
	// 当天首次进入学习流程即记 studied_dates / last_studied_date
	out.LastStudiedDate = today
	out.StudiedDates = appendUnique(out.StudiedDates, today)

	// 首次提交前点过「不会读」或本次错误/未填写 → 需强化，清零，明天到期
	if isHardBeforeSubmit || !oc.Correct {
		out.LearningStatus = StatusReinforce
		out.SuccessCount = 0
		out.NextDueDate = AddDays(today, 1)
		return out
	}

	// 以下为「首次默写正确」分支
	switch out.LearningStatus {
	case StatusUnlearned:
		// 未学词首次正确：+1，学习中，1 天后（PRD：新词池未学词首次学习正确）
		out.LearningStatus = StatusLearning
		out.SuccessCount = 1
		out.NextDueDate = AddDays(today, 1)
	case StatusLearning:
		out.SuccessCount++
		if out.SuccessCount >= requiredSuccessfulRetrievals {
			becomeMastered(&out, today)
		} else {
			out.NextDueDate = nextConsolidationDueDate(out.SuccessCount, today)
		}
	case StatusReinforce:
		out.SuccessCount++
		if out.SuccessCount >= requiredSuccessfulRetrievals {
			becomeMastered(&out, today)
		} else {
			out.NextDueDate = nextConsolidationDueDate(out.SuccessCount, today)
		}
	case StatusMastered:
		// 已掌握抽查正确：保持已掌握，按阶段拉长间隔
		out.SuccessCount = requiredSuccessfulRetrievals // 封顶
		out.NextDueDate = advanceMasteredDueDate(out.MasteredReviewIndex, today)
		// 推进抽查阶段（封顶到最后一档）
		if out.MasteredReviewIndex < len(masteredReviewDays)-1 {
			out.MasteredReviewIndex++
		}
	}

	// 首次默写正确：记录成功日期、累加 success_dates（一天一次，幂等）
	out.LastSuccessDate = today
	if !contains(out.SuccessDates, today) {
		out.SuccessDates = append(out.SuccessDates, today)
	}
	// success_count 封顶为巩固阶段要求次数。
	if out.SuccessCount > requiredSuccessfulRetrievals {
		out.SuccessCount = requiredSuccessfulRetrievals
	}
	return out
}

// becomeMastered 把记录置为已掌握：完成 1/3/7/14 天巩固后，30 天后首次抽查。
func becomeMastered(l *WordLearning, today string) {
	l.LearningStatus = StatusMastered
	l.SuccessCount = requiredSuccessfulRetrievals
	l.MasteredReviewIndex = 0
	l.NextDueDate = AddDays(today, masteredReviewDays[0]) // 30 天后
	if l.FirstMasteredDate == "" {
		l.FirstMasteredDate = today
	}
}

// nextConsolidationDueDate 返回巩固阶段下次间隔：1/3/7/14 天。
func nextConsolidationDueDate(successCount int, today string) string {
	switch successCount {
	case 1:
		return AddDays(today, 1)
	case 2:
		return AddDays(today, 3)
	case 3:
		return AddDays(today, 7)
	default:
		return AddDays(today, 14)
	}
}

// advanceMasteredDueDate 已掌握抽查正确后，按当前抽查阶段决定下次到期。
// 阶段 0(初次已掌握) → 30 天；阶段 1 → 60 天；阶段 2 → 120 天；阶段 3+ → 240 天。
func advanceMasteredDueDate(reviewIndex int, today string) string {
	idx := reviewIndex
	if idx >= len(masteredReviewDays) {
		idx = len(masteredReviewDays) - 1
	}
	// 注意：抽查正确按「下一步阶段」的间隔。已掌握首次正确由 becomeMastered 处理（60 天）；
	// 这里处理已是 mastered 的抽查：阶段 0 已被 becomeMastered 用过，抽查正确进入阶段 1 用 60 天。
	if idx+1 < len(masteredReviewDays) {
		return AddDays(today, masteredReviewDays[idx+1])
	}
	return AddDays(today, masteredReviewDays[len(masteredReviewDays)-1])
}

// MarkHardBeforeSubmit 单词卡阶段点击「不会读」（首次默写提交前有效）。
// PRD：学习中/需强化/已掌握 → 需强化，success_count 清零，next_due=明天。
// 已提交首次默写后（submission_locked）调用应被 handler 拒绝；本函数不做该判定。
func MarkHardBeforeSubmit(in WordLearning, today string) WordLearning {
	out := in
	out.LearningStatus = StatusReinforce
	out.SuccessCount = 0
	out.NextDueDate = AddDays(today, 1)
	out.LastStudiedDate = today
	out.StudiedDates = appendUnique(out.StudiedDates, today)
	return out
}

// EnterCard 首次进入单词卡：未学 → 学习中。
// PRD：抽中但尚未进入单词卡前状态仍是未学；首次进入该词单词卡时改为学习中。
// 幂等：非未学状态不改变。
func EnterCard(in WordLearning, today string) WordLearning {
	out := in
	if out.LearningStatus == StatusUnlearned {
		out.LearningStatus = StatusLearning
	}
	out.LastStudiedDate = today
	out.StudiedDates = appendUnique(out.StudiedDates, today)
	if out.NextDueDate == "" {
		out.NextDueDate = today
	}
	return out
}

// --- 小工具 ---

func appendUnique(slice []string, v string) []string {
	if !contains(slice, v) {
		slice = append(slice, v)
	}
	return slice
}

func contains(slice []string, v string) bool {
	for _, s := range slice {
		if s == v {
			return true
		}
	}
	return false
}

// JudgeAnswer 判定默写答案：correct / wrong / blank。比较前做大小写与首尾空白归一。
func JudgeAnswer(answer, expected string) string {
	a := normalizeWord(answer)
	if a == "" {
		return DictBlank
	}
	if a == normalizeWord(expected) {
		return DictCorrect
	}
	return DictWrong
}

func normalizeWord(s string) string {
	b := []byte(s)
	out := make([]byte, 0, len(b))
	for _, c := range b {
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			continue
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out = append(out, c)
	}
	return string(out)
}
