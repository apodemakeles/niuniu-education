<script setup lang="ts">
import { ref, onUnmounted } from 'vue'

const visible = ref(false)
const message = ref('')
let timer: ReturnType<typeof setTimeout> | null = null

function show(msg: string, duration = 3000) {
  message.value = msg
  visible.value = true
  if (timer) clearTimeout(timer)
  timer = setTimeout(() => (visible.value = false), duration)
}

defineExpose({ show })
onUnmounted(() => {
  if (timer) clearTimeout(timer)
})
</script>

<template>
  <Transition name="toast">
    <div v-if="visible" class="toast" role="status" aria-live="polite">
      {{ message }}
    </div>
  </Transition>
</template>
