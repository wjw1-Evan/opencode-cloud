import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import './style.css'
import { api } from './api'
import { useI18n } from './i18n'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: () => import('./views/Login.vue') },
    { path: '/initialize', component: () => import('./views/Initialize.vue') },
    { path: '/portal', component: () => import('./views/Portal.vue') },
    {
      path: '/admin',
      component: () => import('./views/admin/AdminLayout.vue'),
      children: [
        { path: '', redirect: '/admin/dashboard' },
        { path: 'dashboard', component: () => import('./views/admin/Dashboard.vue') },
        { path: 'users', component: () => import('./views/admin/Users.vue') },
        { path: 'templates', component: () => import('./views/admin/Templates.vue') },
        { path: 'images', component: () => import('./views/admin/Images.vue') },
        { path: 'help', component: () => import('./views/admin/Help.vue') },
      ],
    },
  ],
})

router.beforeEach(async (to) => {
  if (to.path === '/' || to.path === '/initialize') return true
  try {
    await api.me()
    return true
  } catch {
    return { path: '/' }
  }
})

const app = createApp(App)
const i18n = useI18n()
app.provide('i18n', i18n)
app.use(router)
app.mount('#app')
