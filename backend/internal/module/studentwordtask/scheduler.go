package studentwordtask

import (
	"crypto/rand"
	"math/big"
	"sort"
	"time"
)

// scheduler.go 实现每日任务生成：先保障到期复习，再公平引入新词。

// CandidateWord 是任务生成的中间结构。
type CandidateWord struct {
	Word     WordInfo
	Learning WordLearning
}

// PlanResult 是当天任务计划。
type PlanResult struct {
	FirstPool            []CandidateWord
	ReviewPool           []CandidateWord
	ReviewPoolCapacity   int // 基础复习容量加上空出的新词名额
	LoadDay              string
	ReviewQualifiedCount int
}

// BuildTodayPlan 生成每日任务计划。新词按入库时间公平排队；逾期复习积压时收紧新词。
func BuildTodayPlan(
	today, termStartDate string,
	strategy Strategy,
	words []WordInfo,
	learnings []WordLearning,
	randFn func(max int) int,
) PlanResult {
	if randFn == nil {
		randFn = cryptoRandInt
	}

	learningByID := make(map[string]WordLearning, len(learnings))
	for _, l := range learnings {
		learningByID[l.WordID] = l
	}

	termDay := 1
	if termStartDate != "" {
		termDay = DaysBetween(termStartDate, today) + 1
		if termDay < 1 {
			termDay = 1
		}
	}
	stopMasteredSampling := termDay > strategy.MasteredSampleStopDay

	var unlearned []CandidateWord
	type scored struct {
		c           CandidateWord
		score       int
		overdueDays int
	}
	var candidates []scored
	overdueReviewCount := 0

	for _, w := range words {
		l, ok := learningByID[w.ID]
		status := w.Status
		if ok {
			status = l.LearningStatus
		}
		if status == StatusUnlearned {
			unlearned = append(unlearned, CandidateWord{Word: w, Learning: l})
			continue
		}

		switch status {
		case StatusLearning, StatusReinforce:
			if l.NextDueDate == "" || l.NextDueDate > today {
				continue
			}
			if l.NextDueDate < today {
				overdueReviewCount++
			}
		case StatusMastered:
			if stopMasteredSampling || l.NextDueDate == "" || l.NextDueDate > today {
				continue
			}
			if randFn(100) >= strategy.MasteredDueSampleRatio {
				continue
			}
		}

		score := computeReviewScore(status, l.NextDueDate, today, strategy)
		if score < strategy.ReviewPoolMinScore {
			continue
		}
		candidates = append(candidates, scored{
			c:           CandidateWord{Word: w, Learning: l},
			score:       score,
			overdueDays: maxInt(0, DaysBetween(l.NextDueDate, today)),
		})
	}
	qualifiedCount := len(candidates)

	// 最久未学的新词优先；created_at 相同再按 ID 稳定排序，保证一定会轮到。
	newQuota := newPoolQuota(strategy, overdueReviewCount)
	sort.SliceStable(unlearned, func(i, j int) bool {
		ci, cj := unlearned[i].Word.CreatedAt, unlearned[j].Word.CreatedAt
		if ci != cj {
			if ci == "" {
				return false
			}
			if cj == "" {
				return true
			}
			return ci < cj
		}
		return unlearned[i].Word.ID < unlearned[j].Word.ID
	})
	if len(unlearned) > newQuota {
		unlearned = unlearned[:newQuota]
	}

	// 空出的新词名额只追加给需强化词，单日总量仍不超过原来的 3+9。
	transferredSlots := maxInt(0, strategy.NewPoolCount-len(unlearned))
	reviewCapacity := strategy.ReviewPoolMaxCount + transferredSlots

	// 逾期最久的词先回收，之后才比较状态分，避免低状态词被强化词长期挤压。
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].overdueDays != candidates[j].overdueDays {
			return candidates[i].overdueDays > candidates[j].overdueDays
		}
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		di, dj := candidates[i].c.Learning.NextDueDate, candidates[j].c.Learning.NextDueDate
		if di != dj {
			return di < dj
		}
		return candidates[i].c.Word.ID < candidates[j].c.Word.ID
	})

	baseCount := minInt(len(candidates), strategy.ReviewPoolMaxCount)
	reviewPool := make([]CandidateWord, 0, reviewCapacity)
	for _, c := range candidates[:baseCount] {
		reviewPool = append(reviewPool, c.c)
	}
	for _, c := range candidates[baseCount:] {
		if len(reviewPool) >= reviewCapacity {
			break
		}
		if c.c.Learning.LearningStatus == StatusReinforce {
			reviewPool = append(reviewPool, c.c)
		}
	}

	return PlanResult{
		FirstPool:            unlearned,
		ReviewPool:           reviewPool,
		ReviewPoolCapacity:   reviewCapacity,
		LoadDay:              classifyLoadDay(qualifiedCount, reviewCapacity),
		ReviewQualifiedCount: qualifiedCount,
	}
}

func newPoolQuota(s Strategy, overdueReviewCount int) int {
	if s.ReviewOverduePauseNew > 0 && overdueReviewCount >= s.ReviewOverduePauseNew {
		return 0
	}
	if s.ReviewOverdueReduceNew > 0 && overdueReviewCount >= s.ReviewOverdueReduceNew {
		return minInt(1, s.NewPoolCount)
	}
	return s.NewPoolCount
}

// computeReviewScore 复习分 = 状态基础分 + 到期加分 + 逾期加分。
func computeReviewScore(status, dueDate, today string, s Strategy) int {
	base := 0
	switch status {
	case StatusReinforce:
		base = s.ReinforceBaseScore
	case StatusLearning:
		base = s.LearningBaseScore
	case StatusMastered:
		base = s.MasteredBaseScore
	}
	bonus := 0
	if dueDate != "" && dueDate <= today {
		bonus += s.DueTodayBonus
		overdue := DaysBetween(dueDate, today)
		if overdue > 0 {
			ob := overdue * s.OverdueDailyBonus
			if ob > s.OverdueBonusCap {
				ob = s.OverdueBonusCap
			}
			bonus += ob
		}
	}
	return base + bonus
}

func classifyLoadDay(qualifiedCount, reviewCapacity int) string {
	switch {
	case qualifiedCount < 3:
		return LoadDayLight
	case qualifiedCount > reviewCapacity:
		return LoadDayPressure
	default:
		return LoadDayNormal
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func cryptoRandInt(max int) int {
	if max <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return int(time.Now().UnixNano()) % max
	}
	return int(n.Int64())
}

// AllocateDisplayOrder 为今日任务分配 display_order：新词池在前，非新词池在后。
func AllocateDisplayOrder(firstPool, reviewPool []CandidateWord) (firstOrders, reviewOrders map[string]int) {
	firstOrders = map[string]int{}
	reviewOrders = map[string]int{}
	for i, c := range firstPool {
		firstOrders[c.Word.ID] = i
	}
	offset := len(firstPool)
	for i, c := range reviewPool {
		reviewOrders[c.Word.ID] = offset + i
	}
	return
}
