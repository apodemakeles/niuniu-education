<script setup lang="ts">
import type { TodayResponse, MissionWord } from '@/types/practice'

defineProps<{
  today: TodayResponse
}>()

const emit = defineEmits<{
  start: []
}>()

function laneClass(word: MissionWord) {
  return [word.wordType, word.poolType]
}
</script>

<template>
  <section class="kid-view active" id="dashboardView">
    <section class="kid-hero-panel">
      <div class="kid-hero-main">
        <div class="kid-hero-copy">
          <p class="small-label">今日任务</p>
          <h2>{{ today.completed ? '今天已经完成啦' : '先听，再读，最后默写' }}</h2>
          <div class="kid-load-tag" :class="today.loadDay">
            <strong>{{ today.loadDayLabel }}</strong>
            <span>{{ today.loadDayDesc }}</span>
          </div>
          <button class="primary-btn" type="button" :disabled="today.totalCount === 0" @click="emit('start')">
            {{ today.totalCount === 0 ? '暂无任务' : (today.completed ? '今日已完成，查看复盘' : '开始学习') }}
          </button>
        </div>
        <div class="kid-study-visual" aria-hidden="true">
          <div class="kid-orbit kid-orbit-one"></div>
          <div class="kid-orbit kid-orbit-two"></div>
          <div class="kid-book-shape">
            <span>Aa</span>
          </div>
        </div>
      </div>

      <div class="kid-mission-section">
        <div class="kid-panel-head">
          <h2>任务词</h2>
          <div class="kid-counts">
            <span>新词池 {{ today.firstPool.length }}/{{ today.newPoolCount }}</span>
            <span>非新词池 {{ today.reviewPool.length }}/{{ today.reviewPoolMax }}</span>
          </div>
        </div>
        <div class="kid-empty" v-if="today.empty">
          <h2>词库还是空的</h2>
          <p>{{ today.emptyHint }}</p>
        </div>
        <div v-else>
          <div class="kid-mission-lane">
            <div class="kid-lane-title">
              <strong>新词池</strong>
              <span>从未学状态随机抽取，最多 {{ today.newPoolCount }} 个</span>
            </div>
            <div class="kid-word-grid first-pool">
              <article v-for="w in today.firstPool" :key="w.id" class="kid-mission-word" :class="laneClass(w)">
                <strong>{{ w.text }}</strong>
                <span>{{ w.meaningZh }} · {{ w.phonetic || '暂无音标' }}</span>
                <span>{{ w.typeLabel }}标签 · {{ w.stage }}</span>
                <span>状态：{{ w.learningLabel }} · 连对 {{ w.readStatus === 'read_done' ? '已读' : '' }}</span>
              </article>
              <p v-if="today.firstPool.length === 0" class="kid-note">今天没有新词</p>
            </div>
          </div>
          <div class="kid-mission-lane">
            <div class="kid-lane-title">
              <strong>非新词池</strong>
              <span>从学习中、需强化、已掌握中按分数选择，最多 {{ today.reviewPoolMax }} 个</span>
            </div>
            <div class="kid-word-grid review-pool">
              <article v-for="w in today.reviewPool" :key="w.id" class="kid-mission-word" :class="laneClass(w)">
                <strong>{{ w.text }}</strong>
                <span>{{ w.meaningZh }} · {{ w.phonetic || '暂无音标' }}</span>
                <span>{{ w.typeLabel }}标签 · {{ w.stage }}</span>
                <span>状态：{{ w.learningLabel }}</span>
              </article>
              <p v-if="today.reviewPool.length === 0" class="kid-note">今天没有复习词</p>
            </div>
          </div>
        </div>
      </div>
    </section>
  </section>
</template>
