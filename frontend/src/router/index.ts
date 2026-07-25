import { createRouter, createWebHistory } from 'vue-router'

// history base 跟随 vite 的 base（import.meta.env.BASE_URL），支持子路径部署。
// base=/ 时路由为 /、/library、/practice；base=/ed/ 时路由对应 /ed、/ed/library、/ed/practice。
export const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', redirect: '/library' },
    { path: '/library', name: 'library', component: () => import('@/views/LibraryView.vue') },
    { path: '/practice', name: 'practice', component: () => import('@/views/PracticeView.vue') },
  ],
})
