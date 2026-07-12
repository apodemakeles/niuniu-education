package wordlibrary

// 单词模型，字段对齐 001_init.sql 的 words 表。
// 学习进度字段（FirstLearnedAt 等）留给孩子端，MVP 不写不展示。
type Word struct {
	ID             string `json:"id"`
	LibraryID      string `json:"-"`
	Text           string `json:"text"`
	MeaningZh      string `json:"meaningZh"`
	Phonetic       string `json:"phonetic"`
	WordType       string `json:"wordType"` // new / mistake
	Status         string `json:"status"`   // unlearned/learning/reinforce/mastered
	FirstLearnedAt string `json:"-"`
	LastReviewedAt string `json:"-"`
	ReviewCount    int    `json:"-"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
	LastEditedAt   string `json:"lastEditedAt"`
	Source         string `json:"source"` // manual/paste/photo
	DeletedAt      string `json:"-"`
}

// WordExample 是单词的 AI 学习例句。每词固定维护 3 条，displayOrder 为 0~2。
type WordExample struct {
	ID           string `json:"id"`
	WordID       string `json:"wordId"`
	Sentence     string `json:"sentence"`
	DisplayOrder int    `json:"displayOrder"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

// ExampleTarget 是批量生成时所需的最小单词信息。
type ExampleTarget struct {
	WordID    string
	Text      string
	MeaningZh string
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
	StatusUnlearned = "unlearned"
	StatusLearning  = "learning"
	StatusReinforce = "reinforce"
	StatusMastered  = "mastered"
)

// Source 取值
const (
	SourceManual = "manual"
	SourcePaste  = "paste"
	SourcePhoto  = "photo"
)

// DefaultStatusForType 仅用于兼容旧 words.status 列；实际状态由 word_learning 决定。
func DefaultStatusForType(wordType string) string {
	return StatusUnlearned
}
