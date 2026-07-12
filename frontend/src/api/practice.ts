// 学生端背单词任务前台的 API 封装。统一走 client.ts 的 request<T>。

import { request } from './client'
import type {
  TodayResponse,
  CardDetailResponse,
  ReadingResponse,
  DictationResponse,
  SubmitAnswer,
  SubmitDictationResponse,
  DoneResponse,
  CheckinResponse,
  PronunciationResponse,
} from '@/types/practice'

export function fetchPronunciation(wordId: string): Promise<PronunciationResponse> {
  return request<PronunciationResponse>(`/pronunciations/${wordId}`)
}

// GET /practice/today 今日任务首页
export function fetchToday(): Promise<TodayResponse> {
  return request<TodayResponse>('/practice/today')
}

// POST /practice/cards/:wordId/enter 首次进入单词卡（未学→学习中）
export function enterCard(wordId: string): Promise<CardDetailResponse> {
  return request<CardDetailResponse>(`/practice/cards/${wordId}/enter`, { method: 'POST' })
}

// POST /practice/cards/:wordId/listen 标记已听音
export function markListened(wordId: string): Promise<CardDetailResponse> {
  return request<CardDetailResponse>(`/practice/cards/${wordId}/listen`, { method: 'POST' })
}

// POST /practice/cards/:wordId/read-done 标记我读完了
export function markReadDone(wordId: string): Promise<CardDetailResponse> {
  return request<CardDetailResponse>(`/practice/cards/${wordId}/read-done`, { method: 'POST' })
}

// POST /practice/cards/:wordId/hard 标记不会读
export function markHard(wordId: string): Promise<CardDetailResponse> {
  return request<CardDetailResponse>(`/practice/cards/${wordId}/hard`, { method: 'POST' })
}

// GET /practice/reading 取今日短文（无则触发生成）
export function fetchReading(): Promise<ReadingResponse> {
  return request<ReadingResponse>('/practice/reading')
}

// POST /practice/reading/regenerate 重新生成短文
export function regenerateReading(): Promise<ReadingResponse> {
  return request<ReadingResponse>('/practice/reading/regenerate', { method: 'POST' })
}

// POST /practice/reading/complete 完成阅读（校验停留时长）
export function completeReading(): Promise<{ done: boolean }> {
  return request<{ done: boolean }>('/practice/reading/complete', { method: 'POST' })
}

// GET /practice/dictation 取默写题面
export function fetchDictation(): Promise<DictationResponse> {
  return request<DictationResponse>('/practice/dictation')
}

// POST /practice/dictation/submit 首次默写提交
export function submitDictation(answers: SubmitAnswer[]): Promise<SubmitDictationResponse> {
  return request<SubmitDictationResponse>('/practice/dictation/submit', {
    method: 'POST',
    body: JSON.stringify({ answers }),
  })
}

// POST /practice/dictation/correct 订正提交
export function correctDictation(answers: SubmitAnswer[]): Promise<SubmitDictationResponse> {
  return request<SubmitDictationResponse>('/practice/dictation/correct', {
    method: 'POST',
    body: JSON.stringify({ answers }),
  })
}

// GET /practice/done 完成页数据
export function fetchDone(): Promise<DoneResponse> {
  return request<DoneResponse>('/practice/done')
}

// POST /practice/checkin 打卡
export function checkin(): Promise<CheckinResponse> {
  return request<CheckinResponse>('/practice/checkin', { method: 'POST' })
}
