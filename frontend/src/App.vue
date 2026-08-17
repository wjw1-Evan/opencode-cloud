<template>
  <router-view />
  <div class="lang-switcher">
    <button :class="['lang-btn', { active: locale === 'zh' }]" @click="setLocale('zh')">中文</button>
    <button :class="['lang-btn', { active: locale === 'en' }]" @click="setLocale('en')">EN</button>
  </div>
  <div v-if="toast" :class="['toast', toast.type]">{{ toast.msg }}</div>
</template>

<script setup>
import { ref, provide, inject } from 'vue'

const { locale, setLocale } = inject('i18n')

const toast = ref(null)
function notify(msg, type = 'ok') {
  toast.value = { msg, type }
  setTimeout(() => (toast.value = null), 3000)
}
provide('notify', notify)
</script>

<style scoped>
.lang-switcher {
  position: fixed;
  bottom: 20px;
  right: 20px;
  display: flex;
  gap: 2px;
  background: rgba(14, 20, 38, 0.85);
  backdrop-filter: blur(12px);
  border-radius: 8px;
  border: 1px solid var(--glass-border);
  padding: 3px;
  z-index: 200;
}
.lang-btn {
  padding: 5px 12px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--text-2);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}
.lang-btn:hover {
  color: var(--text-0);
}
.lang-btn.active {
  background: linear-gradient(135deg, rgba(34, 211, 238, 0.2), rgba(139, 92, 246, 0.2));
  color: var(--text-0);
}
</style>
