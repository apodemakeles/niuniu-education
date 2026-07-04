<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useLibraryStore } from '@/stores/library'
import { deleteWord } from '@/api/words'
import { WORD_TYPE_TEXT, WORD_STATUS_TEXT } from '@/types/word'
import type { Word, WordType } from '@/types/word'
import type { ImportResult } from '@/types/draft'
import PhotoImportModal from '@/components/import/PhotoImportModal.vue'
import PasteImportModal from '@/components/import/PasteImportModal.vue'
import WordEditModal from '@/components/word/WordEditModal.vue'
import WordCreateModal from '@/components/word/WordCreateModal.vue'
import ExportModal from '@/components/export/ExportModal.vue'
import Toast from '@/components/common/Toast.vue'

const store = useLibraryStore()
const toast = ref<InstanceType<typeof Toast> | null>(null)

// 弹窗状态
const showPhotoModal = ref(false)
const showPasteModal = ref(false)
const showCreateModal = ref(false)
const showExportModal = ref(false)
const editingWord = ref<Word | null>(null)

// 筛选
const activeType = ref<'all' | 'new' | 'mistake'>('all')
const statusFilter = ref('all')
const searchQuery = ref('')

const filteredWords = computed(() => {
  return store.words.filter((w) => {
    const matchType = activeType.value === 'all' || w.wordType === activeType.value
    const matchStatus = statusFilter.value === 'all' || w.status === statusFilter.value
    const q = searchQuery.value.trim().toLowerCase()
    const matchQuery =
      q === '' ||
      w.text.toLowerCase().includes(q) ||
      w.meaningZh.toLowerCase().includes(q) ||
      (w.phonetic || '').toLowerCase().includes(q)
    return matchType && matchStatus && matchQuery
  })
})

const stats = computed(() => {
  const all = store.words
  return {
    total: all.length,
    newWords: all.filter((w) => w.wordType === 'new').length,
    mistakeWords: all.filter((w) => w.wordType === 'mistake').length,
  }
})

const categories = computed(() => [
  { key: 'all' as const, title: '全部单词', desc: '查看新词和易错词', count: stats.value.total },
  { key: 'new' as const, title: '新词', desc: '计划学习的单词', count: stats.value.newWords },
  { key: 'mistake' as const, title: '易错词', desc: '需要重点复习的单词', count: stats.value.mistakeWords },
])

function reload() {
  store.load()
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
  // 对齐原型 app.js：未学直接删，其余软删
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

// 录入弹窗默认类型随当前分类联动
const createDefaultType = computed<WordType | undefined>(() =>
  activeType.value === 'new' ? 'new' : activeType.value === 'mistake' ? 'mistake' : undefined,
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
      <!-- 分类栏 -->
      <section class="panel">
        <div class="panel-head"><h2>分类</h2></div>
        <div class="library-list">
          <button
            v-for="cat in categories"
            :key="cat.key"
            class="library-card"
            :class="{ active: activeType === cat.key }"
            type="button"
            @click="activeType = cat.key"
          >
            <strong>{{ cat.title }}</strong>
            <span>{{ cat.desc }}</span>
            <span>{{ cat.count }} 个单词</span>
          </button>
        </div>
      </section>

      <!-- 详情区 -->
      <section class="panel">
        <div class="stats-grid">
          <div><span>总词数</span><strong>{{ stats.total }}</strong></div>
          <div><span>新词</span><strong>{{ stats.newWords }}</strong></div>
          <div><span>易错词</span><strong>{{ stats.mistakeWords }}</strong></div>
        </div>

        <div class="action-strip">
          <button class="primary-btn" type="button" @click="showCreateModal = true">逐个录入</button>
          <button class="secondary-btn" type="button" @click="showPasteModal = true">粘贴导入</button>
          <button class="secondary-btn" type="button" @click="showPhotoModal = true">拍照导入</button>
        </div>

        <div class="table-toolbar">
          <input v-model="searchQuery" type="search" placeholder="搜索单词、中文、音标" />
          <select v-model="statusFilter" aria-label="状态筛选">
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
                <tr v-for="w in filteredWords" :key="w.id">
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
                <tr v-if="filteredWords.length === 0">
                  <td colspan="6" class="note">没有匹配的单词。</td>
                </tr>
              </tbody>
            </table>
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
    <ExportModal v-if="showExportModal" @closed="showExportModal = false" />
    <Toast ref="toast" />
  </main>
</template>
