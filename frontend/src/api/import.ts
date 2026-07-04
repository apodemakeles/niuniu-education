import { request, streamSSE } from './client'
import type { OCRDraftResponse, ConfirmRow, ImportResult, DraftRow } from '@/types/draft'

// 上传图片执行 OCR，返回草稿行（不入库）。非流式版本（向后兼容）。
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

// 流式 OCR 回调接口
export interface StreamCallbacks {
  onStage?: (stage: string) => void
  onRow?: (row: DraftRow) => void
  onFinal?: (result: { imageId: string; rows: DraftRow[]; total: number }) => void
  onError?: (message: string) => void
}

/**
 * 流式识别图片：SSE 推送进度阶段 + 增量词行 + 最终结果。
 * 返回 AbortController 用于取消。
 */
export function recognizeImageStream(file: File, cb: StreamCallbacks): AbortController {
  const controller = new AbortController()
  const form = new FormData()
  form.append('image', file)

  // 每个增量 row 需要补 rowId（后端增量事件不带）
  let rowIndex = 0

  streamSSE(
    '/imports/ocr/stream',
    { method: 'POST', body: form, headers: {} },
    (ev) => {
      switch (ev.event) {
        case 'stage':
          cb.onStage?.(ev.data.stage)
          break
        case 'row':
          cb.onRow?.({ ...ev.data, rowId: `stream-${rowIndex++}`, issues: [] })
          break
        case 'final':
          cb.onFinal?.(ev.data)
          break
        case 'error':
          cb.onError?.(ev.data.message || '识别失败')
          break
      }
    },
    controller.signal,
  ).catch((e) => {
    if ((e as Error).name !== 'AbortError') {
      cb.onError?.((e as Error).message)
    }
  })

  return controller
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
