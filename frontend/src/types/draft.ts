// 导入草稿相关类型，对齐后端 wordlibrary.DTO

export type DraftIssue = 'low_confidence' | 'missing_meaning' | 'duplicate' | 'invalid'

export interface DraftRow {
  rowId: string
  text: string
  meaningZh: string
  phonetic: string
  wordType: 'new' | 'mistake'
  confidence: number
  issues: DraftIssue[]
}

export interface OCRDraftResponse {
  imageId: string
  rows: DraftRow[]
  rawText?: string
}

export interface ConfirmRow {
  text: string
  meaningZh: string
  phonetic: string
  wordType: 'new' | 'mistake'
}

export interface ImportResultDetail {
  rowId?: string
  wordId?: string
  text: string
  result: 'added' | 'skipped' | 'invalid'
  reason?: string
}

export interface ImportResult {
  added: number
  skipped: number
  invalid: number
  details: ImportResultDetail[]
}
