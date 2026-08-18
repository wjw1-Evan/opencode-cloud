<template>
  <div>
    <div class="page-head">
      <h2>{{ t('navDashboard') }}</h2>
      <div class="head-right">
        <span class="live" :class="liveClass"><span class="dot"></span>{{ liveLabel }}</span>
        <span class="updated" v-if="updatedAt">{{ t('updatedAt') }} {{ updatedAt }}</span>
      </div>
    </div>

    <div class="grid" v-if="stats">
      <div class="card stat" v-for="(s, i) in cards" :key="i" :style="{ '--i': i }">
        <div class="icon">{{ s.icon }}</div>
        <div class="num" :class="{ small: s.small }">{{ s.value }}</div>
        <div class="label">{{ s.label }}</div>
      </div>
    </div>

    <div class="panels" v-if="stats">
      <div class="panel card">
        <div class="panel-title">{{ t('userStatus') }}</div>
        <div class="stack" v-if="hasUsers">
          <div class="stack-seg" v-for="(seg, i) in userSegs" :key="'u' + i"
               :class="seg.cls" :style="{ width: seg.pct + '%' }"
               :title="`${seg.label}: ${seg.n} (${seg.pct}%)`"></div>
        </div>
        <div class="legend">
          <span v-for="(seg, i) in userSegs" :key="'l' + i" class="legend-item">
            <i :class="seg.cls"></i>{{ seg.label }} <b>{{ seg.n }}</b>
          </span>
        </div>
        <div class="divider"></div>
        <div class="panel-title sub">{{ t('containerStatus') }}</div>
        <div class="stack" v-if="hasContainers">
          <div class="stack-seg" v-for="(seg, i) in contSegs" :key="'c' + i"
               :class="seg.cls" :style="{ width: seg.pct + '%' }"
               :title="`${seg.label}: ${seg.n} (${seg.pct}%)`"></div>
        </div>
        <div class="legend">
          <span v-for="(seg, i) in contSegs" :key="'cl' + i" class="legend-item">
            <i :class="seg.cls"></i>{{ seg.label }} <b>{{ seg.n }}</b>
          </span>
        </div>
      </div>

      <div class="panel card">
        <div class="panel-title">{{ t('trend24h') }}</div>
        <div class="chart">
          <div class="bar" v-for="(v, i) in stats.requests.last24h" :key="i"
               :style="{ height: barH(v) + '%' }"
               :class="{ peak: v > 0 && v === max24h }"
               :title="hourLabel(i) + ':00 — ' + v">
            <span class="bar-val" v-if="v > 0 && v === max24h">{{ v }}</span>
          </div>
        </div>
        <div class="axis">
          <span>0:00</span><span>6:00</span><span>12:00</span><span>18:00</span><span>23:00</span>
        </div>
      </div>

      <div class="panel card">
        <div class="panel-title">{{ t('realtimeResource') }}</div>
        <div class="res-row">
          <div class="res-label">{{ t('cpuUsage') }}</div>
          <div class="res-value mono">{{ stats.resources.cpu_cores.toFixed(2) }} <small>{{ t('core') }}</small></div>
        </div>
        <div class="res-row">
          <div class="res-label">{{ t('memUsage') }}</div>
          <div class="res-value mono">{{ fmtBytes(stats.resources.mem_bytes) }}
            <small>/ {{ fmtBytes(stats.resources.mem_limit) }}</small></div>
        </div>
        <div class="bar-track">
          <div class="bar-fill mem" :style="{ width: memPct + '%' }"></div>
        </div>
        <div class="res-sub mono">{{ memPct.toFixed(1) }}% {{ t('memUsed') }} · {{ t('runningContainersLabel') }} {{ stats.containers.running }}/{{ stats.containers.total }} {{ t('containers') }}</div>
        <div class="divider"></div>
        <div class="res-row">
          <div class="res-label">{{ t('onlineUsers') }}</div>
          <div class="res-value mono">{{ stats.requests.online }} <small>{{ t('recent5min') }}</small></div>
        </div>
        <div class="res-row">
          <div class="res-label">{{ t('avgResponse') }}</div>
          <div class="res-value mono">{{ stats.requests.avg_latency_ms.toFixed(0) }} <small>ms</small></div>
        </div>
        <div class="res-row">
          <div class="res-label">{{ t('templateCount') }}</div>
          <div class="res-value mono">{{ stats.templates.total }}</div>
        </div>
        <div class="res-row">
          <div class="res-label">{{ t('idleStop') }}</div>
          <div class="res-value mono">{{ stats.idle_timeout.minutes > 0 ? stats.idle_timeout.minutes + ' ' + t('minute') : t('off') }}</div>
        </div>
      </div>

      <div class="panel card">
        <div class="panel-title">{{ t('courseDistribution') }}</div>
        <div class="course" v-for="c in courses" :key="c.course || '_'">
          <div class="course-head">
            <span class="course-name">{{ c.course || t('ungrouped') }}</span>
            <span class="course-meta mono">{{ c.running }}/{{ c.users }} {{ t('runningContainersLabel') }}</span>
          </div>
          <div class="bar-track">
            <div class="bar-fill course" :style="{ width: coursePct(c) + '%' }"></div>
          </div>
        </div>
        <p v-if="!courses.length" class="empty">{{ t('noUsers') }}</p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, inject } from 'vue'
import { api, fmtBytes } from '../../api'

const { t } = inject('i18n')
const notify = inject('notify')

const stats = ref(null)
const updatedAt = ref('')
const loading = ref(false)
const live = ref(true)
const paused = ref(false)
const POLL_MS = 5000
let timer = null
let onVis = null

async function refresh() {
  if (loading.value) return
  if (document.hidden) {
    paused.value = true
    return
  }
  paused.value = false
  loading.value = true
  try {
    stats.value = await api.dashboard()
    live.value = true
    updatedAt.value = new Date().toLocaleTimeString(navigator.language || 'zh-CN', { hour12: false })
  } catch (e) {
    live.value = false
    notify(e.message, 'err')
  } finally {
    loading.value = false
  }
}

const liveLabel = computed(() => {
  if (paused.value) return t('livePaused')
  return live.value ? t('liveActive') : t('liveError')
})

const liveClass = computed(() => {
  if (paused.value) return 'paused'
  return live.value ? 'on' : 'err'
})

const cards = computed(() => {
  if (!stats.value) return []
  const s = stats.value
  return [
    { icon: '◉', label: t('totalUsers'), value: s.users.total },
    { icon: '●', label: t('activeUsers'), value: s.users.active },
    { icon: '◈', label: t('onlineUsers'), value: s.requests.online },
    { icon: '▲', label: t('runningContainers'), value: s.containers.running },
    { icon: '▣', label: t('totalContainers'), value: s.containers.total },
    { icon: '↟', label: t('requests24h'), value: s.requests.last24h.reduce((a, b) => a + b, 0) },
    { icon: '↗', label: t('totalVisits'), value: s.requests.count },
    { icon: '◆', label: t('totalTraffic'), value: fmtBytes(s.requests.bytes), small: true },
  ]
})

const STATUS_META = {
  active: { label: t('enabled'), cls: 'ok' },
  disabled: { label: t('disabled'), cls: 'dim' },
  expired: { label: t('expired'), cls: 'err' },
  running: { label: t('running'), cls: 'ok' },
  stopped: { label: t('stopped'), cls: 'dim' },
  error: { label: t('error'), cls: 'err' },
  creating: { label: t('creating'), cls: 'warn' },
  pending: { label: t('pending'), cls: 'cyan' },
  removed: { label: t('removed'), cls: 'ghost' },
}

function segs(m, total) {
  const out = []
  for (const [k, v] of Object.entries(m)) {
    const meta = STATUS_META[k] || { label: k, cls: 'dim' }
    if (!v) continue
    out.push({ ...meta, k, n: v, pct: total ? (v / total * 100).toFixed(1) : 0 })
  }
  out.sort((a, b) => b.n - a.n)
  return out
}

const userSegs = computed(() => stats.value ? segs(stats.value.users.status, stats.value.users.total) : [])
const contSegs = computed(() => stats.value ? segs(stats.value.containers.status, stats.value.containers.total) : [])
const hasUsers = computed(() => stats.value && stats.value.users.total > 0)
const hasContainers = computed(() => stats.value && stats.value.containers.total > 0)

const max24h = computed(() => stats.value ? Math.max(...stats.value.requests.last24h, 1) : 1)
function barH(v) { return v > 0 ? Math.max(4, v / max24h.value * 100) : 2 }
function hourLabel(i) {
  const h = new Date().getHours()
  const start = (h - 23 + 24 + i) % 24
  return String(start).padStart(2, '0')
}

const memPct = computed(() => {
  if (!stats.value || !stats.value.resources.mem_limit) return 0
  return stats.value.resources.mem_bytes / stats.value.resources.mem_limit * 100
})

const courses = computed(() => stats.value ? stats.value.courses : [])
function coursePct(c) {
  const max = Math.max(...courses.value.map(x => x.users), 1)
  return c.users / max * 100
}

onMounted(() => {
  refresh()
  timer = setInterval(refresh, POLL_MS)
  onVis = () => { if (!document.hidden) refresh() }
  document.addEventListener('visibilitychange', onVis)
})
onUnmounted(() => {
  clearInterval(timer)
  if (onVis) document.removeEventListener('visibilitychange', onVis)
})
</script>

<style scoped>
.page-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}
h2 { margin: 0; font-size: 20px; letter-spacing: 0.02em; }
.head-right { display: flex; align-items: center; gap: 14px; }
.live {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font-size: 12px;
  color: var(--text-2);
  padding: 6px 14px;
  border-radius: 999px;
  border: 1px solid var(--glass-border);
  background: var(--glass);
  transition: border-color 0.2s ease, background 0.2s ease;
}
.live.on {
  color: var(--ok);
  border-color: rgba(52, 211, 153, 0.35);
  background: rgba(52, 211, 153, 0.07);
}
.live.err {
  color: var(--err);
  border-color: rgba(248, 113, 113, 0.4);
  background: rgba(248, 113, 113, 0.08);
}
.live.paused {
  color: var(--warn);
  border-color: rgba(251, 191, 36, 0.35);
  background: rgba(251, 191, 36, 0.07);
}
.live .dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
  box-shadow: 0 0 10px currentColor;
}
.live.on .dot {
  animation: pulse 1.8s ease-in-out infinite;
}
@keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.35; } }
.updated { font-size: 12px; color: var(--text-2); font-family: var(--font-mono); }

.grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 16px;
}
.stat {
  text-align: center;
  padding: 24px 18px;
  position: relative;
  overflow: hidden;
  animation: rise 0.5s cubic-bezier(0.16, 1, 0.3, 1) both;
  animation-delay: calc(var(--i) * 0.05s);
}
@keyframes rise {
  from { transform: translateY(14px); opacity: 0; }
  to { transform: translateY(0); opacity: 1; }
}
.stat::after {
  content: "";
  position: absolute;
  bottom: 0; left: 20%;
  width: 60%; height: 2px;
  background: var(--grad);
  opacity: 0.6;
  border-radius: 2px;
  filter: blur(1px);
}
.stat:hover { border-color: rgba(34, 211, 238, 0.4); box-shadow: 0 0 30px rgba(34, 211, 238, 0.15), var(--shadow); }
.icon {
  width: 38px;
  height: 38px;
  margin: 0 auto 12px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 17px;
  color: var(--cyan);
  background: rgba(34, 211, 238, 0.08);
  border: 1px solid rgba(34, 211, 238, 0.25);
  box-shadow: 0 0 14px rgba(34, 211, 238, 0.12);
  transition: transform 0.25s ease, box-shadow 0.25s ease;
}
.stat:nth-child(even) .icon {
  color: var(--violet);
  background: rgba(139, 92, 246, 0.08);
  border-color: rgba(139, 92, 246, 0.3);
  box-shadow: 0 0 14px rgba(139, 92, 246, 0.15);
}
.stat:hover .icon { transform: translateY(-2px) scale(1.06); }
.stat:nth-child(even):hover .icon { box-shadow: 0 0 22px rgba(139, 92, 246, 0.35); }
.stat:hover .icon { box-shadow: 0 0 22px rgba(34, 211, 238, 0.3); }
.num {
  font-size: 32px;
  font-weight: 700;
  font-family: var(--font-mono);
  background: var(--grad);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}
.num.small { font-size: 24px; }
.label { color: var(--text-2); font-size: 12.5px; margin-top: 6px; letter-spacing: 0.05em; }

.panels {
  display: grid;
  grid-template-columns: 1fr 1.4fr;
  gap: 16px;
}
.panel { min-height: 150px; }
.panel-title {
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.08em;
  color: var(--text-1);
  margin-bottom: 16px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.panel-title::before {
  content: "";
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--grad);
  box-shadow: 0 0 8px rgba(34, 211, 238, 0.7);
  flex-shrink: 0;
}
.panel-title.sub { margin-top: 20px; }
.divider {
  height: 1px;
  margin: 18px 0;
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.12), transparent);
}

/* ---- stacked status bars ---- */
.stack {
  display: flex;
  height: 12px;
  border-radius: 999px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.06);
  margin-bottom: 14px;
}
.stack-seg { height: 100%; transition: width 0.6s cubic-bezier(0.16, 1, 0.3, 1); }
.stack-seg.ok { background: var(--ok); }
.stack-seg.dim { background: var(--text-2); }
.stack-seg.err { background: var(--err); }
.stack-seg.warn { background: var(--warn); }
.stack-seg.cyan { background: var(--cyan); }
.stack-seg.ghost { background: rgba(255, 255, 255, 0.15); }
.legend { display: flex; flex-wrap: wrap; gap: 8px 18px; }
.legend-item { font-size: 12px; color: var(--text-2); display: inline-flex; align-items: center; gap: 6px; }
.legend-item i { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }
.legend-item i.ok { background: var(--ok); box-shadow: 0 0 6px var(--ok); }
.legend-item i.dim { background: var(--text-2); }
.legend-item i.err { background: var(--err); box-shadow: 0 0 6px var(--err); }
.legend-item i.warn { background: var(--warn); }
.legend-item i.cyan { background: var(--cyan); box-shadow: 0 0 6px var(--cyan); }
.legend-item i.ghost { background: rgba(255, 255, 255, 0.25); }
.legend-item b { color: var(--text-0); font-family: var(--font-mono); font-weight: 600; }

/* ---- 24h bar chart ---- */
.chart {
  display: flex;
  align-items: flex-end;
  gap: 3px;
  height: 138px;
  padding: 4px 2px 0;
}
.bar {
  flex: 1;
  min-width: 4px;
  border-radius: 3px 3px 1px 1px;
  background: linear-gradient(180deg, rgba(34, 211, 238, 0.75), rgba(34, 211, 238, 0.15));
  position: relative;
  transition: height 0.6s cubic-bezier(0.16, 1, 0.3, 1);
}
.bar:hover { background: linear-gradient(180deg, var(--cyan), rgba(34, 211, 238, 0.3)); }
.bar.peak {
  background: linear-gradient(180deg, var(--violet), rgba(139, 92, 246, 0.2));
  box-shadow: 0 0 10px rgba(139, 92, 246, 0.4);
}
.bar-val {
  position: absolute;
  top: -20px;
  left: 50%;
  transform: translateX(-50%);
  font-size: 10px;
  font-family: var(--font-mono);
  color: var(--violet);
}
.axis { display: flex; justify-content: space-between; margin-top: 10px; font-size: 10.5px; color: var(--text-2); }

/* ---- resources ---- */
.res-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 7px 0;
  font-size: 13px;
}
.res-label { color: var(--text-1); }
.res-value { color: var(--text-0); font-weight: 600; }
.res-value small { color: var(--text-2); font-weight: 400; font-size: 11.5px; }
.mono { font-family: var(--font-mono); }
.bar-track {
  height: 10px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.06);
  overflow: hidden;
  margin: 6px 0 4px;
}
.bar-fill {
  height: 100%;
  border-radius: 999px;
  transition: width 0.6s cubic-bezier(0.16, 1, 0.3, 1);
}
.bar-fill.mem {
  background: linear-gradient(90deg, var(--cyan), var(--violet));
  box-shadow: 0 0 8px rgba(34, 211, 238, 0.5);
}
.bar-fill.course {
  background: linear-gradient(90deg, rgba(34, 211, 238, 0.8), rgba(139, 92, 246, 0.8));
}
.res-sub { font-size: 11px; color: var(--text-2); margin-top: 6px; }

/* ---- courses ---- */
.course { margin-bottom: 14px; }
.course-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 5px;
}
.course-name { font-size: 13px; color: var(--text-0); }
.course-meta { font-size: 11px; color: var(--text-2); }
.empty { color: var(--text-2); font-size: 13px; }

@media (max-width: 1100px) {
  .grid { grid-template-columns: repeat(2, 1fr); }
  .panels { grid-template-columns: 1fr; }
}
@media (max-width: 720px) {
  .grid { grid-template-columns: repeat(2, 1fr); }
  .stat { padding: 18px 12px; }
  .num { font-size: 24px; }
  .icon { width: 34px; height: 34px; }
}
</style>
