import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ReadingView from '../ReadingView.vue'
import type { ReadingResponse } from '@/types/practice'

function reading(elapsedSeconds: number): ReadingResponse {
  return {
    date: '2026-07-12',
    title: 'A Day at School',
    text: 'A short story.',
    rawText: 'A short story.',
    sceneHint: '',
    coveredWords: [],
    appearances: [],
    status: 'success',
    minSeconds: 120,
    startedAt: '2026-07-12 00:00:00',
    elapsedSeconds,
    canFinish: false,
  }
}

describe('ReadingView 阅读计时', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('隐藏挂载期间不提前累计阅读时间', async () => {
    const wrapper = mount(ReadingView, {
      props: { reading: null, active: false },
    })

    vi.advanceTimersByTime(90_000)
    await wrapper.setProps({ reading: reading(0), active: true })
    expect(wrapper.text()).toContain('还需阅读 120 秒')

    vi.advanceTimersByTime(1_000)
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('还需阅读 119 秒')
  })

  it('收到新的服务端数据时重新校准本地倒计时', async () => {
    const wrapper = mount(ReadingView, {
      props: { reading: reading(30), active: true },
    })

    vi.advanceTimersByTime(5_000)
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('还需阅读 85 秒')

    await wrapper.setProps({ reading: reading(32) })
    expect(wrapper.text()).toContain('还需阅读 88 秒')
  })

  it('离开阅读页后暂停本地累计', async () => {
    const wrapper = mount(ReadingView, {
      props: { reading: reading(100), active: true },
    })

    await wrapper.setProps({ active: false })
    vi.advanceTimersByTime(30_000)
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('还需阅读 20 秒')
    expect(wrapper.find('button.primary-btn').attributes('disabled')).toBeDefined()
  })

  it('调试模式下服务端 canFinish 时立即开放完成按钮', async () => {
    const wrapper = mount(ReadingView, {
      props: {
        reading: {
          ...reading(0),
          canFinish: true,
          debugMode: true,
        },
        active: true,
      },
    })

    expect(wrapper.text()).toContain('调试模式：可直接完成阅读')
    expect(wrapper.find('button.primary-btn').attributes('disabled')).toBeUndefined()
  })
})
