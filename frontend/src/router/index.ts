import { createRouter, createWebHistory } from 'vue-router'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/library' },
    { path: '/library', name: 'library', component: () => import('@/views/LibraryView.vue') },
    { path: '/practice', name: 'practice', component: () => import('@/views/PracticeView.vue') },
  ],
})
