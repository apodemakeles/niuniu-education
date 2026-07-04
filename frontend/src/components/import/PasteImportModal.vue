<script setup lang="ts">
import { ref } from 'vue'
import { parsePaste } from '@/api/import'
import type { DraftRow, ImportResult } from '@/types/draft'
import DraftTable from './DraftTable.vue'

const emit = defineEmits<{
  confirmed: [result: ImportResult]
  closed: []
}>()

const stage = ref<'input' | 'draft'>('input')
const text = ref('')
const error = ref<string | null>(null)
const parsing = ref(false)
const rows = ref<DraftRow[]>([])

async function onParse() {
  if (!text.value.trim()) {
    error.value = '请粘贴单词表文本'
    return
  }
  parsing.value = true
  error.value = null
  try {
    rows.value = await parsePaste(text.value)
    stage.value = 'draft'
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    parsing.value = false
  }
}

function onConfirmed(result: ImportResult) {
  emit('confirmed', result)
}

function close() {
  emit('closed')
}
</script>

<template>
  <div class="modal-backdrop" @click.self="close">
    <section class="modal" role="dialog" aria-modal="true">
      <button class="close-btn" type="button" aria-label="关闭" @click="close">×</button>

      <div v-if="stage === 'input'" class="modal-title">
        <h2>粘贴导入</h2>
        <p>每行一个单词，支持「英文 中文」或「英文,中文,音标」格式。确认前只生成草稿。</p>
        <div class="field full">
          <textarea v-model="text" placeholder="apple 苹果&#10;banana 香蕉&#10;read,阅读,/riːd/" rows="8"></textarea>
        </div>
        <p v-if="error" class="note error">{{ error }}</p>
        <div class="form-actions">
          <button class="secondary-btn" type="button" @click="close">取消</button>
          <button class="primary-btn" type="button" :disabled="parsing" @click="onParse">
            {{ parsing ? '解析中…' : '生成草稿预览' }}
          </button>
        </div>
      </div>

      <div v-else>
        <div class="modal-title">
          <h2>粘贴导入草稿</h2>
          <p>共解析出 {{ rows.length }} 行，请修正后确认入库。</p>
        </div>
        <DraftTable :rows="rows" @confirmed="onConfirmed" @cancelled="close" />
      </div>
    </section>
  </div>
</template>
