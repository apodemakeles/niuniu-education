import { request, streamSSE } from './client'

export interface WordExample {
  id: string
  wordId: string
  sentence: string
  displayOrder: number
  createdAt: string
  updatedAt: string
}

export interface ExampleProgress {
  current: number
  total: number
  wordId?: string
  text?: string
  status: 'generating' | 'success' | 'failed'
  message?: string
}

export function fetchExamples(wordId: string): Promise<WordExample[]> {
  return request<{ examples: WordExample[] }>(`/words/${wordId}/examples`).then((r) => r.examples)
}

export function generateExamples(wordId: string): Promise<WordExample[]> {
  return request<{ examples: WordExample[] }>(`/words/${wordId}/examples/generate`, { method: 'POST' }).then((r) => r.examples)
}

export function regenerateExample(wordId: string, exampleId: string): Promise<WordExample> {
  return request<WordExample>(`/words/${wordId}/examples/${exampleId}/regenerate`, { method: 'POST' })
}

export function streamGenerateExamples(
  path: '/examples/generate/stream' | '/examples/backfill/stream',
  wordIds: string[] | undefined,
  onStart: (total: number) => void,
  onProgress: (progress: ExampleProgress) => void,
): Promise<void> {
  const body = wordIds ? JSON.stringify({ wordIds }) : undefined
  return streamSSE(path, { method: 'POST', body }, (ev) => {
    if (ev.event === 'start') onStart(Number(ev.data.total) || 0)
    if (ev.event === 'progress') onProgress(ev.data as ExampleProgress)
  })
}
