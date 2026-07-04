import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import LibraryView from '../LibraryView.vue'
import type { Word } from '@/types/word'
import type { WordListResult } from '@/api/words'

vi.mock('@/api/words', () => ({
  fetchWords: vi.fn(),
  deleteWord: vi.fn(),
}))

import { fetchWords } from '@/api/words'

const mockWords: Word[] = [
  { id: '1', text: 'apple', meaningZh: '苹果', phonetic: '/ˈæpl/', wordType: 'new', status: 'unlearned' },
  { id: '2', text: 'read', meaningZh: '阅读', phonetic: '/riːd/', wordType: 'mistake', status: 'reinforce' },
  { id: '3', text: 'desk', meaningZh: '书桌', phonetic: '/desk/', wordType: 'new', status: 'learning' },
  { id: '4', text: 'water', meaningZh: '水', phonetic: '', wordType: 'new', status: 'mastered' },
]

function listResult(data: Word[], total = data.length): WordListResult {
  return {
    data,
    pagination: { page: 1, pageSize: 20, total },
    stats: {
      total: 4,
      newWords: 3,
      mistakeWords: 1,
    },
  }
}

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
    ;(fetchWords as ReturnType<typeof vi.fn>).mockResolvedValue(listResult(mockWords))
  })

  it('加载后显示统计：总数4 新词3 易错词1', async () => {
    const wrapper = mountView()
    await flushPromises()

    const strongs = wrapper.findAll('.stats-grid strong').map((n) => n.text())
    expect(strongs).toEqual(['4', '3', '1'])
  })

  it('默认显示全部单词（4 行）', async () => {
    const wrapper = mountView()
    await flushPromises()
    const rows = wrapper.findAll('tbody tr')
    expect(rows.length).toBe(4)
  })

  it('点击"易错词"分类触发服务端筛选', async () => {
    const wrapper = mountView()
    await flushPromises()

    ;(fetchWords as ReturnType<typeof vi.fn>).mockResolvedValue(
      listResult([mockWords[1]], 1),
    )
    const cards = wrapper.findAll('.library-card')
    await cards[2].trigger('click')
    await flushPromises()

    expect(fetchWords).toHaveBeenLastCalledWith(
      expect.objectContaining({ type: 'mistake', page: 1 }),
    )
    const rows = wrapper.findAll('tbody tr')
    expect(rows.length).toBe(1)
    expect(rows[0].text()).toContain('read')
  })

  it('重复点击同一分类不重复请求', async () => {
    const wrapper = mountView()
    await flushPromises()
    const calls = (fetchWords as ReturnType<typeof vi.fn>).mock.calls.length

    await wrapper.findAll('.library-card')[0].trigger('click')
    await flushPromises()
    expect((fetchWords as ReturnType<typeof vi.fn>).mock.calls.length).toBe(calls)
    wrapper.unmount()
  })

  it('状态筛选触发服务端查询', async () => {
    const wrapper = mountView()
    await flushPromises()

    ;(fetchWords as ReturnType<typeof vi.fn>).mockResolvedValue(
      listResult([mockWords[3]], 1),
    )
    await wrapper.find('select').setValue('mastered')
    await flushPromises()

    expect(fetchWords).toHaveBeenLastCalledWith(
      expect.objectContaining({ status: 'mastered', page: 1 }),
    )
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
