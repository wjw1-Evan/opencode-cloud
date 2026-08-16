<template>
  <router-view />
  <div v-if="toast" :class="['toast', toast.type]">{{ toast.msg }}</div>
</template>

<script setup>
import { ref, provide } from 'vue'

const toast = ref(null)
function notify(msg, type = 'ok') {
  toast.value = { msg, type }
  setTimeout(() => (toast.value = null), 3000)
}
provide('notify', notify)
</script>
