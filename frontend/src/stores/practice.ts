import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  fetchToday,
  enterCard,
  markListened,
  markReadDone,
  markHard,
  fetchReading,
  regenerateReading,
  completeReading,
  fetchDictation,
  submitDictation,
  correctDictation,
  fetchDone,
  checkin,
  fetchPronunciation,
} from '@/api/practice'
import { ApiError } from '@/api/client'
import type {
  TodayResponse,
  CardDetailResponse,
  ReadingResponse,
  DictationResponse,
  SubmitAnswer,
  SubmitDictationResponse,
  DoneResponse,
  CheckinResponse,
} from '@/types/practice'

// 学习流程的 5 个步骤（对齐原型 stepper）。
export type PracticeView = 'dashboard' | 'cards' | 'reading' | 'dictation' | 'done'

export const usePracticeStore = defineStore('practice', () => {
  const today = ref<TodayResponse | null>(null)
  const card = ref<CardDetailResponse | null>(null)
  const reading = ref<ReadingResponse | null>(null)
  const dictation = ref<DictationResponse | null>(null)
  const done = ref<DoneResponse | null>(null)
  const checkinResp = ref<CheckinResponse | null>(null)

  const view = ref<PracticeView>('dashboard')
  // 已解锁的步骤（对齐原型：初始只展示步骤1，逐步解锁）
  const unlockedViews = ref<Set<PracticeView>>(new Set(['dashboard']))

  const loading = ref(false)
  const error = ref<string | null>(null)
  const listening = ref(false)
  const readDoneSaving = ref(false)

  // 当前单词卡序号（前端维护，用于上一个/下一个导航）
  const cardIndex = ref(0)

  let loadSeq = 0
  let readingPoll: ReturnType<typeof setTimeout> | null = null

  // --- 首页 ---
  async function loadToday() {
    const seq = ++loadSeq
    loading.value = true
    error.value = null
    try {
      today.value = await fetchToday()
      // 进入首页时重置步骤解锁状态（保留已完成则按 today 推断）
      // 若当天无任务词，停留在 dashboard
    } catch (e) {
      if (seq === loadSeq) error.value = (e as Error).message
    } finally {
      if (seq === loadSeq) loading.value = false
    }
  }

  // --- 步骤推进 ---
  function unlock(v: PracticeView) {
    unlockedViews.value.add(v)
    // 触发响应式：重新赋值 Set
    unlockedViews.value = new Set(unlockedViews.value)
  }

  function switchView(v: PracticeView) {
    if (!unlockedViews.value.has(v)) return
    view.value = v
  }

  // 从首页重新开始今日任务时，只重置前端页面流程；不改动已保存的学习数据。
  // 避免上一轮完成页数据和已解锁步骤误导新一轮的导航状态。
  function resetPracticeSession() {
    card.value = null
    reading.value = null
    dictation.value = null
    done.value = null
    checkinResp.value = null
    cardIndex.value = 0
    error.value = null
    view.value = 'dashboard'
    unlockedViews.value = new Set(['dashboard'])
  }

  // 开始学习 → 进入单词卡，加载第一张卡
  async function startStudy() {
    if (!today.value || today.value.totalCount === 0) return
    unlock('cards')
    switchView('cards')
    cardIndex.value = 0
    await loadCardByIndex(0)
  }

  // 按序号加载单词卡：先 enter（首张触发未学→学习中），后续 listen/readDone 维持
  async function loadCardByIndex(index: number) {
    if (!today.value) return
    const words = [...today.value.firstPool, ...today.value.reviewPool]
    if (index < 0 || index >= words.length) return
    cardIndex.value = index
    const word = words[index]
    try {
      card.value = await enterCard(word.id)
    } catch (e) {
      error.value = (e as Error).message
    }
  }

  // 优先播放服务端缓存的真人词典录音；确实没有时才用浏览器英式 TTS 兜底。
  async function listenCurrent(): Promise<boolean> {
    if (!card.value || listening.value) return false
    listening.value = true
    try {
      const pronunciation = await fetchPronunciation(card.value.word.id)
      await playAudio(pronunciation.audioUrl)
      card.value = await markListened(card.value.word.id)
      return true
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) {
        try {
          await speakWord(card.value.word.text)
          card.value = await markListened(card.value.word.id)
          return true
        } catch (fallbackError) {
          error.value = (fallbackError as Error).message
          return false
        }
      }
      error.value = (e as Error).message
      return false
    } finally {
      listening.value = false
    }
  }

  // 我读完了
  async function readDoneCurrent(): Promise<boolean> {
    if (!card.value || readDoneSaving.value) return false
    readDoneSaving.value = true
    try {
      card.value = await markReadDone(card.value.word.id)
      return true
    } catch (e) {
      error.value = (e as Error).message
      return false
    } finally {
      readDoneSaving.value = false
    }
  }

  // 不会读
  async function hardCurrent() {
    if (!card.value) return
    try {
      card.value = await markHard(card.value.word.id)
    } catch (e) {
      error.value = (e as Error).message
    }
  }

  // 上一个
  async function prevCard() {
    if (cardIndex.value > 0) await loadCardByIndex(cardIndex.value - 1)
  }

  // 下一个；最后一张跳阅读
  async function nextCard() {
    if (!today.value) return
    const total = today.value.totalCount
    if (cardIndex.value < total - 1) {
      await loadCardByIndex(cardIndex.value + 1)
    } else {
      await goReading()
    }
  }

  // --- 阅读 ---
  async function goReading() {
    unlock('reading')
    switchView('reading')
    await loadReading()
  }

  // 当短文仍在生成中（pending）时启动轮询，直到拿到 success/failed 或离开阅读视图。
  function startReadingPollIfPending() {
    if (reading.value?.status !== 'pending') return
    if (readingPoll) clearTimeout(readingPoll)
    readingPoll = setTimeout(() => {
      if (view.value === 'reading') void loadReading()
    }, 1800)
  }

  async function loadReading() {
    loading.value = true
    try {
      reading.value = await fetchReading()
      startReadingPollIfPending()
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function regenerate() {
    loading.value = true
    try {
      reading.value = await regenerateReading()
      // 手动重新生成后，若后端仍返回 pending（异步生成中），需要继续轮询刷新
      startReadingPollIfPending()
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function finishReading(): Promise<boolean> {
    try {
      await completeReading()
      await goDictation()
      return true
    } catch (e) {
      // 阅读时长不足等业务错误，由调用方提示
      const message = (e as Error).message
      // 服务端是完成资格的最终判断方；若本地显示与服务端有偏差，立即重新取数校准。
      if (e instanceof ApiError && e.code === 'READING_TOO_SHORT') {
        await loadReading()
      }
      error.value = message
      return false
    }
  }

  // --- 默写 ---
  async function goDictation() {
    unlock('dictation')
    switchView('dictation')
    await loadDictation()
  }

  async function loadDictation() {
    try {
      dictation.value = await fetchDictation()
    } catch (e) {
      error.value = (e as Error).message
    }
  }

  async function submit(answers: SubmitAnswer[]): Promise<SubmitDictationResponse | null> {
    try {
      const resp = await submitDictation(answers)
      await loadDictation() // 刷新题面（含锁定态）
      return resp
    } catch (e) {
      error.value = (e as Error).message
      return null
    }
  }

  async function correct(answers: SubmitAnswer[]): Promise<SubmitDictationResponse | null> {
    try {
      const resp = await correctDictation(answers)
      await loadDictation()
      return resp
    } catch (e) {
      error.value = (e as Error).message
      return null
    }
  }

  // --- 完成页 ---
  async function goDone() {
    unlock('done') // 兜底：刷新或从已锁定默写页直接跳完成页时 done 可能未解锁
    switchView('done')
    await loadDone()
  }

  async function loadDone() {
    try {
      done.value = await fetchDone()
    } catch (e) {
      error.value = (e as Error).message
    }
  }

  async function doCheckin(): Promise<CheckinResponse | null> {
    error.value = null
    try {
      const resp = await checkin()
      checkinResp.value = resp
      // 刷新完成页打卡概要
      await loadDone()
      return resp
    } catch (e) {
      error.value = (e as Error).message
      return null
    }
  }

  // --- 派生 ---
  const totalCards = computed(() => today.value?.totalCount ?? 0)
  const cardProgressText = computed(() => {
    if (!card.value) return ''
    return `${card.value.cardIndex + 1} / ${card.value.total}`
  })

  return {
    today,
    card,
    reading,
    dictation,
    done,
    checkinResp,
    view,
    unlockedViews,
    loading,
    error,
    listening,
    readDoneSaving,
    cardIndex,
    totalCards,
    cardProgressText,
    loadToday,
    resetPracticeSession,
    startStudy,
    loadCardByIndex,
    listenCurrent,
    readDoneCurrent,
    hardCurrent,
    prevCard,
    nextCard,
    goReading,
    loadReading,
    regenerate,
    finishReading,
    goDictation,
    loadDictation,
    submit,
    correct,
    goDone,
    loadDone,
    doCheckin,
    unlock,
    switchView,
  }
})

let activeAudio: HTMLAudioElement | null = null
function playAudio(url: string): Promise<void> {
  activeAudio?.pause()
  activeAudio = new Audio(url)
  return activeAudio.play()
}

// 仅作为词典录音缺失时的 Web 兜底；iOS/小程序可替换为各自原生实现。
export function speakWord(word: string, rate = 1): Promise<void> {
  return new Promise((resolve, reject) => {
    if (!('speechSynthesis' in window)) {
      reject(new Error('当前设备不支持语音播放'))
      return
    }
    window.speechSynthesis.cancel()
    const utterance = new SpeechSynthesisUtterance(word)
    utterance.lang = 'en-GB'
    utterance.rate = rate
    utterance.onstart = () => resolve()
    utterance.onerror = () => reject(new Error('语音播放失败'))
    window.speechSynthesis.speak(utterance)
  })
}
