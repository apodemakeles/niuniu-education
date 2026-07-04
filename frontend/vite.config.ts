import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

// 开发期：vite dev (5173) 通过 proxy 把 /api 转发到后端 (8787)，浏览器同源无需 CORS。
// 部署期：构建产物由独立静态服务托管，VITE_API_BASE 指向后端绝对地址。
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8787',
        changeOrigin: true,
      },
    },
  },
})
