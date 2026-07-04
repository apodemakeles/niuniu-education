import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchWords } from '@/api/words'
import type { Word } from '@/types/word'

export const useLibraryStore = defineStore('library', () => {
  const words = ref<Word[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function load() {
    loading.value = true
    error.value = null
    try {
      words.value = await fetchWords()
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  return { words, loading, error, load }
})
