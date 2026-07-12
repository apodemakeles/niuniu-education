<script setup lang="ts">
import type { TodayResponse, CardDetailResponse } from '@/types/practice'

defineProps<{
  today: TodayResponse
  card: CardDetailResponse | null
  listening: boolean
  readDoneSaving: boolean
}>()

const emit = defineEmits<{
  listen: []
  readDone: []
  hard: []
  prev: []
  next: []
  selectCard: [index: number]
}>()
</script>

<template>
  <section class="kid-view active" id="cardsView">
    <div class="kid-card-layout">
      <aside class="kid-queue-panel">
        <h2>学习队列</h2>
        <div class="kid-queue-list">
          <button
            v-for="(w, i) in [...today.firstPool, ...today.reviewPool]"
            :key="w.id"
            class="kid-queue-item"
            :class="{ active: card?.cardIndex === i, done: w.readStatus === 'read_done' }"
            type="button"
            @click="emit('selectCard', i)"
          >
            <strong>{{ i + 1 }}. {{ w.text }}</strong>
            <span>{{ w.meaningZh }} · {{ w.typeLabel }} · {{ w.learningLabel }}</span>
          </button>
        </div>
      </aside>

      <section class="kid-word-card" v-if="card">
        <div class="kid-card-top">
          <span>{{ card.cardIndex + 1 }} / {{ card.total }}</span>
          <span class="kid-pill">{{ card.word.typeLabel }} · {{ card.word.stage }} · {{ card.word.learningLabel }}</span>
        </div>
        <p class="kid-phonetic">{{ card.word.phonetic || '暂无音标' }}</p>
        <h2>{{ card.word.text }}</h2>
        <p class="kid-meaning">{{ card.word.meaningZh }}</p>
        <div class="kid-example-box">
          <span>例句<span v-if="card.exampleMissing">（待补充）</span></span>
          <p>{{ card.example }}</p>
        </div>
        <div class="kid-card-controls">
          <p class="kid-hint">
            {{ listening ? '正在准备英式发音，第一次可能需要几秒…' : card.readDone ? '✓ 这一个已经读完了，可以继续下一个。' : card.listened ? '现在自己读一遍，然后点“我读完了”。' : '先听一遍读音，再自己读出来。' }}
          </p>
          <div class="kid-learning-actions" aria-label="当前单词学习操作">
            <button class="sound-btn" type="button" :disabled="listening" :aria-busy="listening" @click="emit('listen')">
              {{ listening ? '正在准备读音…' : '听读音' }}
            </button>
            <button class="primary-btn" type="button" :disabled="!card.listened || readDoneSaving" @click="emit('readDone')">
              {{ readDoneSaving ? '正在记录…' : card.readDone ? '✓ 已读完' : '我读完了' }}
            </button>
            <button class="secondary-btn" type="button" @click="emit('hard')">不会读</button>
          </div>
          <div class="kid-card-navigation" aria-label="切换单词">
            <button class="secondary-btn" type="button" :disabled="card.isFirstCard" @click="emit('prev')">上一个</button>
            <span>切换单词</span>
            <button class="secondary-btn" type="button" @click="emit('next')">
              {{ card.isLastCard ? '去阅读' : '下一个' }}
            </button>
          </div>
        </div>
      </section>
    </div>
  </section>
</template>
