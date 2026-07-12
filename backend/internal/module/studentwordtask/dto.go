package studentwordtask

// dto.go 对齐前端类型。与前端 src/types/practice.ts 一一对应。

// MissionWord 今日任务首页/单词卡的单词展示。
type MissionWord struct {
	ID            string `json:"id"`
	Text          string `json:"text"`
	MeaningZh     string `json:"meaningZh"`
	Phonetic      string `json:"phonetic"`
	WordType      string `json:"wordType"`      // new / mistake（展示标签）
	TypeLabel     string `json:"typeLabel"`     // 新词 / 易错词
	LearningLabel string `json:"learningLabel"` // 未学/学习中/需强化/已掌握
	Stage         string `json:"stage"`         // 复习阶段文案：初学/1天复习 等
	PoolType      string `json:"poolType"`      // first / review
	DisplayOrder  int    `json:"displayOrder"`
	ReadStatus    string `json:"readStatus"` // not_started/listened/read_done/hard
}

// TodayResponse GET /practice/today 响应。
type TodayResponse struct {
	Date          string        `json:"date"`         // YYYY-MM-DD
	LoadDay       string        `json:"loadDay"`      // light/normal/pressure
	LoadDayLabel  string        `json:"loadDayLabel"` // 轻松日/普通日/压力日
	LoadDayDesc   string        `json:"loadDayDesc"`  // 解释文案
	FirstPool     []MissionWord `json:"firstPool"`
	ReviewPool    []MissionWord `json:"reviewPool"`
	NewPoolCount  int           `json:"newPoolCount"`  // 策略上限
	ReviewPoolMax int           `json:"reviewPoolMax"` // 策略上限
	TotalCount    int           `json:"totalCount"`
	Empty         bool          `json:"empty"` // 词库为空
	EmptyHint     string        `json:"emptyHint,omitempty"`
	Completed     bool          `json:"completed"` // 当日任务已完成（含必要订正）
}

// CardDetailResponse GET 单词卡 / POST enter 后返回当前词卡片信息。
type CardDetailResponse struct {
	Word           MissionWord `json:"word"`
	CardIndex      int         `json:"cardIndex"` // 0 起
	Total          int         `json:"total"`
	Example        string      `json:"example"`        // AI 生成例句；缺失时才回退占位
	ExampleMissing bool        `json:"exampleMissing"` // 标记「例句待补充」
	Listened       bool        `json:"listened"`       // 当前词是否已听过音
	ReadDone       bool        `json:"readDone"`       // 当前词是否已读完
	IsFirstCard    bool        `json:"isFirstCard"`
	IsLastCard     bool        `json:"isLastCard"`
}

// ReadingResponse GET /practice/reading 响应。
type ReadingResponse struct {
	Date           string        `json:"date"`
	Title          string        `json:"title"`
	Text           string        `json:"text"`    // 已高亮 HTML（<mark> 包裹目标词）
	RawText        string        `json:"rawText"` // 原文（前端用于听读音等）
	SceneHint      string        `json:"sceneHint"`
	CoveredWords   []CoveredWord `json:"coveredWords"`
	Appearances    []Appearance  `json:"appearances"` // 目标词出现次数列表
	Status         string        `json:"status"`      // pending/success/failed
	MinSeconds     int           `json:"minSeconds"`
	StartedAt      string        `json:"startedAt,omitempty"`
	CompletedAt    string        `json:"completedAt,omitempty"`
	ElapsedSeconds int           `json:"elapsedSeconds"` // 已停留秒数（从 started_at 算）
	CanFinish      bool          `json:"canFinish"`      // 是否满 minSeconds（或调试模式）
	DebugMode      bool          `json:"debugMode"`      // 调试模式：可跳过阅读最短停留
}

// CoveredWord 短文覆盖的今日任务词。
type CoveredWord struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	MeaningZh string `json:"meaningZh"`
}

// Appearance 目标词出现次数。
type Appearance struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Count int    `json:"count"`
}

// DictationResponse GET /practice/dictation 默写题面。
type DictationResponse struct {
	Date  string          `json:"date"`
	Items []DictationItem `json:"items"`
}

// DictationItem 单个默写题。
type DictationItem struct {
	TaskID           string `json:"taskId"`
	WordID           string `json:"wordId"`
	MeaningZh        string `json:"meaningZh"`
	Phonetic         string `json:"phonetic,omitempty"` // 默认隐藏，前端按 toggle 决定是否显示
	PoolType         string `json:"poolType"`
	FirstAnswer      string `json:"firstAnswer,omitempty"`
	FirstResult      string `json:"firstResult,omitempty"` // pending/correct/wrong/blank
	Locked           bool   `json:"locked"`
	CorrectionAnswer string `json:"correctionAnswer,omitempty"`
	CorrectionResult string `json:"correctionResult,omitempty"`
	BonusEligible    bool   `json:"bonusEligible"` // 新词池未学词，首次提交正确可触发 bonus
	BonusAwarded     bool   `json:"bonusAwarded"`
}

// SubmitDictationRequest POST /practice/dictation/submit。
type SubmitDictationRequest struct {
	Answers []SubmitAnswer `json:"answers"`
}

// SubmitAnswer 单词的默写答案。
type SubmitAnswer struct {
	TaskID string `json:"taskId"`
	WordID string `json:"wordId"`
	Answer string `json:"answer"`
}

// SubmitDictationResponse 提交默写结果。
type SubmitDictationResponse struct {
	Items    []DictationItem `json:"items"`
	Correct  int             `json:"correct"`
	Wrong    int             `json:"wrong"`
	Blank    int             `json:"blank"`
	BonusIDs []string        `json:"bonusIds"` // 触发 bonus 的 wordId 列表
}

// CorrectDictationRequest POST /practice/dictation/correct。
type CorrectDictationRequest struct {
	Answers []SubmitAnswer `json:"answers"`
}

// DoneResponse GET /practice/done 完成页数据。
type DoneResponse struct {
	Date            string         `json:"date"`
	Total           int            `json:"total"`
	CorrectCount    int            `json:"correctCount"`
	WrongCount      int            `json:"wrongCount"`
	BlankCount      int            `json:"blankCount"`
	UsedHintCount   int            `json:"usedHintCount"`
	TomorrowReview  []MissionWord  `json:"tomorrowReview"`  // 明天优先复习词
	NeedReinforce   []MissionWord  `json:"needReinforce"`   // 仍需强化词
	DictationLocked bool           `json:"dictationLocked"` // 首次默写是否已提交锁定
	HasCorrection   bool           `json:"hasCorrection"`
	Checkin         CheckinSummary `json:"checkin"`
}

// CheckinSummary 打卡概要。
type CheckinSummary struct {
	MonthLabel  string `json:"monthLabel"`  // 如「7 月」
	CheckedDays []int  `json:"checkedDays"` // 本月已打卡的日期号（1~31）
	TodayDay    int    `json:"todayDay"`
	Streak      int    `json:"streak"`    // 连续打卡天数
	Bonus7Day   bool   `json:"bonus7Day"` // 本次打卡是否触发连续 7 天
}

// CheckinResponse POST /practice/checkin 响应。
type CheckinResponse struct {
	Done    bool           `json:"done"`
	Summary CheckinSummary `json:"summary"`
}
