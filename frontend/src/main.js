import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import './style.css'
import { api } from './api'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: () => import('./views/Login.vue') },
    { path: '/portal', component: () => import('./views/Portal.vue') },
    {
      path: '/admin',
      component: () => import('./views/admin/AdminLayout.vue'),
      children: [
        { path: '', redirect: '/admin/dashboard' },
        { path: 'dashboard', component: () => import('./views/admin/Dashboard.vue') },
        { path: 'users', component: () => import('./views/admin/Users.vue') },
        { path: 'templates', component: () => import('./views/admin/Templates.vue') },
        { path: 'help', component: () => import('./views/admin/Help.vue') },
      ],
    },
  ],
})

router.beforeEach(async (to) => {
  if (to.path === '/') return true
  try {
    await api.me()
    return true
  } catch {
    return { path: '/' }
  }
})

const app = createApp(App)
app.use(router)
app.mount('#app')
