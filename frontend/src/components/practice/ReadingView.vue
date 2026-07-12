<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import type { ReadingResponse } from '@/types/practice'
import { speakWord } from '@/stores/practice'

const props = defineProps<{
  reading: ReadingResponse | null
  active: boolean
}>()

const emit = defineEmits<{
  finish: []
  regenerate: []
}>()

// 服务端 elapsedSeconds 是计时基准；本地只在阅读页可见且短文就绪后推进显示。
const elapsed = ref(props.reading?.elapsedSeconds ?? 0)
let timer: ReturnType<typeof setInterval> | null = null

watch(
  () => [props.reading?.startedAt, props.reading?.elapsedSeconds] as const,
  () => {
    elapsed.value = props.reading?.elapsedSeconds ?? 0
  },
)

onMounted(() => {
  elapsed.value = props.reading?.elapsedSeconds ?? 0
  timer = setInterval(() => {
    if (props.active && props.reading?.status === 'success') {
      elapsed.value += 1
    }
  }, 1000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})

const remaining = computed(() => {
  const min = props.reading?.minSeconds ?? 120
  return Math.max(0, min - elapsed.value)
})

const canFinish = computed(() => remaining.value <= 0 || props.reading?.canFinish)

function listenKeyWords() {
  if (!props.reading) return
  const words = props.reading.appearances.map((a) => a.text).join('. ')
  speakWord(words, 0.75)
}
</script>

<template>
  <section class="kid-view active" id="readingView">
    <section class="kid-reading-panel" v-if="reading">
      <div class="kid-panel-head">
        <div>
          <p class="small-label">Reading</p>
          <h2>{{ reading.title || '今日短文' }}</h2>
        </div>
        <button class="secondary-btn" type="button" @click="listenKeyWords">再听重点词</button>
      </div>

      <div v-if="reading.status === 'pending'" class="kid-reading-generating" aria-live="polite">
        <span class="kid-loading-dot" aria-hidden="true"></span>
        <div><strong>AI 正在准备今日短文</strong><p>你可以先完成单词卡，过一会儿再回来。</p></div>
        <button class="secondary-btn" type="button" @click="emit('regenerate')">重新生成</button>
      </div>

      <div v-else-if="reading.status === 'failed'">
        <p class="kid-scene-note">阅读短文暂时生成失败，可以稍后重试，或让家长允许后跳过进入默写。</p>
        <button class="primary-btn" type="button" @click="emit('regenerate')">重新生成</button>
      </div>

      <template v-else>
        <p class="kid-scene-note" v-if="reading.sceneHint">中文提示：{{ reading.sceneHint }}</p>
        <article class="kid-story" v-html="reading.text"></article>
        <div class="kid-appearances">
          <span v-for="ap in reading.appearances" :key="ap.id">{{ ap.text }} 出现 {{ ap.count }} 次</span>
        </div>
        <div class="kid-reading-finish">
          <div class="kid-timer" v-if="!canFinish">
            ⏳ 还需阅读 {{ remaining }} 秒
          </div>
          <div class="kid-timer kid-timer-ready" v-else>
            ✓ 已阅读够时间，可以完成了
          </div>
          <button class="primary-btn" type="button" :disabled="!canFinish" @click="emit('finish')">
            {{ canFinish ? '我读完短文了' : `继续阅读（还需 ${remaining} 秒）` }}
          </button>
        </div>
      </template>
    </section>
    <p v-else class="kid-note">短文加载中…</p>
  </section>
</template>
