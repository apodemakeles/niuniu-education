import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

// 开发期：vite dev (5173) 通过 proxy 把 /api 转发到后端 (8787)，浏览器同源无需 CORS。
// 部署期：构建产物由独立静态服务托管，前端用相对路径 /api/v1 调后端，经 nginx 反代同源。
//
// base 支持子路径部署：默认根路径 /，构建时用环境变量 VITE_BASE_PATH 覆盖。
// 例如部署到 /ed：VITE_BASE_PATH=/ed/ npm run build；产物引用 /ed/assets/...
// router/index.ts 的 createWebHistory(import.meta.env.BASE_URL) 会自动跟随。
export default defineConfig({
  base: process.env.VITE_BASE_PATH || '/',
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
