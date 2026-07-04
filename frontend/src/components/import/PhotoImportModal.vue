<script setup lang="ts">
import { ref, onUnmounted } from 'vue'
import { recognizeImageStream } from '@/api/import'
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
const stageText = ref('') // 实时进度文字
let selectedFile: File | null = null
let abortController: AbortController | null = null

function onPick(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  selectedFile = file
  previewUrl.value = URL.createObjectURL(file)
  error.value = null
}

function onRecognize() {
  if (!selectedFile) {
    error.value = '请先选择图片'
    return
  }
  stage.value = 'recognizing'
  error.value = null
  rows.value = []
  stageText.value = '正在上传图片…'

  abortController = recognizeImageStream(selectedFile, {
    onStage: (s) => {
      if (s === 'recognizing') stageText.value = '正在识别文字…'
    },
    onRow: (row) => {
      // 增量词行实时出现
      stageText.value = `已识别 ${rows.value.length + 1} 个词…`
      rows.value.push(row)
    },
    onFinal: (result) => {
      // 用最终完整解析结果替换预览（含 issues 标记等）
      rows.value = result.rows
      stage.value = 'draft'
    },
    onError: (msg) => {
      error.value = msg
      stage.value = 'select'
    },
  })
}

function onConfirmed(result: ImportResult) {
  emit('confirmed', result)
}

function close() {
  // 关闭时取消进行中的流
  if (abortController) {
    abortController.abort()
    abortController = null
  }
  emit('closed')
}

onUnmounted(() => {
  if (abortController) abortController.abort()
})
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

      <!-- 阶段2：识别中（实时进度 + 词行滚动出现） -->
      <div v-else-if="stage === 'recognizing'" class="recognizing-wrap">
        <div class="modal-title">
          <h2>{{ stageText || '正在识别…' }}</h2>
          <p class="note">识别结果会实时显示在下方，完成后可编辑确认。</p>
        </div>
        <div v-if="rows.length > 0" class="stream-preview">
          <div class="table-wrap">
            <table class="draft-table">
              <thead>
                <tr>
                  <th>英文单词</th>
                  <th>中文</th>
                  <th>音标</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="r in rows" :key="r.rowId">
                  <td><strong>{{ r.text }}</strong></td>
                  <td>{{ r.meaningZh }}</td>
                  <td>{{ r.phonetic }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
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
