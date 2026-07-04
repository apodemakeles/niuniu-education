<script setup lang="ts">
import { onMounted } from 'vue'
import { useLibraryStore } from '@/stores/library'
import { WORD_TYPE_TEXT, WORD_STATUS_TEXT } from '@/types/word'

const store = useLibraryStore()
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

    <section class="panel">
      <p v-if="store.loading" class="note">加载中…</p>
      <p v-else-if="store.error" class="note error">加载失败：{{ store.error }}</p>

      <template v-else>
        <p class="note">
          M0 骨架联调 · 共 {{ store.words.length }} 个单词（来自后端
          <code>/api/v1/words</code>）
        </p>
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
              <tr v-for="w in store.words" :key="w.id">
                <td><strong>{{ w.text }}</strong></td>
                <td>{{ w.meaningZh }}</td>
                <td>{{ w.phonetic || '未填写' }}</td>
                <td>
                  <span class="tag" :class="w.wordType">{{ WORD_TYPE_TEXT[w.wordType] }}</span>
                </td>
                <td>
                  <span class="status-pill" :class="w.status">
                    {{ WORD_STATUS_TEXT[w.status] }}
                  </span>
                </td>
              </tr>
              <tr v-if="store.words.length === 0">
                <td colspan="5" class="note">没有单词。</td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>
    </section>
  </main>
</template>
