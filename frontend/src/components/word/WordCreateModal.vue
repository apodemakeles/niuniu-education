<script setup lang="ts">
import { ref } from 'vue'
import { createWord } from '@/api/words'
import { ApiError } from '@/api/client'
import type { Word, WordType } from '@/types/word'

const props = defineProps<{ defaultType?: WordType }>()
const emit = defineEmits<{
  created: [word: Word]
  closed: []
}>()

const text = ref('')
const meaningZh = ref('')
const phonetic = ref('')
const wordType = ref<WordType>(props.defaultType || 'new')
const submitting = ref(false)
const error = ref<string | null>(null)
const pendingDuplicate = ref(false) // 当前录入命中重复，等待家长确认 force

async function submit(force = false) {
  if (!text.value.trim() || !meaningZh.value.trim()) {
    error.value = '英文单词和中文释义必填'
    return
  }
  submitting.value = true
  error.value = null
  try {
    const w = await createWord(
      { text: text.value.trim(), meaningZh: meaningZh.value.trim(), phonetic: phonetic.value.trim(), wordType: wordType.value },
      force,
    )
    pendingDuplicate.value = false
    emit('created', w)
    // 连续录入：清空表单，保留类型，聚焦到英文输入框
    text.value = ''
    meaningZh.value = ''
    phonetic.value = ''
    document.querySelector<HTMLInputElement>('input[data-field="create-text"]')?.focus()
  } catch (e) {
    if (e instanceof ApiError && e.code === 'WORD_DUPLICATE') {
      pendingDuplicate.value = true
      error.value = '词库内已有该单词，仍然新增为同形词？'
    } else {
      error.value = (e as Error).message
    }
  } finally {
    submitting.value = false
  }
}

// 回车保存并继续
function onEnter() {
  submit(pendingDuplicate.value)
}
</script>

<template>
  <div class="modal-backdrop" @click.self="emit('closed')">
    <section class="modal" role="dialog" aria-modal="true">
      <button class="close-btn" type="button" aria-label="关闭" @click="emit('closed')">×</button>
      <div class="modal-title">
        <h2>逐个录入单词</h2>
        <p>保存后不关闭窗口，方便连续录入。回车可快速保存下一个。</p>
      </div>

      <form class="form-grid" @submit.prevent="onEnter">
        <div class="field">
          <label>英文单词</label>
          <input v-model="text" data-testid="create-text" required />
        </div>
        <div class="field">
          <label>中文</label>
          <input v-model="meaningZh" data-testid="create-meaning" required />
        </div>
        <div class="field">
          <label>音标</label>
          <input v-model="phonetic" placeholder="/.../" />
        </div>
        <div class="field">
          <label>录入类型</label>
          <select v-model="wordType">
            <option value="new">新词</option>
            <option value="mistake">需强化词（加入复习计划）</option>
          </select>
        </div>

        <p v-if="wordType === 'mistake'" class="note full">保存后会以“需强化”加入复习计划；后续状态由孩子的学习结果自动更新。</p>

        <p v-if="error" class="note error full">{{ error }}</p>

        <div class="form-actions full">
          <button class="secondary-btn" type="button" :disabled="submitting" @click="emit('closed')">完成</button>
          <button class="primary-btn" type="submit" :disabled="submitting">
            {{ submitting ? '保存中…' : pendingDuplicate ? '仍然新增' : '保存并继续' }}
          </button>
        </div>
      </form>
    </section>
  </div>
</template>
