<script setup lang="ts">
import { ref } from 'vue'
import { streamGenerateExamples } from '@/api/examples'

const emit = defineEmits<{ closed: []; finished: [] }>()
const running = ref(false)
const finished = ref(false)
const current = ref(0)
const total = ref(0)
const currentWord = ref('')
const failed = ref(0)
const error = ref<string | null>(null)

async function start() {
  running.value = true
  error.value = null
  failed.value = 0
  try {
    await streamGenerateExamples('/examples/backfill/stream', undefined,
      (value) => { total.value = value },
      (progress) => {
        current.value = progress.current
        currentWord.value = progress.text || currentWord.value
        if (progress.status === 'failed') failed.value++
      })
    finished.value = true
    emit('finished')
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    running.value = false
  }
}
</script>

<template>
  <div class="modal-backdrop" @click.self="!running && emit('closed')">
    <section class="modal example-backfill-modal" role="dialog" aria-modal="true">
      <button class="close-btn" type="button" aria-label="关闭" :disabled="running" @click="emit('closed')">×</button>
      <div class="modal-title">
        <h2>补全历史单词例句</h2>
        <p>只会处理例句不足 3 条的词；每个词生成 3 句适合小学阶段的学习句子。</p>
      </div>

      <div v-if="running" class="example-backfill-status" aria-live="polite">
        <i class="example-spinner" aria-hidden="true"></i>
        <strong>正在生成 {{ current }}/{{ total }}{{ currentWord ? ` · ${currentWord}` : '' }}</strong>
        <span>单个词失败不会影响后续词，可稍后在编辑页重试。</span>
      </div>
      <div v-else-if="finished" class="example-backfill-status done">
        <strong>例句补全完成</strong>
        <span>已处理 {{ total }} 个词{{ failed ? `，${failed} 个失败` : '' }}。</span>
      </div>
      <p v-if="error" class="note error">{{ error }}</p>

      <div class="form-actions">
        <button class="secondary-btn" type="button" :disabled="running" @click="emit('closed')">关闭</button>
        <button v-if="!finished" class="primary-btn" type="button" :disabled="running" @click="start">
          {{ running ? '正在补全…' : '开始补全' }}
        </button>
      </div>
    </section>
  </div>
</template>
