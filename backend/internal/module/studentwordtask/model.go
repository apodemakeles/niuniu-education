package studentwordtask

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// 本包为「学生端背单词任务前台」业务模块（对齐 design/docs/student-word-task.md）。
// 与家长端 wordlibrary 模块并列：只读读取 words，自身管理学习/任务/阅读/打卡。

// --- 枚举（与 DB CHECK 约束、前端文案一一对应） ---

// LearningStatus 单词词组学习状态。
const (
	StatusUnlearned = "unlearned"
	StatusLearning  = "learning"
	StatusReinforce = "reinforce"
	StatusMastered  = "mastered"
)

// PoolType 任务池类型。
const (
	PoolFirst  = "first"  // 新词池
	PoolReview = "review" // 非新词池
)

// ReadStatus 单词卡听读状态。
const (
	ReadNotStarted = "not_started"
	ReadListened   = "listened"
	ReadDone       = "read_done"
	ReadHard       = "hard" // 不会读
)

// DictationResult 默写结果。
const (
	DictPending = "pending"
	DictCorrect = "correct"
	DictWrong   = "wrong"
	DictBlank   = "blank" // 未填写
)

// CorrectionResult 订正结果。
const (
	CorrectNotSubmitted = "not_submitted"
	CorrectCorrect      = "correct"
	CorrectWrong        = "wrong"
	CorrectBlank        = "blank"
)

// ReadingAIStatus AI 短文生成状态。
const (
	ReadingPending = "pending"
	ReadingSuccess = "success"
	ReadingFailed  = "failed"
)

// LoadDayType 复习负载日类型（首页解释当天任务量）。
const (
	LoadDayLight    = "light"    // 轻松日：达到入选分的非新词 < 3
	LoadDayNormal   = "normal"   // 普通日：3 ~ review_pool_max_count
	LoadDayPressure = "pressure" // 压力日：超过 review_pool_max_count，需按分截断
)

// 已掌握抽查阶段 -> 下次到期天数（长期保持：30/60/120/240）。
var masteredReviewDays = []int{30, 60, 120, 240}

// 巩固阶段需要完成 5 次首次默写正确：间隔依次为 1/3/7/14 天，
// 第 5 次成功后才进入长期抽查，避免短期连续答对就过早放到很远的未来。
const requiredSuccessfulRetrievals = 5

// --- 领域模型 ---

// WordLearning 单词词组学习主记录（word_learning 表）。
type WordLearning struct {
	WordID              string
	LibraryID           string
	LearningStatus      string
	SuccessCount        int
	StudiedDates        []string // JSON 解出
	SuccessDates        []string
	LastStudiedDate     string
	LastSuccessDate     string
	NextDueDate         string
	MasteredReviewIndex int
	FirstMasteredDate   string
	UpdatedAt           string
}

// DailyTask 今日任务明细（daily_tasks 表），每词每天一条。
type DailyTask struct {
	ID                   string
	TaskDate             string
	WordID               string
	LibraryID            string
	PoolType             string
	DisplayOrder         int
	ReadStatus           string
	FirstDictationAnswer string
	FirstDictationResult string
	UsedHint             bool
	SubmissionLocked     bool
	CorrectionAnswer     string
	CorrectionResult     string
	CreatedAt            string
	UpdatedAt            string
}

// ReadingPassage 延伸阅读 AI 短文（reading_passages 表），一天一条。
type ReadingPassage struct {
	TaskDate             string
	LibraryID            string
	ReadingTitle         string
	ReadingText          string
	CoveredWordIDs       []string       // JSON 解出
	WordOccurrenceCounts map[string]int // JSON 解出
	ReadingSceneHint     string
	AIGenerationStatus   string
	ReadingStartedAt     sql.NullString
	ReadingCompletedAt   sql.NullString
	ReadingMinSeconds    int
	CreatedAt            string
	UpdatedAt            string
}

// Checkin 打卡记录。
type Checkin struct {
	TaskDate  string
	LibraryID string
	CreatedAt string
}

// Strategy 排程策略参数（review_strategy 表单行）。
type Strategy struct {
	NewPoolCount           int
	ReviewPoolMaxCount     int
	ReviewPoolMinScore     int
	ReinforceBaseScore     int
	LearningBaseScore      int
	MasteredBaseScore      int
	DueTodayBonus          int
	OverdueDailyBonus      int
	OverdueBonusCap        int
	MasteredDueSampleRatio int // 百分比：1 表示 1%
	MasteredSampleStopDay  int
	ReviewOverdueReduceNew int // 逾期复习词达到该数时，新词收紧为 1 个
	ReviewOverduePauseNew  int // 逾期复习词达到该数时，暂停引入新词
	ReadingMinSeconds      int
}

// DefaultStrategy 返回对齐 PRD 模拟推荐参数的默认策略。
// 数据库已有 DEFAULT，这里用于内存测试与兜底。
func DefaultStrategy() Strategy {
	return Strategy{
		NewPoolCount:           3,
		ReviewPoolMaxCount:     9,
		ReviewPoolMinScore:     70,
		ReinforceBaseScore:     90,
		LearningBaseScore:      70,
		MasteredBaseScore:      10,
		DueTodayBonus:          40,
		OverdueDailyBonus:      8,
		OverdueBonusCap:        56,
		MasteredDueSampleRatio: 1,
		MasteredSampleStopDay:  120,
		ReviewOverdueReduceNew: 3,
		ReviewOverduePauseNew:  6,
		ReadingMinSeconds:      120,
	}
}

// --- 时间工具（统一使用本地自然日，避免时区抖动） ---

// Today 返回当前自然日的 YYYY-MM-DD。
func Today() string { return FormatDate(time.Now()) }

// FormatDate 把 time 格式化为 YYYY-MM-DD。
func FormatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// ParseDate 解析 YYYY-MM-DD（容错：仅日期部分）。
func ParseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("空日期")
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("解析日期 %q: %w", s, err)
	}
	return t, nil
}

// AddDays 在 YYYY-MM-DD 上加 n 天，返回 YYYY-MM-DD。
func AddDays(date string, n int) string {
	t, err := ParseDate(date)
	if err != nil {
		return date
	}
	return FormatDate(t.AddDate(0, 0, n))
}

// DaysBetween 返回 from 到 to 的天数差（to - from，可负）。
func DaysBetween(from, to string) int {
	a, err := ParseDate(from)
	if err != nil {
		return 0
	}
	b, err := ParseDate(to)
	if err != nil {
		return 0
	}
	return int(b.Sub(a).Hours() / 24)
}

// --- JSON 列编解码 ---

func encodeJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]" // 容错：默认空
	}
	return string(b)
}

func decodeStringSlice(s string) []string {
	if s == "" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return []string{}
	}
	return out
}

func decodeIntMap(s string) map[string]int {
	if s == "" {
		return map[string]int{}
	}
	out := map[string]int{}
	_ = json.Unmarshal([]byte(s), &out)
	return out
}
