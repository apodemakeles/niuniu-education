import { request } from './client'
import type { Word } from '@/types/word'

interface ListResponse {
  data: Word[]
}

// M0 联调用：获取单词列表。后续 M1 会扩展为完整 CRUD。
export function fetchWords(): Promise<Word[]> {
  return request<ListResponse>('/words').then((r) => r.data)
}
