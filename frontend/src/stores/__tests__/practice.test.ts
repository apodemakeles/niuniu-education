import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { ApiError } from '@/api/client'
import type { DictationResponse, ReadingResponse, SubmitDictationResponse } from '@/types/practice'

vi.mock('@/api/practice', () => ({
  fetchToday: vi.fn(),
  enterCard: vi.fn(),
  markListened: vi.fn(),
  markReadDone: vi.fn(),
  markHard: vi.fn(),
  fetchReading: vi.fn(),
  regenerateReading: vi.fn(),
  completeReading: vi.fn(),
  fetchDictation: vi.fn(),
  submitDictation: vi.fn(),
  correctDictation: vi.fn(),
  fetchDone: vi.fn(),
  checkin: vi.fn(),
  fetchPronunciation: vi.fn(),
}))

import { completeReading, fetchDictation, fetchReading, submitDictation } from '@/api/practice'
import { usePracticeStore } from '../practice'

const serverReading: ReadingResponse = {
  date: '2026-07-12',
  title: 'A Day at School',
  text: 'A short story.',
  rawText: 'A short story.',
  sceneHint: '',
  coveredWords: [],
  appearances: [],
  status: 'success',
  minSeconds: 120,
  startedAt: '2026-07-12 00:00:00',
  elapsedSeconds: 42,
  canFinish: false,
}

const lockedDictation: DictationResponse = {
  date: '2026-07-12',
  items: [{
    taskId: 'task-1',
    wordId: 'word-1',
    meaningZh: '书桌',
    poolType: 'first',
    firstAnswer: 'desl',
    firstResult: 'wrong',
    locked: true,
    correctionResult: 'not_submitted',
    bonusEligible: false,
    bonusAwarded: false,
  }],
}

const submitWrong: SubmitDictationResponse = {
  items: lockedDictation.items,
  correct: 0,
  wrong: 1,
  blank: 0,
  bonusIds: [],
}

describe('practice store 阅读完成', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setActivePinia(createPinia())
  })

  it('服务端判定时长不足后重新加载阅读数据校准', async () => {
    vi.mocked(completeReading).mockRejectedValue(
      new ApiError(409, 'READING_TOO_SHORT', '继续阅读一会儿再完成'),
    )
    vi.mocked(fetchReading).mockResolvedValue(serverReading)
    const store = usePracticeStore()

    expect(await store.finishReading()).toBe(false)
    expect(fetchReading).toHaveBeenCalledOnce()
    expect(store.reading?.elapsedSeconds).toBe(42)
    expect(store.error).toBe('继续阅读一会儿再完成')
  })

  it('首次默写提交后不提前解锁完成页，交由页面决定是否需要订正', async () => {
    vi.mocked(submitDictation).mockResolvedValue(submitWrong)
    vi.mocked(fetchDictation).mockResolvedValue(lockedDictation)
    const store = usePracticeStore()

    const resp = await store.submit([{ taskId: 'task-1', wordId: 'word-1', answer: 'desl' }])

    expect(resp?.wrong).toBe(1)
    expect(store.unlockedViews.has('done')).toBe(false)
    expect(store.dictation).toEqual(lockedDictation)
  })

  it('重新开始任务会清空已解锁的完成流程，避免沿用上一轮导航状态', () => {
    const store = usePracticeStore()
    store.unlock('cards')
    store.unlock('done')
    store.switchView('done')

    store.resetPracticeSession()

    expect(store.view).toBe('dashboard')
    expect([...store.unlockedViews]).toEqual(['dashboard'])
    expect(store.done).toBeNull()
  })
})
