<script setup lang="ts">
import { ref, watch } from 'vue'
import { fetchExamples, generateExamples, regenerateExample } from '@/api/examples'
import type { WordExample } from '@/api/examples'

const props = withDefaults(defineProps<{
  wordId: string
  autoGenerate?: boolean
}>(), { autoGenerate: false })

const emit = defineEmits<{ ready: [examples: WordExample[]] }>()
const examples = ref<WordExample[]>([])
const generating = ref(false)
const regeneratingId = ref<string | null>(null)
const error = ref<string | null>(null)

async function load() {
  error.value = null
  try {
    examples.value = await fetchExamples(props.wordId)
    if (props.autoGenerate && examples.value.length !== 3) await generateAll()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function generateAll() {
  generating.value = true
  error.value = null
  try {
    examples.value = await generateExamples(props.wordId)
    emit('ready', examples.value)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    generating.value = false
  }
}

async function regenerate(example: WordExample) {
  regeneratingId.value = example.id
  error.value = null
  try {
    const updated = await regenerateExample(props.wordId, example.id)
    examples.value = examples.value.map((item) => item.id === updated.id ? updated : item)
    emit('ready', examples.value)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    regeneratingId.value = null
  }
}

watch(() => props.wordId, load, { immediate: true })
</script>

<template>
  <section class="example-panel" aria-live="polite">
    <div class="example-panel-head">
      <div>
        <strong>AI 学习例句</strong>
        <span>适合小学阶段，共 3 句</span>
      </div>
      <button v-if="examples.length !== 3 && !generating" class="secondary-btn" type="button" @click="generateAll">生成 3 句</button>
    </div>

    <div v-if="generating" class="example-generating">
      <i class="example-spinner" aria-hidden="true"></i>
      <span>正在写适合孩子的例句…</span>
    </div>

    <ol v-else-if="examples.length" class="example-list">
      <li v-for="example in examples" :key="example.id">
        <span>{{ example.sentence }}</span>
        <button class="link-btn" type="button" :disabled="regeneratingId === example.id" @click="regenerate(example)">
          {{ regeneratingId === example.id ? '生成中…' : '↻ 换一句' }}
        </button>
      </li>
    </ol>
    <p v-else class="note">还没有例句，可点击“生成 3 句”。</p>
    <p v-if="error" class="note error">{{ error }}</p>
  </section>
</template>
