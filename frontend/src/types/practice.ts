// 与后端 studentwordtask DTO 对齐的类型定义（对齐 design/docs/student-word-task.md）。

export type PoolType = 'first' | 'review'
export type LoadDayType = 'light' | 'normal' | 'pressure'
export type ReadStatus = 'not_started' | 'listened' | 'read_done' | 'hard'
export type DictationResult = 'pending' | 'correct' | 'wrong' | 'blank'
export type CorrectionResult = 'not_submitted' | 'correct' | 'wrong' | 'blank'
export type ReadingAIStatus = 'pending' | 'success' | 'failed'

// 字段名与后端 JSON 契约保持 camelCase。
export interface MissionWord {
  id: string
  text: string
  meaningZh: string
  phonetic: string
  wordType: 'new' | 'mistake'
  typeLabel: string
  learningLabel: string
  stage: string
  poolType: PoolType
  displayOrder: number
  readStatus: ReadStatus
}

export interface TodayResponse {
  date: string
  loadDay: LoadDayType
  loadDayLabel: string
  loadDayDesc: string
  firstPool: MissionWord[]
  reviewPool: MissionWord[]
  newPoolCount: number
  reviewPoolMax: number
  totalCount: number
  empty: boolean
  emptyHint?: string
  completed: boolean
}

export interface CardDetailResponse {
  word: MissionWord
  cardIndex: number
  total: number
  example: string
  exampleMissing: boolean
  listened: boolean
  readDone: boolean
  isFirstCard: boolean
  isLastCard: boolean
}

export interface CoveredWord {
  id: string
  text: string
  meaningZh: string
}

export interface Appearance {
  id: string
  text: string
  count: number
}

export interface ReadingResponse {
  date: string
  title: string
  text: string // 已高亮 HTML
  rawText: string
  sceneHint: string
  coveredWords: CoveredWord[]
  appearances: Appearance[]
  status: ReadingAIStatus
  minSeconds: number
  startedAt?: string
  completedAt?: string
  elapsedSeconds: number
  canFinish: boolean
}

export interface DictationItem {
  taskId: string
  wordId: string
  meaningZh: string
  phonetic?: string
  poolType: PoolType
  firstAnswer?: string
  firstResult?: DictationResult
  locked: boolean
  correctionAnswer?: string
  correctionResult?: CorrectionResult
  bonusEligible: boolean
  bonusAwarded: boolean
}

export interface DictationResponse {
  date: string
  items: DictationItem[]
}

export interface SubmitAnswer {
  taskId: string
  wordId: string
  answer: string
}

export interface SubmitDictationResponse {
  items: DictationItem[]
  correct: number
  wrong: number
  blank: number
  bonusIds: string[]
}

export interface CheckinSummary {
  monthLabel: string
  checkedDays: number[]
  todayDay: number
  streak: number
  bonus7Day: boolean
}

export interface DoneResponse {
  date: string
  total: number
  correctCount: number
  wrongCount: number
  blankCount: number
  usedHintCount: number
  tomorrowReview: MissionWord[]
  needReinforce: MissionWord[]
  dictationLocked: boolean
  hasCorrection: boolean
  checkin: CheckinSummary
}

export interface CheckinResponse {
  done: boolean
  summary: CheckinSummary
}

export interface PronunciationResponse {
  wordId: string
  locale: string
  provider: string
  phonetic: string
  audioUrl: string
  sourceUrl?: string
  licenseName?: string
  licenseUrl?: string
  attribution?: string
}

// --- 文案常量 ---

export const LOAD_DAY_TEXT: Record<LoadDayType, string> = {
  light: '轻松日',
  normal: '普通日',
  pressure: '压力日',
}

export const READ_STATUS_TEXT: Record<ReadStatus, string> = {
  not_started: '未开始',
  listened: '已听音',
  read_done: '已读完',
  hard: '不会读',
}

export const DICTATION_RESULT_TEXT: Record<DictationResult, string> = {
  pending: '待提交',
  correct: '正确',
  wrong: '需订正',
  blank: '未填写',
}
