package wordlibrary

import "github.com/apodemakeles/niuniu-education/backend/internal/platform/ocr"

// DraftRowDTO 是草稿行，对齐前端 DraftRow 与架构文档 §5.3。
type DraftRowDTO struct {
	RowID      string   `json:"rowId"`
	Text       string   `json:"text"`
	MeaningZh  string   `json:"meaningZh"`
	Phonetic   string   `json:"phonetic"`
	WordType   string   `json:"wordType"`
	Confidence float64  `json:"confidence"`
	Issues     []string `json:"issues,omitempty"` // low_confidence / missing_meaning / duplicate
}

// OCRDraftResponse 是 /imports/ocr 的响应。
type OCRDraftResponse struct {
	ImageID string        `json:"imageId"`
	Rows    []DraftRowDTO `json:"rows"`
	RawText string        `json:"rawText,omitempty"`
}

// ConfirmImportRequest 是 /imports/confirm 的请求体。
type ConfirmImportRequest struct {
	Rows []ConfirmRow `json:"rows"`
}

// ConfirmRow 是确认入库的草稿行（经家长修正后）。
type ConfirmRow struct {
	Text      string `json:"text"`
	MeaningZh string `json:"meaningZh"`
	Phonetic  string `json:"phonetic"`
	WordType  string `json:"wordType"`
}

// ImportResultDetail 单行的入库结果。
type ImportResultDetail struct {
	RowID  string `json:"rowId,omitempty"`
	Text   string `json:"text"`
	Result string `json:"result"` // added / skipped / invalid
	Reason string `json:"reason,omitempty"`
}

// ImportResultResponse 是 /imports/confirm 的响应。
type ImportResultResponse struct {
	Added   int                  `json:"added"`
	Skipped int                  `json:"skipped"`
	Invalid int                  `json:"invalid"`
	Details []ImportResultDetail `json:"details"`
}

// fromOCRDraft 把 OCR provider 产出的草稿行转为 DTO（附加 issues 标记）。
func fromOCRDraft(rows []ocr.DraftRow) []DraftRowDTO {
	out := make([]DraftRowDTO, 0, len(rows))
	for i, r := range rows {
		d := DraftRowDTO{
			RowID:      draftRowID(i),
			Text:       r.Text,
			MeaningZh:  r.MeaningZh,
			Phonetic:   r.Phonetic,
			WordType:   orDefault(r.WordType, TypeNew),
			Confidence: r.Confidence,
		}
		var issues []string
		if d.Confidence > 0 && d.Confidence < 0.7 {
			issues = append(issues, "low_confidence")
		}
		if d.MeaningZh == "" {
			issues = append(issues, "missing_meaning")
		}
		if d.Text == "" {
			issues = append(issues, "invalid")
		}
		d.Issues = issues
		out = append(out, d)
	}
	return out
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func draftRowID(i int) string {
	return "draft-" + string(rune('a'+i))
}

// CreateWordRequest 是手动逐个录入的请求体。
type CreateWordRequest struct {
	Text      string `json:"text"`
	MeaningZh string `json:"meaningZh"`
	Phonetic  string `json:"phonetic"`
	WordType  string `json:"wordType"`
}

// UpdateWordRequest 是编辑单词属性的请求体（不含 text）。
type UpdateWordRequest struct {
	MeaningZh string `json:"meaningZh"`
	Phonetic  string `json:"phonetic"`
	WordType  string `json:"wordType"`
}

// DeleteWordResponse 是删除单词的响应。
type DeleteWordResponse struct {
	Kind string `json:"kind"` // physical / logical
}

// LibraryStats 是词库全局统计（不受列表筛选影响）。
type LibraryStats struct {
	Total        int `json:"total"`
	NewWords     int `json:"newWords"`
	MistakeWords int `json:"mistakeWords"`
}

// PaginationMeta 是分页元数据。
type PaginationMeta struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
	Total    int `json:"total"`
}

// WordListResponse 是 GET /words 的响应。
type WordListResponse struct {
	Data       []Word         `json:"data"`
	Pagination PaginationMeta `json:"pagination"`
	Stats      LibraryStats   `json:"stats"`
}
