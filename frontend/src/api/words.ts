import { request } from './client'
import type { Word, WordType, WordStatus } from '@/types/word'

interface ListResponse {
  data: Word[]
}

// 单词列表（支持 type/status/q 筛选）。
export function fetchWords(params?: {
  type?: 'all' | 'new' | 'mistake'
  status?: string
  q?: string
}): Promise<Word[]> {
  return request<ListResponse>('/words', { query: params }).then((r) => r.data)
}

// 手动逐个录入。重复时后端返回 409，由调用方处理。
export function createWord(input: {
  text: string
  meaningZh: string
  phonetic?: string
  wordType: WordType
}, force = false): Promise<Word> {
  const forceParam = force ? '?force=1' : ''
  return request<Word>(`/words${forceParam}`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

// 编辑单词属性（不含 text）。
export function updateWord(id: string, input: {
  meaningZh: string
  phonetic: string
  wordType: WordType
  status: WordStatus
}): Promise<Word> {
  return request<Word>(`/words/${id}`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

// 删除单词。返回 kind: physical | logical。
export function deleteWord(id: string): Promise<{ kind: 'physical' | 'logical' }> {
  return request<{ kind: 'physical' | 'logical' }>(`/words/${id}`, { method: 'DELETE' })
}
