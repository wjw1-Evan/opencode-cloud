<template>
  <div class="login-wrap">
    <div class="nebula n1"></div>
    <div class="nebula n2"></div>
    <div class="grid-overlay"></div>

    <div class="login-shell">
      <div class="card intro">
        <div class="intro-brand">
          <div class="intro-logo">◈</div>
          <div>
            <h1 class="intro-title">DevCapsule</h1>
            <p class="intro-sub">{{ t('brandSub') }}</p>
          </div>
        </div>
        <p class="intro-desc">{{ t('introDesc') }}</p>
        <div class="features">
          <div class="feature" v-for="f in features" :key="f.titleKey">
            <span class="f-icon">{{ f.icon }}</span>
            <div class="f-text">
              <b>{{ t(f.titleKey) }}</b>
              <p>{{ t(f.descKey) }}</p>
            </div>
          </div>
        </div>
      </div>

      <div class="card login-card">
        <div class="brand">
          <div class="logo">◈</div>
          <h2>{{ t('loginTitle') }}</h2>
          <p class="sub">{{ t('welcomeSub') }}</p>
        </div>

        <form @submit.prevent="submit">
          <div style="margin-bottom: 14px">
            <label>{{ t('username') }}</label>
            <input v-model="username" autocomplete="username" :placeholder="t('usernamePlaceholder')" required />
          </div>
          <div style="margin-bottom: 20px">
            <label>{{ t('password') }}</label>
            <input v-model="password" type="password" autocomplete="current-password" :placeholder="t('passwordPlaceholder')" required />
          </div>
          <button class="btn btn-primary submit" type="submit" :disabled="loading">
            <span class="scan" v-if="loading"></span>
            {{ loading ? t('logging') : t('login') }}
          </button>
          <p v-if="error" class="err">{{ error }}</p>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, inject, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'

const { t } = inject('i18n')

const username = ref('')
const password = ref('')
const loading = ref(false)
const error = ref('')
const router = useRouter()

const features = [
  { icon: '◈', titleKey: 'feat1Title', descKey: 'feat1Desc' },
  { icon: '▦', titleKey: 'feat2Title', descKey: 'feat2Desc' },
  { icon: '⚡', titleKey: 'feat3Title', descKey: 'feat3Desc' },
  { icon: '☁', titleKey: 'feat4Title', descKey: 'feat4Desc' },
]

onMounted(async () => {
  try {
    const data = await api.initialized()
    if (!data.initialized) {
      router.push('/initialize')
    }
  } catch {}
})

async function submit() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.login(username.value, password.value)
    if (data.user && data.user.role === 'admin') {
      router.push('/admin/dashboard')
    } else {
      router.push('/portal')
    }
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
  filter: blur(60px);
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
.login-shell {
  display: flex;
  align-items: stretch;
  gap: 28px;
  position: relative;
  z-index: 2;
}
.intro {
  width: 400px;
  padding: 34px 30px;
  text-align: left;
  display: flex;
  flex-direction: column;
}
.intro-brand {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 18px;
}
.intro-logo {
  width: 48px;
  height: 48px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 22px;
  color: #04101a;
  background: var(--grad);
  box-shadow: 0 6px 24px rgba(34, 211, 238, 0.4);
  flex-shrink: 0;
}
.intro-title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
  letter-spacing: 0.02em;
  background: var(--grad);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}
.intro-sub {
  margin: 2px 0 0;
  color: var(--text-2);
  font-size: 12px;
  letter-spacing: 0.12em;
}
.intro-desc {
  color: var(--text-1);
  font-size: 13.5px;
  line-height: 1.7;
  margin: 0 0 20px;
}
.features {
  display: flex;
  flex-direction: column;
  gap: 14px;
  margin-top: auto;
}
.feature {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}
.f-icon {
  width: 34px;
  height: 34px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 15px;
  color: var(--cyan);
  background: rgba(34, 211, 238, 0.08);
  border: 1px solid rgba(34, 211, 238, 0.25);
  box-shadow: 0 0 12px rgba(34, 211, 238, 0.1);
  flex-shrink: 0;
}
.f-text b { color: var(--text-0); font-size: 13.5px; }
.f-text p { margin: 2px 0 0; color: var(--text-2); font-size: 12px; line-height: 1.6; }
.login-card {
  width: 360px;
  padding: 36px 32px;
  text-align: center;
  position: relative;
  z-index: 2;
  display: flex;
  flex-direction: column;
  justify-content: center;
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
h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  letter-spacing: 0.03em;
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

@media (max-width: 960px) {
  .intro { display: none; }
  .login-card { width: 380px; }
}
@media (max-width: 720px) {
  .login-shell { gap: 0; }
  .login-card {
    width: calc(100vw - 28px);
    max-width: 380px;
    padding: 30px 22px;
  }
}
</style>
