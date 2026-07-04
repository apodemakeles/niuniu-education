import { request } from './client'
import type { OCRDraftResponse, ConfirmRow, ImportResult, DraftRow } from '@/types/draft'

// 上传图片执行 OCR，返回草稿行（不入库）。
export function recognizeImage(file: File): Promise<OCRDraftResponse> {
  const form = new FormData()
  form.append('image', file)
  // multipart 不设 Content-Type，让浏览器自动带 boundary
  return request<OCRDraftResponse>('/imports/ocr', {
    method: 'POST',
    body: form,
    headers: {}, // 覆盖默认 application/json
  })
}

// 解析粘贴的文本为草稿行（不入库）。
export function parsePaste(text: string): Promise<DraftRow[]> {
  return request<{ rows: DraftRow[] }>('/imports/parse', {
    method: 'POST',
    body: JSON.stringify({ text }),
  }).then((r) => r.rows)
}

// 确认草稿入库。
export function confirmImport(rows: ConfirmRow[]): Promise<ImportResult> {
  return request<ImportResult>('/imports/confirm', {
    method: 'POST',
    body: JSON.stringify({ rows }),
  })
}
