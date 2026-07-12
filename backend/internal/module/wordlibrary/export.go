package wordlibrary

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"strings"
)

// DictationItem 是默写表的一行（序号 + 中文）。
type DictationItem struct {
	Index   int    `json:"index"`
	Meaning string `json:"meaning"`
}

// DictationPreview 是导出预览数据，供前端渲染默写表。
type DictationPreview struct {
	Title string          `json:"title"`
	Items []DictationItem `json:"items"`
}

// scopeStatus 把导出范围映射到 status 值。
func scopeStatus(scope string) string {
	switch scope {
	case "unlearned", "learning", "reinforce", "mastered":
		return scope
	default:
		return "all"
	}
}

// BuildDictationPreview 按导出范围生成默写表预览数据。
func (h *Handler) BuildDictationPreview(ctx context.Context, scope, title string) (*DictationPreview, error) {
	words, err := h.store.List(ctx, ListParams{Status: scopeStatus(scope), PageSize: 0})
	if err != nil {
		return nil, err
	}
	items := make([]DictationItem, 0, len(words.Words))
	for i, w := range words.Words {
		items = append(items, DictationItem{Index: i + 1, Meaning: w.MeaningZh})
	}
	if title == "" {
		title = "单词默写练习"
	}
	return &DictationPreview{Title: title, Items: items}, nil
}

// BuildDictationDoc 生成 Word 兼容的 HTML 内容（application/msword），含预览阶段可临时编辑的中文。
// items 由前端传入（预览时家长可能临时修改过中文，且不回写词库）。
func BuildDictationDoc(title string, items []DictationItem) string {
	var rows strings.Builder
	for _, it := range items {
		rows.WriteString(fmt.Sprintf(
			`<tr><td>%d</td><td>%s</td><td class="answer"></td></tr>`,
			it.Index, html.EscapeString(it.Meaning),
		))
	}
	safeTitle := html.EscapeString(title)
	return fmt.Sprintf(`<!doctype html>
<html>
  <head>
    <meta charset="utf-8" />
    <title>%s</title>
    <style>
      body { font-family: "Microsoft YaHei", sans-serif; color: #111; }
      h1 { text-align: center; font-size: 22px; margin: 12px 0 18px; }
      .meta { display: flex; justify-content: space-between; margin-bottom: 14px; font-size: 14px; }
      table { width: 100%%; border-collapse: collapse; table-layout: fixed; }
      th, td { border: 1px solid #222; padding: 9px; font-size: 14px; }
      th:nth-child(1), td:nth-child(1) { width: 52px; text-align: center; }
      th:nth-child(2), td:nth-child(2) { width: 34%%; }
      td.answer { height: 34px; }
    </style>
  </head>
  <body>
    <h1>%s</h1>
    <div class="meta"><span>姓名：____________</span><span>日期：____________</span></div>
    <table>
      <thead><tr><th>序号</th><th>中文</th><th>英文默写</th></tr></thead>
      <tbody>
        %s
      </tbody>
    </table>
  </body>
</html>`, safeTitle, safeTitle, rows.String())
}

// parseExportItems 解析前端传入的预览数据（家长可能临时改过中文）。
func parseExportItems(rawItems []map[string]any) []DictationItem {
	items := make([]DictationItem, 0, len(rawItems))
	for i, m := range rawItems {
		meaning, _ := m["meaning"].(string)
		if idx, ok := m["index"].(float64); ok {
			items = append(items, DictationItem{Index: int(idx), Meaning: meaning})
		} else {
			items = append(items, DictationItem{Index: i + 1, Meaning: meaning})
		}
	}
	return items
}

// filenameFromTitle 由标题生成安全的下载文件名。
func filenameFromTitle(title string) string {
	s := strings.TrimSpace(title)
	if s == "" {
		s = "单词默写练习"
	}
	// URL 编码文件名，避免中文乱码
	return url.QueryEscape(s) + ".doc"
}
