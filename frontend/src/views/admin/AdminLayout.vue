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
        <RouterLink to="/admin/help"><span class="dot"></span>使用帮助</RouterLink>
      </nav>
      <div class="side-foot">
        <button class="btn logout" @click="logout">退出登录</button>
      </div>
    </aside>
    <main class="main">
      <router-view />
    </main>
  </div>
</template>

<script setup>
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
.side-foot { padding: 8px; }
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
