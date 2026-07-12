<script setup lang="ts">
import { computed } from 'vue'
import { RouterView, useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()

// 是否处于学生端（学生端不显示家长端切换浮层，保持卡通纯净）
const isKid = computed(() => route.path.startsWith('/practice'))

function go(target: 'library' | 'practice') {
  router.push(target === 'library' ? '/library' : '/practice')
}
</script>

<template>
  <RouterView />
  <!-- 右下角角色切换浮层：家长端 → 学生端。学生端不显示，保持卡通纯净。 -->
  <nav v-if="!isKid" class="role-switch" aria-label="切换角色">
    <button type="button" class="role-btn" @click="go('practice')">孩子的背单词任务 →</button>
  </nav>
</template>
