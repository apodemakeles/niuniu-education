<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { DoneResponse } from '@/types/practice'
import bonusStar from '@/assets/bonus-star.svg'

const props = defineProps<{
  done: DoneResponse | null
  bonusJustEarned: boolean // 父组件打卡后置 true，触发弹窗
  checkingIn: boolean
}>()

const emit = defineEmits<{
  finish: [] // 完成今日任务（打卡）
  retryWrong: [] // 回默写页订正
  bonusSeen: [] // bonus 弹窗已关闭，父组件重置 bonusJustEarned
}>()

const showBonus = ref(false)

const checkedInToday = computed(() => {
  if (!props.done) return false
  return props.done.checkin.checkedDays.includes(props.done.checkin.todayDay)
})

// 父组件置 bonusJustEarned=true 时弹窗
watch(
  () => props.bonusJustEarned,
  (v) => {
    if (v) showBonus.value = true
  },
)

// 本月日历格：1~月底
const calendarDays = computed(() => {
  if (!props.done) return []
  const now = new Date()
  const year = now.getFullYear()
  const month = now.getMonth() + 1
  const daysInMonth = new Date(year, month, 0).getDate()
  const checkedSet = new Set(props.done.checkin.checkedDays)
  return Array.from({ length: daysInMonth }, (_, i) => {
    const day = i + 1
    return {
      day,
      checked: checkedSet.has(day),
      isToday: day === props.done!.checkin.todayDay,
    }
  })
})

function closeBonus() {
  showBonus.value = false
  emit('bonusSeen')
}

function onFinish() {
  // 已打卡或请求进行中时不重复发起请求；父组件仍会作为最终处理入口。
  if (checkedInToday.value || props.checkingIn) return
  emit('finish')
}
</script>

<template>
  <section class="kid-view active" id="doneView" v-if="done">
    <section class="kid-done-panel">
      <div class="kid-badge-mark">✓</div>
      <p class="small-label">Finished</p>
      <h2>今日任务完成</h2>
      <p class="kid-hint">完成 {{ done.total }} 个单词学习，首次默写正确 {{ done.correctCount }} 个。</p>
      <div class="kid-result-grid">
        <div>
          <span>默写正确</span>
          <strong>{{ done.correctCount }}</strong>
        </div>
        <div>
          <span>需要强化</span>
          <strong>{{ done.wrongCount }}</strong>
        </div>
        <div>
          <span>使用提示</span>
          <strong>{{ done.usedHintCount }}</strong>
        </div>
      </div>

      <div class="kid-tomorrow-box" v-if="done.tomorrowReview.length">
        <h3>明天优先复习</h3>
        <div>
          <span v-for="w in done.tomorrowReview" :key="w.id">
            {{ w.text }} · {{ w.phonetic || '暂无音标' }}
          </span>
        </div>
      </div>

      <div class="kid-checkin-box">
        <div class="kid-checkin-head">
          <div>
            <h3>本月打卡</h3>
            <p>连续打卡 {{ done.checkin.streak }} 天</p>
          </div>
          <span>{{ done.checkin.monthLabel }}</span>
        </div>
        <div class="kid-checkin-grid">
          <span
            v-for="d in calendarDays"
            :key="d.day"
            class="kid-checkin-day"
            :class="{ checked: d.checked, today: d.isToday }"
          >
            {{ d.day }}
          </span>
        </div>
      </div>

      <div class="kid-dictation-actions" style="justify-content: center">
        <button
          class="secondary-btn"
          type="button"
          v-if="done.needReinforce.length && !done.hasCorrection"
          @click="emit('retryWrong')"
        >
          再练一遍错词
        </button>
        <button
          class="primary-btn"
          type="button"
          :disabled="checkedInToday || checkingIn"
          @click="onFinish"
        >
          {{ checkingIn ? '正在打卡…' : (checkedInToday ? '今日已打卡' : '完成今日任务') }}
        </button>
      </div>
    </section>

    <div class="kid-bonus-modal-backdrop" v-if="showBonus">
      <section class="kid-bonus-modal" role="dialog" aria-modal="true" aria-labelledby="checkinBonusTitle">
        <img :src="bonusStar" alt="连续打卡奖励" />
        <p class="small-label">Check-in Bonus</p>
        <h2 id="checkinBonusTitle">连续 7 天打卡</h2>
        <p>今天也完成啦，获得一枚连续打卡 bonus。</p>
        <button class="primary-btn" type="button" @click="closeBonus">收下奖励</button>
      </section>
    </div>
  </section>
  <p v-else class="kid-note">加载完成页…</p>
</template>
