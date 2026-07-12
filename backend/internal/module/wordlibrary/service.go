package wordlibrary

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/apodemakeles/niuniu-education/backend/internal/platform/ocr"
)

// Service 承载单词库业务规则。M1 最小子集：导入相关。
type Service struct {
	store  *Store
	ocr    ocr.Provider
	logger *slog.Logger
}

func NewService(store *Store, ocrProvider ocr.Provider, logger *slog.Logger) *Service {
	return &Service{store: store, ocr: ocrProvider, logger: logger}
}

// RecognizeForDraft 调 OCR provider 生成草稿行（不入库），返回 DTO 与原始文本。
func (s *Service) RecognizeForDraft(ctx context.Context, image []byte, mimeType string) (rows []DraftRowDTO, rawText string, err error) {
	res, err := s.ocr.Recognize(ctx, image, mimeType)
	if err != nil {
		return nil, "", err
	}
	return fromOCRDraft(res.Rows), res.RawText, nil
}

// ParsePasteForDraft 解析粘贴的文本为草稿行（不入库）。
// 支持格式（对齐 PRD 粘贴导入）：
//   - "apple 苹果"            （空格分隔）
//   - "apple,苹果,/ˈæpl/"     （逗号分隔）
//   - "apple /ˈæpl/ 苹果"     （含音标）
//
// 复用 ocr.ParseOCRText（它已支持这些分隔格式），只是不经过 OCR。
func (s *Service) ParsePasteForDraft(text string) []DraftRowDTO {
	rows := ocr.ParseOCRText(text)
	return fromOCRDraft(rows)
}

// ConfirmImport 确认草稿入库。
//
// 规则（对齐 PRD "确认入库前的校验"）：
//   - 英文单词为空：标记 invalid，不入库
//   - 重复（text+meaningZh 命中已存在记录）：默认跳过（skipped）
//   - 新词保持未学；明确标为易错词的条目会建立“需强化”学习记录
func (s *Service) ConfirmImport(ctx context.Context, req ConfirmImportRequest) (*ImportResultResponse, error) {
	resp := &ImportResultResponse{Details: []ImportResultDetail{}}

	for i, r := range req.Rows {
		rowID := draftRowID(i)
		// 校验：英文单词为空不能入库
		if r.Text == "" {
			resp.Invalid++
			resp.Details = append(resp.Details, ImportResultDetail{RowID: rowID, Text: r.Text, Result: "invalid", Reason: "英文单词为空"})
			continue
		}
		// 去重：text+meaningZh 命中则跳过
		exists, err := s.store.ExistsByTextMeaning(ctx, r.Text, r.MeaningZh)
		if err != nil {
			return nil, fmt.Errorf("查重失败: %w", err)
		}
		if exists {
			resp.Skipped++
			resp.Details = append(resp.Details, ImportResultDetail{RowID: rowID, Text: r.Text, Result: "skipped", Reason: "词库内已存在"})
			continue
		}
		// 入库；易错词的“需强化”状态由 Store 写入 word_learning。
		_, err = s.store.CreateWord(ctx, CreateWordParams{
			Text:      r.Text,
			MeaningZh: r.MeaningZh,
			Phonetic:  r.Phonetic,
			WordType:  orDefault(r.WordType, TypeNew),
			Source:    SourcePhoto,
		})
		if err != nil {
			resp.Invalid++
			resp.Details = append(resp.Details, ImportResultDetail{RowID: rowID, Text: r.Text, Result: "invalid", Reason: err.Error()})
			continue
		}
		resp.Added++
		resp.Details = append(resp.Details, ImportResultDetail{RowID: rowID, Text: r.Text, Result: "added"})
	}
	return resp, nil
}
