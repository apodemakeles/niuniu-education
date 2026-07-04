import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import LibraryView from '../LibraryView.vue'
import type { Word } from '@/types/word'

// mock fetchWords 避免真实网络请求
vi.mock('@/api/words', () => ({
  fetchWords: vi.fn(),
}))

import { fetchWords } from '@/api/words'

const mockWords: Word[] = [
  { id: '1', text: 'apple', meaningZh: '苹果', phonetic: '/ˈæpl/', wordType: 'new', status: 'unlearned' },
  { id: '2', text: 'read', meaningZh: '阅读', phonetic: '/riːd/', wordType: 'mistake', status: 'reinforce' },
  { id: '3', text: 'desk', meaningZh: '书桌', phonetic: '/desk/', wordType: 'new', status: 'learning' },
  { id: '4', text: 'water', meaningZh: '水', phonetic: '', wordType: 'new', status: 'mastered' },
]

function mountView() {
  const pinia = createPinia()
  setActivePinia(pinia)
  return mount(LibraryView, {
    global: { plugins: [pinia] },
  })
}

describe('LibraryView 筛选与统计', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    ;(fetchWords as ReturnType<typeof vi.fn>).mockResolvedValue(mockWords)
  })

  it('加载后显示统计：总数4 新词3 易错词1', async () => {
    const wrapper = mountView()
    await flushPromises() // 等 store.load() 完成

    const strongs = wrapper.findAll('.stats-grid strong').map((n) => n.text())
    expect(strongs).toEqual(['4', '3', '1'])
  })

  it('默认显示全部单词（4 行）', async () => {
    const wrapper = mountView()
    await flushPromises()
    const rows = wrapper.findAll('tbody tr')
    expect(rows.length).toBe(4)
  })

  it('点击"易错词"分类只显示易错词', async () => {
    const wrapper = mountView()
    await flushPromises()

    // 分类卡第3个是"易错词"
    const cards = wrapper.findAll('.library-card')
    expect(cards.length).toBe(3)
    await cards[2].trigger('click') // mistake

    await flushPromises()
    const rows = wrapper.findAll('tbody tr')
    expect(rows.length).toBe(1)
    expect(rows[0].text()).toContain('read')
  })

  it('状态筛选选"已掌握"只显示 mastered', async () => {
    const wrapper = mountView()
    await flushPromises()

    // 改状态下拉
    await wrapper.find('select').setValue('mastered')
    const rows = wrapper.findAll('tbody tr')
    expect(rows.length).toBe(1)
    expect(rows[0].text()).toContain('water')
  })

  it('加载失败显示错误', async () => {
    ;(fetchWords as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('服务不可用'))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('加载失败')
  })
})
