import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import DictationView from '../DictationView.vue'
import type { DictationResponse } from '@/types/practice'

const dictation: DictationResponse = {
  date: '2026-07-12',
  items: [
    {
      taskId: 'correct-task', wordId: 'correct-word', meaningZh: '苹果', poolType: 'first',
      firstAnswer: 'apple', firstResult: 'correct', locked: true,
      correctionResult: 'not_submitted', bonusEligible: false, bonusAwarded: false,
    },
    {
      taskId: 'wrong-task', wordId: 'wrong-word', meaningZh: '书桌', poolType: 'first',
      firstAnswer: 'desl', firstResult: 'wrong', locked: true,
      correctionResult: 'not_submitted', bonusEligible: false, bonusAwarded: false,
    },
  ],
}

describe('DictationView 完成页错词订正', () => {
  it('进入订正后只显示首次错误词，并提交该词的新答案', async () => {
    const wrapper = mount(DictationView, {
      props: {
        dictation,
        submitted: true,
        bonusIds: [],
        correctionRequested: true,
        reviewMode: false,
      },
    })

    expect(wrapper.text()).toContain('书桌')
    expect(wrapper.text()).not.toContain('苹果')
    expect(wrapper.get('input').element.value).toBe('')

    await wrapper.get('input').setValue('desk')
    await wrapper.get('button.primary-btn').trigger('click')

    expect(wrapper.emitted('correct')).toEqual([[
      [{ taskId: 'wrong-task', wordId: 'wrong-word', answer: 'desk' }],
    ]])
  })

  it('订正错误的词会继续展示，已订正正确的词不会重复出现', () => {
    const wrapper = mount(DictationView, {
      props: {
        dictation: {
          ...dictation,
          items: [
            {
              ...dictation.items[0],
              taskId: 'fixed-task',
              meaningZh: '已经订正正确',
              firstResult: 'wrong',
              correctionResult: 'correct',
            },
            {
              ...dictation.items[1],
              taskId: 'retry-task',
              meaningZh: '仍需订正',
              correctionResult: 'wrong',
            },
          ],
        },
        submitted: true,
        bonusIds: [],
        correctionRequested: true,
        reviewMode: false,
      },
    })

    expect(wrapper.text()).not.toContain('已经订正正确')
    expect(wrapper.text()).toContain('仍需订正')
    expect(wrapper.text()).toContain('还不对，再试一次')
    expect(wrapper.findAll('input')).toHaveLength(1)
  })

  it('今日复盘只展示正确写法，不回放首次错误拼写', () => {
    const wrapper = mount(DictationView, {
      props: {
        dictation: {
          ...dictation,
          items: [
            {
              ...dictation.items[0],
              firstAnswer: 'apple',
              firstResult: 'correct',
              correctionResult: 'not_submitted',
              bonusAwarded: true,
              phonetic: '/æpl/',
            },
            {
              ...dictation.items[1],
              firstAnswer: 'desl',
              correctionAnswer: 'desk',
              correctionResult: 'correct',
            },
          ],
        },
        submitted: true,
        bonusIds: [],
        correctionRequested: false,
        reviewMode: true,
      },
    })

    expect(wrapper.text()).toContain('一次写对')
    expect(wrapper.text()).toContain('后来订正正确')
    expect(wrapper.text()).toContain('今日复盘：记住正确写法')
    expect(wrapper.text()).toContain('apple')
    expect(wrapper.text()).toContain('desk')
    expect(wrapper.text()).toContain('/æpl/')
    expect(wrapper.text()).not.toContain('desl')
    expect(wrapper.findAll('input')).toHaveLength(0)
    expect(wrapper.get('img[alt="首次默写拼对奖励"]').classes()).toContain('kid-review-bonus')
  })
})
