<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { DictationResponse, SubmitAnswer } from '@/types/practice'
import bonusStar from '@/assets/bonus-star.svg'

const props = defineProps<{
  dictation: DictationResponse | null
  submitted: boolean // 是否已首次提交
  bonusIds: string[] // 首次提交正确的 bonus 词
  correctionRequested: boolean // 完成页请求直接进入订正
  reviewMode: boolean // 今日完成后回看：只展示正确写法，不回放首次错误答案
}>()

const emit = defineEmits<{
  submit: [answers: SubmitAnswer[]]
  correct: [answers: SubmitAnswer[]]
  showHint: [] // 切换音标提示（记录 used_hint 由后端在提交时统一处理）
  goDone: [] // 已提交锁定后直接进入完成页
}>()

// 本地答案缓存：key = taskId
const answers = ref<Record<string, string>>({})
const showPhonetic = ref(false)
const correctionMode = ref(false)
const correctionTargets = ref<Set<string>>(new Set())

function needsCorrection(item: DictationResponse['items'][number]) {
  return (item.firstResult === 'wrong' || item.firstResult === 'blank')
    && item.correctionResult !== 'correct'
}

// 从 dictation 初始化答案（首次进入或刷新后）
watch(
  () => props.dictation,
  (d) => {
    if (!d) return
    const next: Record<string, string> = {}
    for (const it of d.items) {
      // 已锁定的用后端返回的首次答案；未锁定且本地无值则空
      if (it.locked) {
        next[it.taskId] = it.firstAnswer ?? ''
      } else {
        next[it.taskId] = answers.value[it.taskId] ?? ''
      }
    }
    answers.value = next
    // 订正过且正确的词不再展示；错误或未填写的订正仍要继续练。
    if (props.submitted) {
      correctionTargets.value = new Set(d.items.filter(needsCorrection).map((it) => it.taskId))
    }
    if (correctionMode.value && props.correctionRequested) {
      startCorrection()
    }
  },
  { immediate: true },
)

function togglePhonetic() {
  showPhonetic.value = !showPhonetic.value
  emit('showHint')
}

const allItems = computed(() => props.dictation?.items ?? [])
const correctionItems = computed(() => allItems.value.filter((it) => correctionTargets.value.has(it.taskId)))
const displayedItems = computed(() => (props.reviewMode || !correctionMode.value ? allItems.value : correctionItems.value))

// 是否所有词都已提交锁定（后端持久化状态，刷新或返回后仍正确）
const allLocked = computed(() => allItems.value.length > 0 && allItems.value.every((it) => it.locked))

function onSubmit() {
  const items = correctionMode.value ? correctionItems.value : allItems.value
  const payload: SubmitAnswer[] = items.map((it) => ({
    taskId: it.taskId,
    wordId: it.wordId,
    answer: answers.value[it.taskId] ?? '',
  }))
  if (correctionMode.value) {
    emit('correct', payload)
  } else {
    emit('submit', payload)
  }
}

function startCorrection() {
  // 只保留尚未订正正确的首次错词；已订正正确的词不必重复出现。
  correctionTargets.value = new Set(
    allItems.value
      .filter(needsCorrection)
      .map((it) => it.taskId),
  )
  // 进入订正：清空错词输入框，待孩子照影子字重写
  for (const it of correctionItems.value) {
    if (correctionTargets.value.has(it.taskId)) {
      answers.value[it.taskId] = ''
    }
  }
  correctionMode.value = true
}

function reviewAnswer(item: DictationResponse['items'][number]) {
  return item.firstResult === 'correct' ? item.firstAnswer : item.correctionAnswer
}

function reviewLabel(item: DictationResponse['items'][number]) {
  return item.firstResult === 'correct' ? '一次写对' : '后来订正正确'
}

function hasBonus(item: DictationResponse['items'][number]) {
  // bonusAwarded 来自后端持久化结果，刷新后仍可稳定展示；bonusIds 仅用于本次提交的即时反馈。
  // bonusIds 可能为 null（后端空切片 JSON 序列化为 null），需兜底。
  return item.bonusAwarded || (props.bonusIds ?? []).includes(item.wordId)
}

watch(
  () => props.correctionRequested,
  (requested) => {
    if (requested) startCorrection()
    else correctionMode.value = false
  },
  { immediate: true },
)

// 影子字：订正时显示正确拼写的浅绿提示。
// 注意：后端默写题面不返回英文单词（防止作弊），订正影子字需要从 daily_task 的预期答案获取。
// 这里通过 coveredWords 的 text 反查不到，故订正影子字在后端 correction 接口返回正确拼写。
// 为简化第一版：订正时不显示完整影子字，改为提示「照着读音再写一遍」。
// （PRD 的影子字需要后端在订正模式下返回正确拼写，本期占位用空影子字，留 TODO。）
function ghostSpelling(): string {
  return '' // TODO: 后端订正接口返回正确拼写后填充
}
</script>

<template>
  <section class="kid-view active" id="dictationView">
    <section class="kid-dictation-panel" v-if="dictation">
      <div class="kid-panel-head">
        <div>
          <p class="small-label">{{ reviewMode ? 'Review' : 'Dictation' }}</p>
          <h2>{{ reviewMode ? '今日复盘：记住正确写法' : '根据中文默写英文' }}</h2>
        </div>
        <button class="secondary-btn" type="button" v-if="!reviewMode" @click="togglePhonetic">
          {{ showPhonetic ? '隐藏音标提示' : '显示音标提示' }}
        </button>
      </div>
      <div class="kid-dictation-list">
        <div
          v-for="it in displayedItems"
          :key="it.taskId"
          class="kid-dictation-row"
          :class="{
            correct: !reviewMode && submitted && it.firstResult === 'correct',
            wrong: !reviewMode && submitted && (it.firstResult === 'wrong' || it.firstResult === 'blank'),
            correction: correctionMode && correctionTargets.has(it.taskId),
            review: reviewMode,
          }"
        >
          <div class="prompt">
            <strong>{{ it.meaningZh }}</strong>
            <span>{{ reviewMode || showPhonetic ? (it.phonetic || '暂无音标') : '音标已隐藏' }}</span>
          </div>
          <div class="kid-correction-input-shell" v-if="correctionMode && correctionTargets.has(it.taskId)">
            <span class="kid-ghost-spelling" aria-hidden="true">{{ ghostSpelling() }}</span>
            <input
              v-model="answers[it.taskId]"
              placeholder="照着读音再写一遍"
              :data-task="it.taskId"
            />
          </div>
          <div v-else-if="reviewMode" class="kid-review-answer">
            <span>正确写法</span>
            <strong>{{ reviewAnswer(it) }}</strong>
          </div>
          <div v-else>
            <input
              v-model="answers[it.taskId]"
              :readonly="it.locked && !correctionMode"
              placeholder="写出英文单词"
              :data-task="it.taskId"
            />
          </div>
          <span class="kid-result-label">
            <template v-if="reviewMode">
              <span>{{ reviewLabel(it) }}</span>
              <img
                v-if="it.firstResult === 'correct' && hasBonus(it)"
                class="kid-bonus-reward kid-review-bonus"
                :src="bonusStar"
                alt="首次默写拼对奖励"
              />
            </template>
            <img
              v-else-if="hasBonus(it)"
              class="kid-bonus-reward"
              :src="bonusStar"
              alt="首次默写拼对奖励"
            />
            <template v-else-if="correctionMode && it.correctionResult === 'wrong'">
              还不对，再试一次
            </template>
            <template v-else-if="correctionMode && it.correctionResult === 'blank'">
              还没有填写
            </template>
            <template v-else-if="submitted">
              {{ it.firstResult === 'correct' ? '正确' : it.firstResult === 'blank' ? '未填写' : '需订正' }}
            </template>
          </span>
        </div>
      </div>
      <div class="kid-dictation-actions">
        <template v-if="reviewMode">
          <p class="kid-review-note">今天的正确写法都在这里啦，明天再继续巩固。</p>
          <button class="primary-btn" type="button" @click="emit('goDone')">返回完成页</button>
        </template>
        <!-- 订正模式：提交订正 -->
        <template v-else-if="correctionMode">
          <button class="primary-btn" type="button" @click="onSubmit">提交订正</button>
        </template>
        <!-- 已全部提交锁定：不再允许提交，直接进入完成页 -->
        <template v-else-if="allLocked">
          <button class="primary-btn" type="button" @click="emit('goDone')">查看结果</button>
          <button
            class="secondary-btn"
            type="button"
            v-if="correctionTargets.size > 0"
            @click="startCorrection"
          >
            订正错词
          </button>
        </template>
        <!-- 未提交：正常提交默写 -->
        <template v-else>
          <button class="primary-btn" type="button" @click="onSubmit">提交默写</button>
        </template>
      </div>
    </section>
    <p v-else class="kid-note">默写题加载中…</p>
  </section>
</template>
