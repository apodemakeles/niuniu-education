import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import TodayDashboard from '../TodayDashboard.vue'
import type { TodayResponse } from '@/types/practice'

const completedToday: TodayResponse = {
  date: '2026-07-12',
  loadDay: 'normal',
  loadDayLabel: '普通日',
  loadDayDesc: '今天复习量适中，按顺序完成就好。',
  firstPool: [],
  reviewPool: [],
  newPoolCount: 3,
  reviewPoolMax: 9,
  totalCount: 1,
  empty: false,
  completed: true,
}

describe('TodayDashboard 已完成任务', () => {
  it('当天完成后显示复盘入口，而不是重新开始学习', async () => {
    const wrapper = mount(TodayDashboard, { props: { today: completedToday } })

    expect(wrapper.text()).toContain('今天已经完成啦')
    const button = wrapper.get('button.primary-btn')
    expect(button.text()).toBe('今日已完成，查看复盘')

    await button.trigger('click')
    expect(wrapper.emitted('start')).toHaveLength(1)
  })
})
