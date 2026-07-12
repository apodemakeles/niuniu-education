<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useLibraryStore } from '@/stores/library'
import { deleteWord } from '@/api/words'
import { WORD_TYPE_TEXT, WORD_STATUS_TEXT } from '@/types/word'
import type { Word, WordType } from '@/types/word'
import type { ImportResult } from '@/types/draft'
import PhotoImportModal from '@/components/import/PhotoImportModal.vue'
import PasteImportModal from '@/components/import/PasteImportModal.vue'
import WordEditModal from '@/components/word/WordEditModal.vue'
import WordCreateModal from '@/components/word/WordCreateModal.vue'
import ExampleBackfillModal from '@/components/word/ExampleBackfillModal.vue'
import ExportModal from '@/components/export/ExportModal.vue'
import Toast from '@/components/common/Toast.vue'

const store = useLibraryStore()
const toast = ref<InstanceType<typeof Toast> | null>(null)

// 弹窗状态
const showPhotoModal = ref(false)
const showPasteModal = ref(false)
const showCreateModal = ref(false)
const showExportModal = ref(false)
const showExampleBackfill = ref(false)
const editingWord = ref<Word | null>(null)

// 搜索框本地值（防抖后提交到 store；跳过与 store 相同的值，避免挂载时重复请求）
const searchInput = ref(store.filters.q)
let searchTimer: ReturnType<typeof setTimeout> | null = null
watch(searchInput, (q) => {
  if (q === store.filters.q) return
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    store.setQuery(q)
  }, 300)
})

const categories = computed(() => [
  { key: 'all' as const, title: '全部单词', desc: '查看新词和易错词', count: store.stats.total },
  { key: 'new' as const, title: '新词', desc: '计划学习的单词', count: store.stats.newWords },
  { key: 'mistake' as const, title: '易错词', desc: '需要重点复习的单词', count: store.stats.mistakeWords },
])

const totalPages = computed(() =>
  Math.max(1, Math.ceil(store.pagination.total / store.pagination.pageSize)),
)

const pageRangeText = computed(() => {
  const { total, page, pageSize } = store.pagination
  if (total === 0) return '0 条'
  const start = (page - 1) * pageSize + 1
  const end = Math.min(page * pageSize, total)
  return `${start}–${end} / 共 ${total} 条`
})

function reload() {
  return store.load()
}

function onTypeChange(type: 'all' | 'new' | 'mistake') {
  store.setType(type)
}

function onStatusChange(e: Event) {
  store.setStatus((e.target as HTMLSelectElement).value)
}

function onImportConfirmed(result: ImportResult, source: 'photo' | 'paste') {
  if (source === 'photo') showPhotoModal.value = false
  else showPasteModal.value = false
  const parts: string[] = []
  if (result.added) parts.push(`新增 ${result.added}`)
  if (result.skipped) parts.push(`跳过重复 ${result.skipped}`)
  if (result.invalid) parts.push(`失败 ${result.invalid}`)
  toast.value?.show(parts.length ? `已入库：${parts.join('，')}` : '没有变化')
  reload()
}

function onWordCreated() {
  toast.value?.show('已保存，继续录入下一个。')
  reload()
}

function onWordSaved() {
  editingWord.value = null
  toast.value?.show('单词已更新，孩子学习进度未重置。')
  reload()
}

async function onDelete(w: Word) {
  const isUnlearned = w.status === 'unlearned'
  const confirmMsg = isUnlearned
    ? `确认删除「${w.text}」？未学单词将直接删除。`
    : `确认移除「${w.text}」？将保留学习记录，可后续恢复。`
  if (!window.confirm(confirmMsg)) return

  try {
    const res = await deleteWord(w.id)
    toast.value?.show(
      res.kind === 'physical' ? '未学单词已直接删除。' : '已从单词库移除，学习记录保留。',
    )
    reload()
  } catch (e) {
    toast.value?.show('删除失败：' + (e as Error).message)
  }
}

const createDefaultType = computed<WordType | undefined>(() =>
  store.filters.type === 'new' ? 'new' : store.filters.type === 'mistake' ? 'mistake' : undefined,
)

onMounted(() => store.load())
</script>

<template>
  <main class="main">
    <header class="topbar">
      <div>
        <p class="eyebrow">Parent Word Library</p>
        <h1>单词库管理</h1>
      </div>
      <div class="top-actions">
        <button class="secondary-btn" type="button" @click="showExportModal = true">导出</button>
      </div>
    </header>

    <div class="library-layout">
      <section class="panel">
        <div class="panel-head"><h2>分类</h2></div>
        <div class="library-list">
          <button
            v-for="cat in categories"
            :key="cat.key"
            class="library-card"
            :class="{ active: store.filters.type === cat.key }"
            type="button"
            @click="onTypeChange(cat.key)"
          >
            <strong>{{ cat.title }}</strong>
            <span>{{ cat.desc }}</span>
            <span>{{ cat.count }} 个单词</span>
          </button>
        </div>
      </section>

      <section class="panel">
        <div class="stats-grid">
          <div><span>总词数</span><strong>{{ store.stats.total }}</strong></div>
          <div><span>新词</span><strong>{{ store.stats.newWords }}</strong></div>
          <div><span>易错词</span><strong>{{ store.stats.mistakeWords }}</strong></div>
        </div>

        <div class="action-strip">
          <button class="primary-btn" type="button" @click="showCreateModal = true">逐个录入</button>
          <button class="secondary-btn" type="button" @click="showPasteModal = true">粘贴导入</button>
          <button class="secondary-btn" type="button" @click="showPhotoModal = true">拍照导入</button>
          <button class="secondary-btn" type="button" @click="showExampleBackfill = true">补全缺失例句</button>
        </div>

        <div class="table-toolbar">
          <input
            v-model="searchInput"
            type="search"
            placeholder="搜索单词、中文、音标"
          />
          <select
            :value="store.filters.status"
            aria-label="状态筛选"
            @change="onStatusChange"
          >
            <option value="all">全部状态</option>
            <option value="unlearned">未学</option>
            <option value="learning">学习中</option>
            <option value="reinforce">需强化</option>
            <option value="mastered">已掌握</option>
          </select>
        </div>

        <p v-if="store.loading" class="note">加载中…</p>
        <p v-else-if="store.error" class="note error">加载失败：{{ store.error }}</p>
        <template v-else>
          <div class="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>英文单词</th>
                  <th>中文</th>
                  <th>音标</th>
                  <th>类型</th>
                  <th>状态</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="w in store.words" :key="w.id">
                  <td><strong>{{ w.text }}</strong></td>
                  <td>{{ w.meaningZh }}</td>
                  <td>{{ w.phonetic || '未填写' }}</td>
                  <td><span class="tag" :class="w.wordType">{{ WORD_TYPE_TEXT[w.wordType] }}</span></td>
                  <td><span class="status-pill" :class="w.status">{{ WORD_STATUS_TEXT[w.status] }}</span></td>
                  <td>
                    <div class="row-actions">
                      <button class="link-btn" type="button" @click="editingWord = w">编辑</button>
                      <button class="link-btn" type="button" @click="onDelete(w)">删除</button>
                    </div>
                  </td>
                </tr>
                <tr v-if="store.words.length === 0">
                  <td colspan="6" class="note">没有匹配的单词。</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div v-if="store.pagination.total > 0" class="pagination-bar">
            <span class="pagination-info">{{ pageRangeText }}</span>
            <div class="pagination-actions">
              <button
                class="secondary-btn"
                type="button"
                :disabled="store.pagination.page <= 1 || store.loading"
                @click="store.setPage(store.pagination.page - 1)"
              >
                上一页
              </button>
              <span class="pagination-page">第 {{ store.pagination.page }} / {{ totalPages }} 页</span>
              <button
                class="secondary-btn"
                type="button"
                :disabled="store.pagination.page >= totalPages || store.loading"
                @click="store.setPage(store.pagination.page + 1)"
              >
                下一页
              </button>
            </div>
          </div>
        </template>
      </section>
    </div>

    <PhotoImportModal
      v-if="showPhotoModal"
      @confirmed="(r) => onImportConfirmed(r, 'photo')"
      @closed="showPhotoModal = false"
    />
    <PasteImportModal
      v-if="showPasteModal"
      @confirmed="(r) => onImportConfirmed(r, 'paste')"
      @closed="showPasteModal = false"
    />
    <WordCreateModal
      v-if="showCreateModal"
      :default-type="createDefaultType"
      @created="onWordCreated"
      @closed="showCreateModal = false"
    />
    <WordEditModal
      v-if="editingWord"
      :word="editingWord"
      @saved="onWordSaved"
      @closed="editingWord = null"
    />
    <ExampleBackfillModal
      v-if="showExampleBackfill"
      @finished="toast?.show('例句补全完成，可在编辑页查看或单句重试。')"
      @closed="showExampleBackfill = false"
    />
    <ExportModal v-if="showExportModal" @closed="showExportModal = false" />
    <Toast ref="toast" />
  </main>
</template>
