import { request } from './client'

export interface DictationItem {
  index: number
  meaning: string
}

export interface DictationPreview {
  title: string
  items: DictationItem[]
}

// 获取默写表预览数据（前端渲染预览表）。
export function previewDictation(scope: string, title?: string): Promise<DictationPreview> {
  return request<DictationPreview>('/exports/dictation:preview', {
    method: 'POST',
    body: JSON.stringify({ scope, title }),
  })
}

// 生成并下载 Word 文件。items 来自预览阶段（中文可能被临时编辑过，不回写词库）。
// 返回 Blob 供前端触发下载。
export async function downloadDictation(title: string, items: DictationItem[]): Promise<Blob> {
  const res = await fetch(`${import.meta.env.VITE_API_BASE}/exports/dictation`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title, items }),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(`导出失败 (${res.status}) ${text}`)
  }
  return res.blob()
}
