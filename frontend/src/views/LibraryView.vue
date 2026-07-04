<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useLibraryStore } from '@/stores/library'
import { WORD_TYPE_TEXT, WORD_STATUS_TEXT } from '@/types/word'
import type { ImportResult } from '@/types/draft'
import PhotoImportModal from '@/components/import/PhotoImportModal.vue'
import Toast from '@/components/common/Toast.vue'

const store = useLibraryStore()
const showPhotoModal = ref(false)
const toast = ref<InstanceType<typeof Toast> | null>(null)

const activeType = ref<'all' | 'new' | 'mistake'>('all')
const statusFilter = ref('all')

const filteredWords = computed(() => {
  return store.words.filter((w) => {
    const matchType = activeType.value === 'all' || w.wordType === activeType.value
    const matchStatus = statusFilter.value === 'all' || w.status === statusFilter.value
    return matchType && matchStatus
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

function onImportConfirmed(result: ImportResult) {
  showPhotoModal.value = false
  const parts: string[] = []
  if (result.added) parts.push(`新增 ${result.added}`)
  if (result.skipped) parts.push(`跳过重复 ${result.skipped}`)
  if (result.invalid) parts.push(`失败 ${result.invalid}`)
  toast.value?.show(parts.length ? `已入库：${parts.join('，')}` : '没有变化')
  reload()
}

onMounted(() => store.load())
</script>

<template>
  <main class="main">
    <header class="topbar">
      <div>
        <p class="eyebrow">Parent Word Library</p>
        <h1>单词库管理</h1>
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
          <button class="primary-btn" type="button" @click="showPhotoModal = true">
            拍照导入
          </button>
        </div>

        <div class="table-toolbar">
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
                </tr>
              </thead>
              <tbody>
                <tr v-for="w in filteredWords" :key="w.id">
                  <td><strong>{{ w.text }}</strong></td>
                  <td>{{ w.meaningZh }}</td>
                  <td>{{ w.phonetic || '未填写' }}</td>
                  <td>
                    <span class="tag" :class="w.wordType">{{ WORD_TYPE_TEXT[w.wordType] }}</span>
                  </td>
                  <td>
                    <span class="status-pill" :class="w.status">{{ WORD_STATUS_TEXT[w.status] }}</span>
                  </td>
                </tr>
                <tr v-if="filteredWords.length === 0">
                  <td colspan="5" class="note">没有匹配的单词。</td>
                </tr>
              </tbody>
            </table>
          </div>
        </template>
      </section>
    </div>

    <PhotoImportModal
      v-if="showPhotoModal"
      @confirmed="onImportConfirmed"
      @closed="showPhotoModal = false"
    />
    <Toast ref="toast" />
  </main>
</template>
