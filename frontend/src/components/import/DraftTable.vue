<script setup lang="ts">
import { ref } from 'vue'
import type { DraftRow, ConfirmRow, ImportResult } from '@/types/draft'
import { confirmImport } from '@/api/import'
import { streamGenerateExamples } from '@/api/examples'

const props = defineProps<{
  rows: DraftRow[]
  title?: string
}>()

const emit = defineEmits<{
  confirmed: [result: ImportResult]
  cancelled: []
}>()

const submitting = ref(false)
const error = ref<string | null>(null)
const generationCurrent = ref(0)
const generationTotal = ref(0)
const generationWord = ref('')
const generationFailed = ref(0)

function removeRow(index: number) {
  props.rows.splice(index, 1)
}

function issueText(row: DraftRow): string {
  if (!row.issues || row.issues.length === 0) return ''
  const map: Record<string, string> = {
    low_confidence: '低置信度',
    missing_meaning: '缺中文',
    duplicate: '重复',
    invalid: '无效',
  }
  return row.issues.map((i) => map[i] || i).join('、')
}

async function onConfirm() {
  submitting.value = true
  error.value = null
  // 过滤掉英文单词为空的行
  const validRows: ConfirmRow[] = props.rows
    .filter((r) => r.text.trim() !== '')
    .map((r) => ({
      text: r.text.trim(),
      meaningZh: r.meaningZh.trim(),
      phonetic: r.phonetic.trim(),
      wordType: r.wordType,
    }))
  if (validRows.length === 0) {
    error.value = '没有可入库的行（英文单词均为空）'
    submitting.value = false
    return
  }
  try {
    const result = await confirmImport(validRows)
    const addedIDs = result.details
      .filter((detail) => detail.result === 'added' && detail.wordId)
      .map((detail) => detail.wordId!)
    if (addedIDs.length > 0) {
      generationCurrent.value = 0
      generationTotal.value = addedIDs.length
      generationWord.value = ''
      generationFailed.value = 0
      await streamGenerateExamples('/examples/generate/stream', addedIDs,
        (total) => { generationTotal.value = total },
        (progress) => {
          generationCurrent.value = progress.current
          generationWord.value = progress.text || generationWord.value
          if (progress.status === 'failed') generationFailed.value++
        })
    }
    emit('confirmed', result)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="draft-wrap">
    <p class="draft-note">
      请先修正草稿。低置信度或缺中文的行需重点检查，确认入库前不会写入正式单词库。
      可删除无关行（如标题、姓名栏）。
    </p>

    <div class="table-wrap">
      <table class="draft-table">
        <thead>
          <tr>
            <th>英文单词</th>
            <th>中文</th>
            <th>音标</th>
            <th>类型</th>
            <th>提示</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(row, i) in rows" :key="i" :class="{ flagged: row.issues?.length }">
            <td><input v-model="row.text" data-field="text" /></td>
            <td><input v-model="row.meaningZh" data-field="meaningZh" /></td>
            <td><input v-model="row.phonetic" data-field="phonetic" /></td>
            <td>
              <select v-model="row.wordType">
                <option value="new">新词</option>
                <option value="mistake">易错词</option>
              </select>
            </td>
            <td><span v-if="issueText(row)" class="issue-pill">{{ issueText(row) }}</span></td>
            <td>
              <button class="link-btn" type="button" @click="removeRow(i)">删除</button>
            </td>
          </tr>
          <tr v-if="rows.length === 0">
            <td colspan="6" class="note">没有草稿行。</td>
          </tr>
        </tbody>
      </table>
    </div>

    <p v-if="error" class="note error">{{ error }}</p>

    <div class="draft-actions">
      <button class="secondary-btn" type="button" :disabled="submitting" @click="emit('cancelled')">
        取消
      </button>
      <button class="primary-btn" type="button" :disabled="submitting" @click="onConfirm">
        {{ submitting ? (generationTotal ? `正在生成例句 ${generationCurrent}/${generationTotal}` : '确认入库中…') : '确认入库并生成例句' }}
      </button>
    </div>

    <div v-if="submitting && generationTotal" class="example-batch-progress" aria-live="polite">
      <i class="example-spinner" aria-hidden="true"></i>
      <div><strong>正在为新增词写例句</strong><span>{{ generationCurrent }}/{{ generationTotal }} {{ generationWord ? `· ${generationWord}` : '' }}</span></div>
      <small v-if="generationFailed">已有 {{ generationFailed }} 个词生成失败，可在“编辑”中重试。</small>
    </div>
  </div>
</template>
