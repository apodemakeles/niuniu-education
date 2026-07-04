// 与后端 DTO 对齐的类型定义（M0 仅 Word，后续随接口迭代补充）

export type WordType = 'new' | 'mistake'

export type WordStatus = 'unlearned' | 'learning' | 'reinforce' | 'mastered'

export type WordSource = 'manual' | 'paste' | 'photo'

export interface Word {
  id: string
  text: string
  meaningZh: string
  phonetic: string
  wordType: WordType
  status: WordStatus
  source?: WordSource
  createdAt?: string
  updatedAt?: string
  lastEditedAt?: string | null
}

export const WORD_TYPE_TEXT: Record<WordType, string> = {
  new: '新词',
  mistake: '易错词',
}

export const WORD_STATUS_TEXT: Record<WordStatus, string> = {
  unlearned: '未学',
  learning: '学习中',
  reinforce: '需强化',
  mastered: '已掌握',
}
