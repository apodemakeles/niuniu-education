<script setup lang="ts">
import { ref } from 'vue'
import { recognizeImage } from '@/api/import'
import type { DraftRow, ImportResult } from '@/types/draft'
import DraftTable from './DraftTable.vue'

const emit = defineEmits<{
  confirmed: [result: ImportResult]
  closed: []
}>()

const stage = ref<'select' | 'recognizing' | 'draft'>('select')
const previewUrl = ref<string | null>(null)
const error = ref<string | null>(null)
const rows = ref<DraftRow[]>([])

let selectedFile: File | null = null

function onPick(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  selectedFile = file
  previewUrl.value = URL.createObjectURL(file)
  error.value = null
}

async function onRecognize() {
  if (!selectedFile) {
    error.value = '请先选择图片'
    return
  }
  stage.value = 'recognizing'
  error.value = null
  try {
    const res = await recognizeImage(selectedFile)
    rows.value = res.rows
    stage.value = 'draft'
  } catch (e) {
    error.value = (e as Error).message
    stage.value = 'select'
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

      <!-- 阶段1：选择图片 -->
      <div v-if="stage === 'select'" class="modal-title">
        <h2>拍照导入</h2>
        <p>图片识别后只生成草稿，家长修正并确认后才会入库。</p>
        <div class="draft-grid">
          <div class="photo-preview">
            <img v-if="previewUrl" :src="previewUrl" alt="图片预览" />
            <span v-else>选择或拍摄单词表图片</span>
          </div>
          <div>
            <input type="file" accept="image/*" capture="environment" @change="onPick" />
            <p v-if="error" class="note error">{{ error }}</p>
            <div class="form-actions">
              <button class="secondary-btn" type="button" @click="close">取消</button>
              <button class="primary-btn" type="button" :disabled="!selectedFile" @click="onRecognize">
                识别并生成草稿
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- 阶段2：识别中 -->
      <div v-else-if="stage === 'recognizing'" class="modal-title">
        <h2>正在识别…</h2>
        <p class="note">OCR 识别可能需要十几秒，请稍候。</p>
      </div>

      <!-- 阶段3：草稿预览与确认 -->
      <div v-else>
        <div class="modal-title">
          <h2>拍照识别草稿</h2>
          <p>共识别出 {{ rows.length }} 行，请修正后确认入库。</p>
        </div>
        <DraftTable :rows="rows" @confirmed="onConfirmed" @cancelled="close" />
      </div>
    </section>
  </div>
</template>
