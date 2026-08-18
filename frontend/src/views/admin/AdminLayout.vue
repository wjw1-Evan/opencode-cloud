<template>
  <div class="layout" :class="{ 'menu-open': menuOpen }">
    <header class="topbar">
      <button class="menu-btn" :aria-label="t('navMenu')" @click="menuOpen = !menuOpen">
        <span class="menu-bar"></span>
        <span class="menu-bar"></span>
        <span class="menu-bar"></span>
      </button>
      <span class="topbar-brand">◈ DevCapsule</span>
    </header>
    <div v-if="menuOpen" class="side-mask" @click="menuOpen = false"></div>
    <aside class="side" :class="{ open: menuOpen }">
      <div class="brand">
        <span class="logo">◈</span>
        <span class="brand-name">DevCapsule</span>
        <button class="side-close" :aria-label="t('close')" @click="menuOpen = false">×</button>
      </div>
      <nav>
        <RouterLink to="/admin/dashboard" @click="menuOpen = false"><span class="dot"></span>{{ t('navDashboard') }}</RouterLink>
        <RouterLink to="/admin/users" @click="menuOpen = false"><span class="dot"></span>{{ t('navUsers') }}</RouterLink>
        <RouterLink to="/admin/templates" @click="menuOpen = false"><span class="dot"></span>{{ t('navTemplates') }}</RouterLink>
        <RouterLink to="/admin/images" @click="menuOpen = false"><span class="dot"></span>{{ t('navImages') }}</RouterLink>
        <RouterLink to="/admin/help" @click="menuOpen = false"><span class="dot"></span>{{ t('navHelp') }}</RouterLink>
      </nav>
      <div class="side-foot">
        <button class="btn pwd-btn" @click="showPwd = true; menuOpen = false">{{ t('changePwd') }}</button>
        <button class="btn logout" @click="askLogout(); menuOpen = false">{{ t('logout') }}</button>
      </div>
    </aside>
    <main class="main">
      <router-view />
    </main>

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
import { ref, inject, watch, onUnmounted } from 'vue'
import { api } from '../../api'
import ConfirmDialog from '../../components/ConfirmDialog.vue'

const { t } = inject('i18n')

const menuOpen = ref(false)
const showPwd = ref(false)
const showLogoutConfirm = ref(false)
const oldPwd = ref('')
const newPwd = ref('')
const confirmPwd = ref('')
const pwdBusy = ref(false)
const pwdError = ref('')

// Lock body scroll while the mobile drawer is open.
watch(menuOpen, (v) => {
  document.body.style.overflow = v ? 'hidden' : ''
})
onUnmounted(() => {
  document.body.style.overflow = ''
})

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

function askLogout() {
  showLogoutConfirm.value = true
}

function doLogout() {
  window.location.href = '/platform/auth/logout'
}
</script>

<style scoped>
.layout { display: flex; min-height: 100vh; }
.side {
  width: 216px;
  background: rgba(10, 15, 30, 0.65);
  backdrop-filter: blur(16px) saturate(140%);
  -webkit-backdrop-filter: blur(16px) saturate(140%);
  border-right: 1px solid var(--glass-border);
  display: flex;
  flex-direction: column;
  padding: 22px 14px 18px;
  position: sticky;
  top: 0;
  height: 100vh;
}
.menu-btn {
  display: none;
  width: 40px;
  height: 40px;
  border-radius: 12px;
  background: rgba(10, 15, 30, 0.85);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid var(--glass-border);
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 5px;
  cursor: pointer;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}
.menu-btn:hover {
  border-color: rgba(34, 211, 238, 0.45);
  box-shadow: 0 0 14px rgba(34, 211, 238, 0.2);
}
.menu-bar {
  width: 18px;
  height: 2px;
  border-radius: 2px;
  background: var(--text-0);
  transition: background 0.2s ease;
}
.menu-btn:hover .menu-bar { background: var(--cyan); }
.side-mask { display: none; }
.topbar {
  display: none;
  align-items: center;
  gap: 12px;
  padding: 0 16px;
  height: 56px;
  background: rgba(6, 9, 18, 0.82);
  backdrop-filter: blur(14px) saturate(150%);
  -webkit-backdrop-filter: blur(14px) saturate(150%);
  border-bottom: 1px solid var(--glass-border);
}
.topbar-brand {
  font-size: 15px;
  font-weight: 700;
  letter-spacing: 0.03em;
  background: var(--grad);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}
.side-close { display: none; }
.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 8px 24px;
}
.logo {
  width: 34px;
  height: 34px;
  border-radius: 11px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  color: #04101a;
  background: var(--grad);
  box-shadow: 0 4px 18px rgba(34, 211, 238, 0.4);
  animation: logoGlow 3.2s ease-in-out infinite;
}
@keyframes logoGlow {
  0%, 100% { box-shadow: 0 4px 18px rgba(34, 211, 238, 0.4); }
  50% { box-shadow: 0 4px 28px rgba(139, 92, 246, 0.65); }
}
.brand-name {
  font-size: 15px;
  font-weight: 700;
  letter-spacing: 0.02em;
  background: var(--grad);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}
nav { display: flex; flex-direction: column; gap: 4px; flex: 1; }
nav a {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--text-1);
  padding: 11px 14px;
  border-radius: 12px;
  font-size: 14px;
  position: relative;
  transition: all 0.22s ease;
}
nav a .dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--text-2);
  transition: all 0.22s ease;
}
nav a:hover { background: rgba(255, 255, 255, 0.06); color: var(--text-0); }
nav a:hover .dot { box-shadow: 0 0 8px currentColor; }
nav a.router-link-active {
  background: linear-gradient(135deg, rgba(34, 211, 238, 0.16), rgba(139, 92, 246, 0.16));
  color: var(--text-0);
  border: 1px solid rgba(34, 211, 238, 0.25);
}
nav a.router-link-active::before {
  content: "";
  position: absolute;
  left: 0; top: 50%;
  transform: translateY(-50%);
  width: 3px; height: 60%;
  border-radius: 3px;
  background: var(--grad);
  box-shadow: 0 0 12px rgba(34, 211, 238, 0.8);
}
nav a.router-link-active .dot {
  background: var(--cyan);
  box-shadow: 0 0 10px var(--cyan);
}
.side-foot {
  padding: 12px 8px 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  border-top: 1px solid rgba(255, 255, 255, 0.07);
}
.pwd-btn {
  width: 100%;
  color: var(--text-2);
  border-color: var(--glass-border);
  background: transparent;
}
.pwd-btn:hover { color: var(--cyan); border-color: rgba(34, 211, 238, 0.4); }
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
.logout {
  width: 100%;
  color: var(--text-2);
  border-color: var(--glass-border);
  background: transparent;
}
.logout:hover { color: var(--err); border-color: rgba(248, 113, 113, 0.5); box-shadow: 0 0 14px rgba(248, 113, 113, 0.2); }
.main {
  flex: 1;
  padding: 28px 32px;
  max-width: 1200px;
}

@media (max-width: 900px) {
  .topbar {
    display: flex;
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    z-index: 120;
  }
  .menu-btn { display: flex; }
  .side {
    position: fixed;
    left: 0;
    top: 0;
    height: 100vh;
    transform: translateX(-100%);
    transition: transform 0.28s cubic-bezier(0.16, 1, 0.3, 1);
    z-index: 150;
    padding-top: 24px;
  }
  .side.open {
    transform: translateX(0);
    box-shadow: 0 0 60px rgba(0, 0, 0, 0.65);
  }
  .side-close {
    display: flex;
    margin-left: auto;
    width: 30px;
    height: 30px;
    align-items: center;
    justify-content: center;
    border-radius: 9px;
    border: 1px solid var(--glass-border);
    background: transparent;
    color: var(--text-2);
    font-size: 18px;
    line-height: 1;
    cursor: pointer;
    transition: color 0.2s ease, border-color 0.2s ease;
  }
  .side-close:hover {
    color: var(--err);
    border-color: rgba(248, 113, 113, 0.45);
  }
  .side-mask {
    display: block;
    position: fixed;
    inset: 0;
    background: rgba(2, 4, 10, 0.55);
    backdrop-filter: blur(3px);
    -webkit-backdrop-filter: blur(3px);
    z-index: 140;
  }
  .main {
    padding: 76px 18px 40px;
    max-width: 100%;
  }
  .layout.menu-open { height: 100vh; overflow: hidden; }
}
</style>
