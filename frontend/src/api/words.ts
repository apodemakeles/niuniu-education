import { request } from './client'
import type { Word, WordType } from '@/types/word'

export interface LibraryStats {
  total: number
  newWords: number
  mistakeWords: number
}

export interface PaginationMeta {
  page: number
  pageSize: number
  total: number
}

export interface WordListResult {
  data: Word[]
  pagination: PaginationMeta
  stats: LibraryStats
}

export interface WordListParams {
  type?: 'all' | 'new' | 'mistake'
  status?: string
  q?: string
  page?: number
  pageSize?: number
}

interface RawWordListResponse {
  data?: Word[] | null
  pagination?: Partial<PaginationMeta> | null
  stats?: Partial<LibraryStats> | null
}

function normalizeListResult(raw: RawWordListResponse, params?: WordListParams): WordListResult {
  const data = raw.data ?? []
  const page = raw.pagination?.page ?? params?.page ?? 1
  const pageSize = raw.pagination?.pageSize ?? params?.pageSize ?? 20
  const total = raw.pagination?.total ?? data.length
  return {
    data,
    pagination: { page, pageSize, total },
    stats: {
      total: raw.stats?.total ?? total,
      newWords: raw.stats?.newWords ?? 0,
      mistakeWords: raw.stats?.mistakeWords ?? 0,
    },
  }
}

// 单词列表（支持 type/status/q 筛选与分页；q 为英文/中文/音标前缀匹配）。
export function fetchWords(params?: WordListParams): Promise<WordListResult> {
  const query: Record<string, string | number | undefined> = { ...params }
  if (query.type === 'all') delete query.type
  if (query.status === 'all') delete query.status
  if (!query.q) delete query.q
  return request<RawWordListResponse>('/words', { query }).then((r) => normalizeListResult(r, params))
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
