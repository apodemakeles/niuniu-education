import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import DraftTable from '../DraftTable.vue'
import type { DraftRow } from '@/types/draft'

// mock confirmImport API
vi.mock('@/api/import', () => ({
  confirmImport: vi.fn(),
}))

import { confirmImport } from '@/api/import'

function makeRows(): DraftRow[] {
  return [
    { rowId: 'a', text: 'apple', meaningZh: '苹果', phonetic: '/ˈæpl/', wordType: 'new', confidence: 0.9, issues: [] },
    { rowId: 'b', text: '', meaningZh: '', phonetic: '', wordType: 'new', confidence: 0, issues: ['invalid'] },
    { rowId: 'c', text: 'window', meaningZh: '窗户', phonetic: '', wordType: 'new', confidence: 0.8, issues: [] },
  ]
}

describe('DraftTable', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('点击删除会移除对应行', async () => {
    const rows = makeRows()
    const wrapper = mount(DraftTable, { props: { rows } })
    const deleteBtns = wrapper.findAll('button.link-btn')
    expect(deleteBtns.length).toBe(3)
    await deleteBtns[0].trigger('click')
    // 第一行被删除，剩余 2 行
    expect(wrapper.findAll('tbody tr').length).toBe(2)
  })

  it('确认入库时过滤掉英文单词为空的行，并调用 confirmImport', async () => {
    const mockResp = { added: 2, skipped: 0, invalid: 0, details: [] }
    ;(confirmImport as ReturnType<typeof vi.fn>).mockResolvedValue(mockResp)

    const rows = makeRows()
    const wrapper = mount(DraftTable, { props: { rows } })

    // 点击"确认入库"
    await wrapper.find('button.primary-btn').trigger('click')
    await flushPromises()

    // confirmImport 应被调用，且只传 2 行（apple + window，空文本行被过滤）
    expect(confirmImport).toHaveBeenCalledTimes(1)
    const arg = (confirmImport as ReturnType<typeof vi.fn>).mock.calls[0][0]
    expect(arg.length).toBe(2)
    expect(arg[0].text).toBe('apple')
    expect(arg[1].text).toBe('window')

    // 应 emit confirmed 事件带结果
    const emitted = wrapper.emitted('confirmed')
    expect(emitted).toBeTruthy()
    expect(emitted![0][0]).toEqual(mockResp)
  })

  it('全部行英文为空时给出错误提示，不调用 API', async () => {
    const rows: DraftRow[] = [
      { rowId: 'a', text: '', meaningZh: '', phonetic: '', wordType: 'new', confidence: 0, issues: [] },
    ]
    const wrapper = mount(DraftTable, { props: { rows } })
    await wrapper.find('button.primary-btn').trigger('click')
    await flushPromises()

    expect(confirmImport).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('没有可入库的行')
  })

  it('低置信度行显示提示标记', () => {
    const rows: DraftRow[] = [
      { rowId: 'a', text: 'clirnb', meaningZh: '攀爬', phonetic: '', wordType: 'mistake', confidence: 0.62, issues: ['low_confidence'] },
    ]
    const wrapper = mount(DraftTable, { props: { rows } })
    expect(wrapper.text()).toContain('低置信度')
    // 该行应有 flagged 样式类
    expect(wrapper.find('tbody tr').classes()).toContain('flagged')
  })
})
