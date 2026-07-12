<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { usePracticeStore } from '@/stores/practice'
import type { PracticeView } from '@/stores/practice'
import type { SubmitAnswer } from '@/types/practice'
import TodayDashboard from '@/components/practice/TodayDashboard.vue'
import WordCardStudy from '@/components/practice/WordCardStudy.vue'
import ReadingView from '@/components/practice/ReadingView.vue'
import DictationView from '@/components/practice/DictationView.vue'
import DoneView from '@/components/practice/DoneView.vue'

const store = usePracticeStore()

// 默写提交状态
const submitted = ref(false)
const bonusIds = ref<string[]>([])
const correctionRequested = ref(false)
const reviewMode = ref(false)
const taskCompleted = ref(false)
// 打卡 bonus 触发
const bonusJustEarned = ref(false)
const checkingIn = ref(false)

// 学生端自带 toast（不依赖家长端 Toast 组件）
const toastMsg = ref('')
const toastVisible = ref(false)
let toastTimer: ReturnType<typeof setTimeout> | null = null
function showToast(msg: string) {
  toastMsg.value = msg
  toastVisible.value = true
  if (toastTimer) clearTimeout(toastTimer)
  toastTimer = setTimeout(() => (toastVisible.value = false), 2200)
}

const steps: { view: PracticeView; label: string }[] = [
  { view: 'dashboard', label: '今日任务' },
  { view: 'cards', label: '单词卡' },
  { view: 'reading', label: '延伸阅读' },
  { view: 'dictation', label: '默写' },
  { view: 'done', label: '完成' },
]

function stepLabel(step: { view: PracticeView; label: string }) {
  return step.view === 'dictation' && taskCompleted.value ? '今日复盘' : step.label
}

function todayDateText() {
  return new Date().toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', weekday: 'long' })
}

function stepClick(v: PracticeView) {
  if (store.unlockedViews.has(v)) {
    reviewMode.value = v === 'dictation' && taskCompleted.value
    store.switchView(v)
    if (v === 'cards' && store.card) {
      // 回到单词卡时不重新 enter
    }
    if (v === 'reading') store.loadReading()
    if (v === 'dictation') store.loadDictation()
    if (v === 'done') {
      reviewMode.value = false
      store.loadDone()
    }
  }
}

async function goDone() {
  reviewMode.value = false
  await store.goDone()
  taskCompleted.value = true
}

// --- 事件处理 ---

async function onStart() {
  if (store.today?.completed) {
    store.unlock('dictation')
    await goDone()
    return
  }
  store.resetPracticeSession()
  submitted.value = false
  bonusIds.value = []
  correctionRequested.value = false
  reviewMode.value = false
  taskCompleted.value = false
  await store.startStudy()
}

async function onSubmit(answers: SubmitAnswer[]) {
  const resp = await store.submit(answers)
  if (resp) {
    submitted.value = true
    bonusIds.value = resp.bonusIds
    const wrong = resp.wrong + resp.blank
    if (wrong === 0) {
      correctionRequested.value = false
      reviewMode.value = false
      showToast('全部写对了！')
      await goDone()
      return
    }

    // 首次结果已经锁定；必须完成错词订正，才允许进入完成页和打卡。
    correctionRequested.value = true
    store.switchView('dictation')
    showToast(`${wrong} 个词需要订正，订正后才能完成今日任务`)
  }
}

async function onCorrect(answers: SubmitAnswer[]) {
  const resp = await store.correct(answers)
  if (!resp) {
    showToast(store.error ?? '订正失败，请再试一次')
    return
  }

  const remaining = resp.items.filter(
    (item) =>
      (item.firstResult === 'wrong' || item.firstResult === 'blank')
      && item.correctionResult !== 'correct',
  ).length ?? 0
  if (remaining > 0) {
    showToast(`${remaining} 个词还没有订正正确，再试一次`)
    return
  }

  correctionRequested.value = false
  reviewMode.value = false
  showToast('全部订正正确，今日任务完成！')
  await goDone()
}

function onRetryWrong() {
  // 首次默写结果必须保持锁定；这里只请求默写页进入订正状态。
  submitted.value = true
  correctionRequested.value = true
  reviewMode.value = false
  store.switchView('dictation')
}

// 已提交锁定的默写页直接进入完成页
async function onGoDoneFromDictation() {
  const hasPendingCorrection = store.dictation?.items.some(
    (item) =>
      (item.firstResult === 'wrong' || item.firstResult === 'blank')
      && item.correctionResult !== 'correct',
  ) ?? false
  if (hasPendingCorrection) {
    correctionRequested.value = true
    showToast('请先订正错词，再完成今日任务')
    return
  }
  submitted.value = true
  await goDone()
}

async function onFinishReading() {
  const ok = await store.finishReading()
  if (!ok) {
    showToast(store.error ?? '继续阅读一会儿再完成')
  }
}

async function onListen() {
  const ok = await store.listenCurrent()
  if (!ok && store.error) showToast(store.error)
}

async function onReadDone() {
  const ok = await store.readDoneCurrent()
  showToast(ok ? '读得真棒，已经记录啦！' : (store.error ?? '记录失败，请再试一次'))
}

async function onCheckin() {
  if (checkingIn.value) return
  const checkedInToday = store.done?.checkin.checkedDays.includes(store.done.checkin.todayDay) ?? false
  if (checkedInToday) {
    showToast('今天已经打过卡啦')
    return
  }

  checkingIn.value = true
  try {
    const resp = await store.doCheckin()
    if (!resp) {
      showToast(store.error ?? '打卡失败，请再试一次')
      return
    }
    if (resp.summary.bonus7Day) {
      bonusJustEarned.value = true
    } else {
      showToast('打卡成功！')
    }
  } finally {
    checkingIn.value = false
  }
}

onMounted(() => {
  store.loadToday()
})
</script>

<template>
  <div class="kid-app">
    <div class="kid-container">
      <header class="kid-topbar">
        <div>
          <p class="kid-eyebrow">Daily Word Mission</p>
          <h1>今日背单词</h1>
        </div>
        <div class="kid-date-card">
          <span>{{ todayDateText() }}</span>
          <strong>{{ store.today?.totalCount ?? 0 }} 个词</strong>
        </div>
      </header>

      <nav class="kid-stepper" aria-label="学习流程">
        <button
          v-for="(s, i) in steps"
          :key="s.view"
          class="kid-step"
          :class="{ active: store.view === s.view }"
          type="button"
          :hidden="!store.unlockedViews.has(s.view)"
          :disabled="!store.unlockedViews.has(s.view)"
          @click="stepClick(s.view)"
        >
          <span>{{ i + 1 }}</span>
          {{ stepLabel(s) }}
        </button>
      </nav>

      <p v-if="store.loading && !store.today" class="kid-note">加载中…</p>
      <p v-else-if="store.error && !store.today" class="kid-note error">加载失败：{{ store.error }}</p>

      <template v-else-if="store.today">
        <TodayDashboard
          v-show="store.view === 'dashboard'"
          :today="store.today"
          @start="onStart"
        />

        <WordCardStudy
          v-show="store.view === 'cards'"
          :today="store.today"
          :card="store.card"
          :listening="store.listening"
          :read-done-saving="store.readDoneSaving"
          @listen="onListen"
          @read-done="onReadDone"
          @hard="() => { store.hardCurrent(); showToast('已标记为需要强化') }"
          @prev="store.prevCard()"
          @next="store.nextCard()"
          @select-card="(i: number) => store.loadCardByIndex(i)"
        />

        <ReadingView
          v-show="store.view === 'reading'"
          :reading="store.reading"
          :active="store.view === 'reading'"
          @finish="onFinishReading"
          @regenerate="store.regenerate()"
        />

        <DictationView
          v-show="store.view === 'dictation'"
          :dictation="store.dictation"
          :submitted="submitted"
          :bonus-ids="bonusIds"
          :correction-requested="correctionRequested"
          :review-mode="reviewMode"
          @submit="onSubmit"
          @correct="onCorrect"
          @go-done="onGoDoneFromDictation"
        />

        <DoneView
          v-show="store.view === 'done'"
          :done="store.done"
          :bonus-just-earned="bonusJustEarned"
          :checking-in="checkingIn"
          @finish="onCheckin"
          @retry-wrong="onRetryWrong"
          @bonus-seen="bonusJustEarned = false"
        />
      </template>

      <!-- 学生端 toast -->
      <div class="kid-toast" :class="{ show: toastVisible }" role="status" aria-live="polite">
        {{ toastMsg }}
      </div>
    </div>
  </div>
</template>
