<template>
  <div class="portal-wrap">
    <div class="nebula n1"></div>
    <div class="nebula n2"></div>

    <div class="card portal-card">
      <div class="top">
        <h1>我的开发环境</h1>
        <button class="btn" @click="logout">退出登录</button>
      </div>

      <p class="hello">你好，<b class="uname">{{ user.username }}</b></p>

      <div v-if="container" class="status-panel">
        <div class="status-row">
          <span class="st-label">状态</span>
          <span class="badge" :class="container.status">{{ container.status }}</span>
        </div>
        <template v-if="container.status === 'running'">
          <div class="status-row">
            <span class="st-label">端口</span>
            <code>{{ container.internal_port }}{{ (container.extra_ports || []).length ? ' / ' + (container.extra_ports || []).join(',') : '' }}</code>
          </div>
          <div class="status-row">
            <span class="st-label">容器</span>
            <code>{{ container.container_name }}</code>
          </div>
        </template>
      </div>
      <p v-else class="meta">尚未分配容器。</p>

      <div class="actions">
        <a class="btn btn-primary open" :href="entry" target="_blank" rel="noopener">
          <span class="dot" :class="{ on: running }"></span>
          打开开发环境 →
        </a>
        <button class="btn" :disabled="!running" @click="refresh">
          刷新状态
        </button>
        <button class="btn" @click="showPwd = !showPwd">{{ showPwd ? '收起' : '修改密码' }}</button>
      </div>

      <form v-if="showPwd" class="pwd-form" @submit.prevent="changePwd">
        <input v-model="oldPwd" type="password" placeholder="当前密码" required />
        <input v-model="newPwd" type="password" placeholder="新密码（至少 8 位）" required minlength="8" />
        <input v-model="confirmPwd" type="password" placeholder="确认新密码" required />
        <button class="btn btn-primary" type="submit" :disabled="pwdBusy">{{ pwdBusy ? '提交中…' : '确认修改' }}</button>
      </form>

      <p class="hint">环境空闲 30 分钟会自动关闭，点击"打开开发环境"会自动唤醒。</p>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { api } from '../api'

const user = ref({})
const container = ref(null)
const showPwd = ref(false)
const oldPwd = ref('')
const newPwd = ref('')
const confirmPwd = ref('')
const pwdBusy = ref(false)

const running = computed(() => container.value && container.value.status === 'running')
const entry = computed(() => `/`)

async function changePwd() {
  if (newPwd.value !== confirmPwd.value) {
    alert('两次输入的新密码不一致')
    return
  }
  pwdBusy.value = true
  try {
    await api.changePassword(oldPwd.value, newPwd.value)
    alert('密码修改成功')
    showPwd.value = false
    oldPwd.value = newPwd.value = confirmPwd.value = ''
  } catch (e) {
    alert(e.message)
  } finally {
    pwdBusy.value = false
  }
}

onMounted(async () => {
  await load()
  await refresh()
})

async function load() {
  try {
    const me = await api.me()
    user.value = me.user || me
  } catch { /* router guard redirects */ }
}

async function refresh() {
  try {
    const me = await api.me()
    user.value = me.user || me
    container.value = me.container || null
  } catch {}
}

async function logout() {
  window.location.href = '/platform/auth/logout'
}
</script>

<style scoped>
.portal-wrap {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  overflow: hidden;
  background:
    radial-gradient(1000px 500px at 15% 25%, rgba(34, 211, 238, 0.13), transparent 60%),
    radial-gradient(900px 600px at 85% 80%, rgba(139, 92, 246, 0.18), transparent 60%),
    #04060d;
}
.nebula {
  position: absolute;
  border-radius: 50%;
  filter: blur(90px);
  opacity: 0.45;
  animation: drift 16s ease-in-out infinite alternate;
}
.n1 { width: 380px; height: 380px; background: rgba(34, 211, 238, 0.35); top: -80px; right: -60px; }
.n2 { width: 460px; height: 460px; background: rgba(139, 92, 246, 0.35); bottom: -140px; left: -100px; animation-delay: -8s; }
@keyframes drift {
  from { transform: translate(0, 0) scale(1); }
  to { transform: translate(-50px, 30px) scale(1.12); }
}
.portal-card {
  width: 520px;
  padding: 36px;
  position: relative;
  z-index: 2;
  border: 1px solid rgba(255, 255, 255, 0.14);
  box-shadow: 0 24px 80px rgba(0, 0, 0, 0.55), inset 0 1px 0 rgba(255, 255, 255, 0.1);
}
.top { display: flex; justify-content: space-between; align-items: center; }
h1 {
  margin: 0;
  font-size: 22px;
  background: var(--grad);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}
.hello { margin: 22px 0 16px; font-size: 16px; color: var(--text-1); }
.uname {
  background: var(--grad);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}
.status-panel {
  background: rgba(0, 0, 0, 0.25);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-md);
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.status-row { display: flex; align-items: center; gap: 14px; }
.st-label {
  width: 52px;
  color: var(--text-2);
  font-size: 12px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
.meta { color: var(--text-2); font-size: 13px; margin: 0; }
.actions { display: flex; gap: 12px; align-items: center; margin: 18px 0; }
.open {
  font-size: 15px;
  padding: 13px 24px;
  text-decoration: none;
  position: relative;
  overflow: hidden;
}
.dot {
  display: inline-block;
  width: 8px; height: 8px;
  border-radius: 50%;
  background: var(--text-2);
  margin-right: 9px;
  vertical-align: 1px;
}
.dot.on {
  background: var(--ok);
  box-shadow: 0 0 12px var(--ok);
  animation: pulse 1.8s ease-in-out infinite;
}
@keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.4; } }
.hint { color: var(--text-2); font-size: 12px; margin-top: 22px; }
.pwd-form { display: flex; flex-direction: column; gap: 10px; margin-top: 4px; }
</style>
