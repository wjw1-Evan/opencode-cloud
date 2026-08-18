<template>
  <div class="portal-wrap">
    <div class="nebula n1"></div>
    <div class="nebula n2"></div>
    <div class="grid-overlay"></div>

    <div class="card portal-card">
      <div class="top">
        <h1>{{ t('myEnv') }}</h1>
        <button class="btn" @click="askLogout">{{ t('logout') }}</button>
      </div>

      <p class="hello">{{ t('hello') }}<b class="uname">{{ user.username }}</b></p>

      <div v-if="container" class="status-panel">
        <div class="status-row">
          <span class="st-label">{{ t('status') }}</span>
          <span class="badge" :class="container.status">{{ container.status }}</span>
        </div>
        <template v-if="container.status === 'running'">
          <div class="status-row">
            <span class="st-label">{{ t('port') }}</span>
            <code>{{ container.internal_port }}{{ (container.extra_ports || []).length ? ' / ' + (container.extra_ports || []).join(',') : '' }}</code>
          </div>
          <div class="status-row">
            <span class="st-label">{{ t('container') }}</span>
            <code>{{ container.container_name }}</code>
          </div>
        </template>
      </div>
      <p v-else class="meta">{{ t('noContainer') }}</p>

      <div class="actions">
        <a class="btn btn-primary open" :class="{ dim: !container }" :href="container ? entry : undefined" target="_blank" rel="noopener" @click="openEnv">
          <span class="dot" :class="{ on: running }"></span>
          {{ t('openEnv') }}
        </a>
        <button class="btn" :disabled="!running" @click="refresh">
          {{ t('refreshStatus') }}
        </button>
        <button class="btn" @click="showPwd = true">{{ t('changePwd') }}</button>
      </div>

      <div v-if="showPwd" class="modal-mask" @click.self="showPwd = false">
        <div class="modal">
          <h3>{{ t('changePwd') }}</h3>
          <div class="pwd-fields">
            <input v-model="oldPwd" type="password" :placeholder="t('currentPwd')" required />
            <input v-model="newPwd" type="password" :placeholder="t('newPwd')" required minlength="8" />
            <input v-model="confirmPwd" type="password" :placeholder="t('confirmNewPwd')" required />
            <p v-if="pwdError" class="pwd-error">{{ pwdError }}</p>
          </div>
          <div class="btns">
            <button class="btn btn-primary" @click="changePwd" :disabled="pwdBusy">{{ pwdBusy ? t('submitting') : t('confirm') }}</button>
            <button class="btn" @click="showPwd = false" :disabled="pwdBusy">{{ t('cancel') }}</button>
          </div>
        </div>
      </div>

      <p class="hint">{{ t('idleHint') }}</p>
    </div>

    <ConfirmDialog
      :visible="showLogoutConfirm"
      :message="t('logoutConfirm')"
      type="danger"
      @confirm="doLogout"
      @cancel="showLogoutConfirm = false"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, inject } from 'vue'
import { api } from '../api'
import ConfirmDialog from '../components/ConfirmDialog.vue'

const { t } = inject('i18n')
const notify = inject('notify')

const user = ref({})
const container = ref(null)
const showPwd = ref(false)
const showLogoutConfirm = ref(false)
const oldPwd = ref('')
const newPwd = ref('')
const confirmPwd = ref('')
const pwdBusy = ref(false)
const pwdError = ref('')

const running = computed(() => container.value && container.value.status === 'running')
const entry = computed(() => `/`)

function openEnv(e) {
  if (!container.value) {
    e.preventDefault()
    notify(t('noContainer'), 'err')
  }
}

async function changePwd() {
  pwdError.value = ''
  if (newPwd.value !== confirmPwd.value) {
    pwdError.value = t('pwdMismatch')
    return
  }
  pwdBusy.value = true
  try {
    await api.changePassword(oldPwd.value, newPwd.value)
    showPwd.value = false
    oldPwd.value = newPwd.value = confirmPwd.value = ''
  } catch (e) {
    pwdError.value = e.message
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

function askLogout() {
  showLogoutConfirm.value = true
}

function doLogout() {
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
  filter: blur(60px);
  opacity: 0.45;
  animation: drift 16s ease-in-out infinite alternate;
}
.n1 { width: 380px; height: 380px; background: rgba(34, 211, 238, 0.35); top: -80px; right: -60px; }
.n2 { width: 460px; height: 460px; background: rgba(139, 92, 246, 0.35); bottom: -140px; left: -100px; animation-delay: -8s; }
.grid-overlay {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(34, 211, 238, 0.045) 1px, transparent 1px),
    linear-gradient(90deg, rgba(34, 211, 238, 0.045) 1px, transparent 1px);
  background-size: 44px 44px;
  mask-image: radial-gradient(ellipse at center, black 25%, transparent 72%);
  -webkit-mask-image: radial-gradient(ellipse at center, black 25%, transparent 72%);
}
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
.open.dim { opacity: 0.45; cursor: not-allowed; }
.open::after {
  content: "";
  position: absolute;
  top: 0; left: -60%;
  width: 45%; height: 100%;
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.28), transparent);
  transform: skewX(-20deg);
  animation: sweep 3s ease-in-out infinite;
}
@keyframes sweep {
  0%, 55% { left: -60%; }
  100% { left: 140%; }
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
.pwd-fields { display: flex; flex-direction: column; gap: 12px; }
.pwd-fields input {
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid var(--glass-border);
  background: rgba(0, 0, 0, 0.3);
  color: var(--text-0);
  font-size: 13px;
}
.pwd-fields input::placeholder { color: var(--text-2); }
.pwd-error { color: var(--err); font-size: 12px; margin: 0; }

@media (max-width: 720px) {
  .portal-card {
    width: calc(100vw - 28px);
    max-width: 520px;
    padding: 28px 20px;
  }
  .actions { flex-wrap: wrap; }
  .actions .btn { flex: 1 1 auto; text-align: center; }
  .top { flex-wrap: wrap; gap: 10px; }
}
</style>
