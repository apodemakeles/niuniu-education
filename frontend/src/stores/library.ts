import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchWords } from '@/api/words'
import type { LibraryStats, PaginationMeta } from '@/api/words'
import type { Word } from '@/types/word'

export type WordTypeFilter = 'all' | 'new' | 'mistake'

const defaultStats = (): LibraryStats => ({ total: 0, newWords: 0, mistakeWords: 0 })
const defaultPagination = (): PaginationMeta => ({ page: 1, pageSize: 20, total: 0 })

export const useLibraryStore = defineStore('library', () => {
  const words = ref<Word[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const stats = ref<LibraryStats>(defaultStats())
  const pagination = ref<PaginationMeta>(defaultPagination())
  const filters = ref({
    type: 'all' as WordTypeFilter,
    status: 'all',
    q: '',
  })

  let loadSeq = 0

  async function load(opts?: { page?: number; resetPage?: boolean }) {
    if (opts?.resetPage) {
      pagination.value.page = 1
    }
    if (opts?.page !== undefined) {
      pagination.value.page = opts.page
    }

    const seq = ++loadSeq
    loading.value = true
    error.value = null

    try {
      let page = pagination.value.page
      const pageSize = pagination.value.pageSize

      for (;;) {
        const result = await fetchWords({
          type: filters.value.type,
          status: filters.value.status,
          q: filters.value.q.trim() || undefined,
          page,
          pageSize,
        })
        if (seq !== loadSeq) return

        words.value = result.data
        pagination.value = result.pagination
        stats.value = result.stats

        const totalPages = Math.max(1, Math.ceil(result.pagination.total / pageSize))
        if (page > totalPages && result.pagination.total > 0) {
          page = totalPages
          pagination.value.page = page
          continue
        }
        break
      }
    } catch (e) {
      if (seq === loadSeq) {
        error.value = (e as Error).message
      }
    } finally {
      if (seq === loadSeq) {
        loading.value = false
      }
    }
  }

  async function setType(type: WordTypeFilter) {
    if (filters.value.type === type) return
    filters.value.type = type
    await load({ resetPage: true })
  }

  async function setStatus(status: string) {
    if (filters.value.status === status) return
    filters.value.status = status
    await load({ resetPage: true })
  }

  async function setQuery(q: string) {
    if (filters.value.q === q) return
    filters.value.q = q
    await load({ resetPage: true })
  }

  async function setPage(page: number) {
    if (pagination.value.page === page) return
    await load({ page })
  }

  return {
    words,
    loading,
    error,
    stats,
    pagination,
    filters,
    load,
    setType,
    setStatus,
    setQuery,
    setPage,
  }
})
