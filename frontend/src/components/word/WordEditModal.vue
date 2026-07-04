<script setup lang="ts">
import { ref, watch } from 'vue'
import { updateWord } from '@/api/words'
import type { Word, WordType, WordStatus } from '@/types/word'

const props = defineProps<{ word: Word }>()
const emit = defineEmits<{
  saved: [word: Word]
  closed: []
}>()

const meaningZh = ref(props.word.meaningZh)
const phonetic = ref(props.word.phonetic)
const wordType = ref<WordType>(props.word.wordType)
const status = ref<WordStatus>(props.word.status)
const submitting = ref(false)
const error = ref<string | null>(null)

// 切换单词时同步表单
watch(() => props.word, (w) => {
  meaningZh.value = w.meaningZh
  phonetic.value = w.phonetic
  wordType.value = w.wordType
  status.value = w.status
})

async function onSave() {
  submitting.value = true
  error.value = null
  try {
    const updated = await updateWord(props.word.id, {
      meaningZh: meaningZh.value.trim(),
      phonetic: phonetic.value.trim(),
      wordType: wordType.value,
      status: status.value,
    })
    emit('saved', updated)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="modal-backdrop" @click.self="emit('closed')">
    <section class="modal" role="dialog" aria-modal="true">
      <button class="close-btn" type="button" aria-label="关闭" @click="emit('closed')">×</button>
      <div class="modal-title">
        <h2>编辑单词</h2>
        <p>英文单词不可修改；如录错请删除后重新录入。</p>
      </div>

      <form class="form-grid" @submit.prevent="onSave">
        <div class="field">
          <label>英文单词</label>
          <input :value="word.text" readonly />
        </div>
        <div class="field">
          <label>中文</label>
          <input v-model="meaningZh" required />
        </div>
        <div class="field">
          <label>音标</label>
          <input v-model="phonetic" placeholder="/.../" />
        </div>
        <div class="field">
          <label>类型</label>
          <select v-model="wordType">
            <option value="new">新词</option>
            <option value="mistake">易错词</option>
          </select>
        </div>
        <div class="field">
          <label>状态</label>
          <select v-model="status">
            <option value="unlearned">未学</option>
            <option value="learning">学习中</option>
            <option value="reinforce">需强化</option>
            <option value="mastered">已掌握</option>
          </select>
        </div>

        <p v-if="error" class="note error full">{{ error }}</p>

        <div class="form-actions full">
          <button class="secondary-btn" type="button" :disabled="submitting" @click="emit('closed')">取消</button>
          <button class="primary-btn" type="submit" :disabled="submitting">
            {{ submitting ? '保存中…' : '保存修改' }}
          </button>
        </div>
      </form>
    </section>
  </div>
</template>
