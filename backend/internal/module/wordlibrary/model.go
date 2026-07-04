package wordlibrary

// 单词模型，字段对齐 001_init.sql 的 words 表。
// 学习进度字段（FirstLearnedAt 等）留给孩子端，MVP 不写不展示。
type Word struct {
	ID             string `json:"id"`
	LibraryID      string `json:"-"`
	Text           string `json:"text"`
	MeaningZh      string `json:"meaningZh"`
	Phonetic       string `json:"phonetic"`
	WordType       string `json:"wordType"`  // new / mistake
	Status         string `json:"status"`    // unlearned/learning/reinforce/mastered
	FirstLearnedAt string `json:"-"`
	LastReviewedAt string `json:"-"`
	ReviewCount    int    `json:"-"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
	LastEditedAt   string `json:"lastEditedAt"`
	Source         string `json:"source"` // manual/paste/photo
	DeletedAt      string `json:"-"`
}

// 固定词库 ID（MVP 唯一词库，迁移脚本中预置）。
const MainLibraryID = "main-library"

// WordType 取值
const (
	TypeNew     = "new"
	TypeMistake = "mistake"
)

// Status 取值
const (
	StatusUnlearned  = "unlearned"
	StatusLearning   = "learning"
	StatusReinforce  = "reinforce"
	StatusMastered   = "mastered"
)

// Source 取值
const (
	SourceManual = "manual"
	SourcePaste  = "paste"
	SourcePhoto  = "photo"
)

// DefaultStatusForType 按原型 app.js 规则：新词默认未学，易错词默认需强化。
func DefaultStatusForType(wordType string) string {
	if wordType == TypeMistake {
		return StatusReinforce
	}
	return StatusUnlearned
}
