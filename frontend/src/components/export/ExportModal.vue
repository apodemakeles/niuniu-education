<script setup lang="ts">
import { ref, watch } from 'vue'
import { previewDictation, downloadDictation, type DictationItem } from '@/api/export'

const emit = defineEmits<{ closed: [] }>()

const scope = ref('all')
const title = ref('单词默写练习')
const items = ref<DictationItem[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const exporting = ref(false)

const scopes = [
  { value: 'all', label: '全部单词' },
  { value: 'unlearned', label: '未学' },
  { value: 'learning', label: '学习中' },
  { value: 'reinforce', label: '需强化' },
  { value: 'mastered', label: '已掌握' },
]

async function loadPreview() {
  loading.value = true
  error.value = null
  try {
    const p = await previewDictation(scope.value, title.value)
    items.value = p.items
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

// 切换范围或标题时重新加载预览
watch([scope], loadPreview)

async function onExport() {
  if (items.value.length === 0) {
    error.value = '没有可导出的内容'
    return
  }
  exporting.value = true
  error.value = null
  try {
    const blob = await downloadDictation(title.value, items.value)
    // 触发浏览器下载
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = (title.value || '单词默写练习') + '.doc'
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
    emit('closed')
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    exporting.value = false
  }
}

loadPreview()
</script>

<template>
  <div class="modal-backdrop" @click.self="emit('closed')">
    <section class="modal" role="dialog" aria-modal="true">
      <button class="close-btn" type="button" aria-label="关闭" @click="emit('closed')">×</button>
      <div class="modal-title">
        <h2>导出单词</h2>
        <p>导出为 Word 默写表，只保留中文提示，英文留空给孩子书写。</p>
      </div>

      <div class="export-layout">
        <section class="export-controls">
          <div class="field">
            <label>导出内容</label>
            <select v-model="scope">
              <option v-for="s in scopes" :key="s.value" :value="s.value">{{ s.label }}</option>
            </select>
          </div>
          <div class="field">
            <label>文件名/标题</label>
            <input v-model="title" />
          </div>
          <p class="note">预览表里的中文可以临时编辑，但不反写正式单词库。</p>
        </section>

        <section class="export-preview-panel">
          <p v-if="loading" class="note">加载预览中…</p>
          <p v-else-if="error" class="note error">{{ error }}</p>
          <template v-else>
            <div class="dictation-sheet">
              <div class="dictation-title">
                <input v-model="title" class="title-input" aria-label="默写表标题" />
                <div class="meta-line">
                  <span>姓名：__________</span>
                  <span>日期：__________</span>
                </div>
              </div>
              <table class="dictation-table">
                <thead>
                  <tr>
                    <th>序号</th>
                    <th>中文</th>
                    <th>英文默写</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(it, i) in items" :key="i">
                    <td class="idx">{{ it.index }}</td>
                    <td><input v-model="it.meaning" class="meaning-input" /></td>
                    <td><div class="answer-lines"></div></td>
                  </tr>
                  <tr v-if="items.length === 0">
                    <td colspan="3" class="note">当前范围没有可导出的单词。</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </template>
        </section>
      </div>

      <div class="form-actions">
        <button class="secondary-btn" type="button" :disabled="exporting" @click="emit('closed')">取消</button>
        <button class="primary-btn" type="button" :disabled="exporting || items.length === 0" @click="onExport">
          {{ exporting ? '导出中…' : '导出 Word' }}
        </button>
      </div>
    </section>
  </div>
</template>
