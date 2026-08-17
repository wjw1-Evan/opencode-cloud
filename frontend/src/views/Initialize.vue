<template>
  <div class="login-wrap">
    <div class="nebula n1"></div>
    <div class="nebula n2"></div>
    <div class="grid-overlay"></div>

    <div class="card login-card">
      <div class="brand">
        <div class="logo">◈</div>
        <h1>DevCapsule <span class="zh">开发胶囊舱</span></h1>
        <p class="sub">首次部署，请设置管理员账户</p>
      </div>

      <form @submit.prevent="submit">
        <div style="margin-bottom: 14px">
          <label>管理员用户名</label>
          <input v-model="username" autocomplete="username" required placeholder="请设置管理员用户名" />
        </div>
        <div style="margin-bottom: 14px">
          <label>管理员密码</label>
          <input v-model="password" type="password" autocomplete="new-password" required placeholder="至少 8 位" minlength="8" />
        </div>
        <div style="margin-bottom: 20px">
          <label>确认密码</label>
          <input v-model="confirmPassword" type="password" autocomplete="new-password" required placeholder="再次输入密码" minlength="8" />
        </div>
        <button class="btn btn-primary submit" type="submit" :disabled="loading">
          <span class="scan" v-if="loading"></span>
          {{ loading ? '正在初始化…' : '完成设置' }}
        </button>
        <p v-if="error" class="err">{{ error }}</p>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'

const username = ref('')
const password = ref('')
const confirmPassword = ref('')
const loading = ref(false)
const error = ref('')
const router = useRouter()

async function submit() {
  if (password.value !== confirmPassword.value) {
    error.value = '两次输入的密码不一致'
    return
  }
  loading.value = true
  error.value = ''
  try {
    await api.initialize(username.value, password.value)
    router.push('/')
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-wrap {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  overflow: hidden;
  background:
    radial-gradient(1000px 500px at 20% 20%, rgba(34, 211, 238, 0.15), transparent 60%),
    radial-gradient(900px 600px at 85% 85%, rgba(139, 92, 246, 0.2), transparent 60%),
    #04060d;
}
.nebula {
  position: absolute;
  border-radius: 50%;
  filter: blur(90px);
  opacity: 0.5;
  animation: drift 18s ease-in-out infinite alternate;
}
.n1 { width: 420px; height: 420px; background: rgba(34, 211, 238, 0.35); top: -100px; left: -80px; }
.n2 { width: 520px; height: 520px; background: rgba(139, 92, 246, 0.35); bottom: -160px; right: -120px; animation-delay: -9s; }
@keyframes drift {
  from { transform: translate(0, 0) scale(1); }
  to { transform: translate(60px, 40px) scale(1.15); }
}
.grid-overlay {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(34, 211, 238, 0.05) 1px, transparent 1px),
    linear-gradient(90deg, rgba(34, 211, 238, 0.05) 1px, transparent 1px);
  background-size: 44px 44px;
  mask-image: radial-gradient(ellipse at center, black 30%, transparent 75%);
  -webkit-mask-image: radial-gradient(ellipse at center, black 30%, transparent 75%);
}
.login-card {
  width: 380px;
  padding: 40px 34px;
  text-align: center;
  position: relative;
  z-index: 2;
  border: 1px solid rgba(255, 255, 255, 0.14);
  box-shadow: 0 24px 80px rgba(0, 0, 0, 0.6), inset 0 1px 0 rgba(255, 255, 255, 0.1);
}
.brand { margin-bottom: 28px; }
.logo {
  width: 56px;
  height: 56px;
  margin: 0 auto 16px;
  border-radius: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 26px;
  color: #04101a;
  background: var(--grad);
  box-shadow: 0 8px 30px rgba(34, 211, 238, 0.45);
}
h1 {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
  letter-spacing: 0.02em;
  background: var(--grad);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}
h1 .zh {
  margin-left: 4px;
  font-size: 14px;
  font-weight: 600;
  letter-spacing: 0.15em;
  background: none;
  -webkit-text-fill-color: var(--text-1);
}
.sub { color: var(--text-2); font-size: 13px; margin: 8px 0 0; }
form { text-align: left; }
.submit { width: 100%; padding: 13px; font-size: 15px; position: relative; overflow: hidden; }
.scan {
  position: absolute;
  top: 0; left: -60%;
  width: 50%; height: 100%;
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.45), transparent);
  animation: scanline 1.2s linear infinite;
}
@keyframes scanline { from { left: -60%; } to { left: 120%; } }
.err { color: var(--err); font-size: 13px; margin-top: 14px; text-align: center; }
</style>
