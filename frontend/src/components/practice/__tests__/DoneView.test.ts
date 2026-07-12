import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import DoneView from '../DoneView.vue'
import type { DoneResponse, MissionWord } from '@/types/practice'

function done(checkedDays: number[] = [], tomorrowReview: MissionWord[] = []): DoneResponse {
  return {
    date: '2026-07-12',
    total: 3,
    correctCount: 3,
    wrongCount: 0,
    blankCount: 0,
    usedHintCount: 0,
    tomorrowReview,
    needReinforce: [],
    dictationLocked: true,
    hasCorrection: false,
    checkin: {
      monthLabel: '7 月',
      checkedDays,
      todayDay: 12,
      streak: checkedDays.includes(12) ? 1 : 0,
      bonus7Day: false,
    },
  }
}

describe('DoneView 今日打卡', () => {
  it('未打卡时点击完成今日任务会通知父组件执行打卡', async () => {
    const wrapper = mount(DoneView, {
      props: { done: done(), bonusJustEarned: false, checkingIn: false },
    })

    await wrapper.get('button.primary-btn').trigger('click')

    expect(wrapper.emitted('finish')).toHaveLength(1)
  })

  it('当天已打卡时显示完成状态并禁止重复提交', () => {
    const wrapper = mount(DoneView, {
      props: { done: done([12]), bonusJustEarned: false, checkingIn: false },
    })
    const button = wrapper.get('button.primary-btn')

    expect(button.text()).toBe('今日已打卡')
    expect(button.attributes('disabled')).toBeDefined()
  })

  it('明天优先复习只展示单词和音标，不暴露学习状态', () => {
    const wrapper = mount(DoneView, {
      props: {
        done: done([], [{
          id: 'word-1',
          text: 'desk',
          meaningZh: '书桌',
          phonetic: '/desk/',
          wordType: 'mistake',
          typeLabel: '易错词',
          learningLabel: '需强化',
          stage: '复习',
          poolType: 'review',
          displayOrder: 0,
          readStatus: 'read_done',
        }]),
        bonusJustEarned: false,
        checkingIn: false,
      },
    })

    const tomorrow = wrapper.get('.kid-tomorrow-box')
    expect(tomorrow.text()).toContain('desk · /desk/')
    expect(tomorrow.text()).not.toContain('需强化')
  })
})
