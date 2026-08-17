<template>
  <div class="layout">
    <aside class="side">
      <div class="brand">
        <span class="logo">◈</span>
        <span class="brand-name">DevCapsule</span>
      </div>
      <nav>
        <RouterLink to="/admin/dashboard"><span class="dot"></span>总览</RouterLink>
        <RouterLink to="/admin/users"><span class="dot"></span>用户与容器</RouterLink>
        <RouterLink to="/admin/templates"><span class="dot"></span>镜像模板</RouterLink>
        <RouterLink to="/admin/images"><span class="dot"></span>镜像管理</RouterLink>
        <RouterLink to="/admin/help"><span class="dot"></span>使用帮助</RouterLink>
      </nav>
      <div class="side-foot">
        <button class="btn pwd-btn" @click="showPwd = true">修改密码</button>
        <button class="btn logout" @click="logout">退出登录</button>
      </div>
    </aside>
    <main class="main">
      <router-view />
    </main>

    <div v-if="showPwd" class="modal-mask" @click.self="showPwd = false">
      <div class="modal">
        <h3>修改密码</h3>
        <div class="pwd-fields">
          <input v-model="oldPwd" type="password" placeholder="当前密码" required />
          <input v-model="newPwd" type="password" placeholder="新密码（至少 8 位）" required minlength="8" />
          <input v-model="confirmPwd" type="password" placeholder="确认新密码" required />
          <p v-if="pwdError" class="pwd-error">{{ pwdError }}</p>
        </div>
        <div class="btns">
          <button class="btn btn-primary" @click="changePwd" :disabled="pwdBusy">{{ pwdBusy ? '提交中…' : '确认修改' }}</button>
          <button class="btn" @click="showPwd = false" :disabled="pwdBusy">取消</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { api } from '../../api'

const showPwd = ref(false)
const oldPwd = ref('')
const newPwd = ref('')
const confirmPwd = ref('')
const pwdBusy = ref(false)
const pwdError = ref('')

async function changePwd() {
  pwdError.value = ''
  if (newPwd.value !== confirmPwd.value) {
    pwdError.value = '两次输入的新密码不一致'
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

function logout() {
  window.location.href = '/platform/auth/logout'
}
</script>

<style scoped>
.layout { display: flex; min-height: 100vh; }
.side {
  width: 216px;
  background: rgba(10, 15, 30, 0.65);
  backdrop-filter: blur(30px) saturate(150%);
  -webkit-backdrop-filter: blur(30px) saturate(150%);
  border-right: 1px solid var(--glass-border);
  display: flex;
  flex-direction: column;
  padding: 22px 14px 18px;
  position: sticky;
  top: 0;
  height: 100vh;
}
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
nav a.router-link-active {
  background: linear-gradient(135deg, rgba(34, 211, 238, 0.16), rgba(139, 92, 246, 0.16));
  color: var(--text-0);
  border: 1px solid rgba(34, 211, 238, 0.25);
}
nav a.router-link-active .dot {
  background: var(--cyan);
  box-shadow: 0 0 10px var(--cyan);
}
.side-foot { padding: 8px; display: flex; flex-direction: column; gap: 6px; }
.pwd-btn {
  width: 100%;
  color: var(--text-2);
  border-color: var(--glass-border);
  background: transparent;
}
.pwd-btn:hover { color: var(--cyan); border-color: rgba(34, 211, 238, 0.4); }
.modal-mask {
  position: fixed; inset: 0;
  background: rgba(2, 4, 10, 0.7);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  display: flex; align-items: center; justify-content: center; z-index: 100;
  animation: fadeIn 0.2s ease;
}
@keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
.modal {
  background: rgba(14, 20, 38, 0.92);
  backdrop-filter: blur(30px) saturate(150%);
  -webkit-backdrop-filter: blur(30px) saturate(150%);
  border: 1px solid var(--glass-border-strong);
  border-radius: var(--radius-lg);
  padding: 26px;
  width: 420px; max-width: 92vw;
  max-height: 90vh; overflow-y: auto;
  box-shadow: 0 24px 80px rgba(0, 0, 0, 0.6), inset 0 1px 0 rgba(255, 255, 255, 0.08);
  animation: pop 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}
@keyframes pop {
  from { transform: scale(0.96) translateY(10px); opacity: 0; }
  to { transform: scale(1) translateY(0); opacity: 1; }
}
.modal h3 { margin: 0 0 16px; font-size: 15px; letter-spacing: 0.03em; }
.pwd-fields { display: flex; flex-direction: column; gap: 10px; }
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
.btns { margin-top: 16px; display: flex; gap: 8px; }
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
</style>
