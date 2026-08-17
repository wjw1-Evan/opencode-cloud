<template>
  <div>
    <div class="page-head">
      <h2>总览</h2>
      <div class="head-right">
        <span class="live"><span class="dot"></span>系统在线</span>
        <span class="updated" v-if="updatedAt">更新于 {{ updatedAt }}</span>
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
        <div class="panel-title">用户状态</div>
        <div class="stack" v-if="hasUsers">
          <div class="stack-seg" v-for="(seg, i) in userSegs" :key="'u' + i"
               :class="seg.cls" :style="{ width: seg.pct + '%' }"
               :title="`${seg.label}: ${seg.n}（${seg.pct}%）`"></div>
        </div>
        <div class="legend">
          <span v-for="(seg, i) in userSegs" :key="'l' + i" class="legend-item">
            <i :class="seg.cls"></i>{{ seg.label }} <b>{{ seg.n }}</b>
          </span>
        </div>
        <div class="divider"></div>
        <div class="panel-title sub">容器状态</div>
        <div class="stack" v-if="hasContainers">
          <div class="stack-seg" v-for="(seg, i) in contSegs" :key="'c' + i"
               :class="seg.cls" :style="{ width: seg.pct + '%' }"
               :title="`${seg.label}: ${seg.n}（${seg.pct}%）`"></div>
        </div>
        <div class="legend">
          <span v-for="(seg, i) in contSegs" :key="'cl' + i" class="legend-item">
            <i :class="seg.cls"></i>{{ seg.label }} <b>{{ seg.n }}</b>
          </span>
        </div>
      </div>

      <div class="panel card">
        <div class="panel-title">近 24 小时访问趋势</div>
        <div class="chart">
          <div class="bar" v-for="(v, i) in stats.requests.last24h" :key="i"
               :style="{ height: barH(v) + '%' }"
               :class="{ peak: v > 0 && v === max24h }"
               :title="hourLabel(i) + '时：' + v + ' 次'">
            <span class="bar-val" v-if="v > 0 && v === max24h">{{ v }}</span>
          </div>
        </div>
        <div class="axis">
          <span>0时</span><span>6时</span><span>12时</span><span>18时</span><span>23时</span>
        </div>
      </div>

      <div class="panel card">
        <div class="panel-title">实时资源占用</div>
        <div class="res-row">
          <div class="res-label">CPU 总用量</div>
          <div class="res-value mono">{{ stats.resources.cpu_cores.toFixed(2) }} <small>核</small></div>
        </div>
        <div class="res-row">
          <div class="res-label">内存占用</div>
          <div class="res-value mono">{{ fmtBytes(stats.resources.mem_bytes) }}
            <small>/ {{ fmtBytes(stats.resources.mem_limit) }}</small></div>
        </div>
        <div class="bar-track">
          <div class="bar-fill mem" :style="{ width: memPct + '%' }"></div>
        </div>
        <div class="res-sub mono">{{ memPct.toFixed(1) }}% 已用 · 运行中 {{ stats.containers.running }}/{{ stats.containers.total }} 容器</div>
        <div class="divider"></div>
        <div class="res-row">
          <div class="res-label">在线用户</div>
          <div class="res-value mono">{{ stats.requests.online }} <small>最近5分钟</small></div>
        </div>
        <div class="res-row">
          <div class="res-label">平均响应</div>
          <div class="res-value mono">{{ stats.requests.avg_latency_ms.toFixed(0) }} <small>ms</small></div>
        </div>
        <div class="res-row">
          <div class="res-label">镜像模板</div>
          <div class="res-value mono">{{ stats.templates.total }}</div>
        </div>
        <div class="res-row">
          <div class="res-label">空闲自动停止</div>
          <div class="res-value mono">{{ stats.idle_timeout.minutes > 0 ? stats.idle_timeout.minutes + ' 分钟' : '关闭' }}</div>
        </div>
      </div>

      <div class="panel card">
        <div class="panel-title">课程分布</div>
        <div class="course" v-for="c in courses" :key="c.course || '_'">
          <div class="course-head">
            <span class="course-name">{{ c.course || '未分组' }}</span>
            <span class="course-meta mono">{{ c.running }}/{{ c.users }} 运行中</span>
          </div>
          <div class="bar-track">
            <div class="bar-fill course" :style="{ width: coursePct(c) + '%' }"></div>
          </div>
        </div>
        <p v-if="!courses.length" class="empty">暂无用户，请先批量建号</p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { api, fmtBytes } from '../../api'

const stats = ref(null)
const updatedAt = ref('')
const POLL_MS = 1000
let timer = null

async function refresh() {
  stats.value = await api.dashboard()
  updatedAt.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
}

const cards = computed(() => {
  if (!stats.value) return []
  const s = stats.value
  return [
    { icon: '◉', label: '用户总数', value: s.users.total },
    { icon: '●', label: '活跃用户', value: s.users.active },
    { icon: '◈', label: '在线用户', value: s.requests.online },
    { icon: '▲', label: '运行中容器', value: s.containers.running },
    { icon: '▣', label: '容器总数', value: s.containers.total },
    { icon: '↟', label: '24h 请求', value: s.requests.last24h.reduce((a, b) => a + b, 0) },
    { icon: '↗', label: '累计访问', value: s.requests.count },
    { icon: '◆', label: '累计流量', value: fmtBytes(s.requests.bytes), small: true },
  ]
})

const STATUS_META = {
  active: { label: '启用', cls: 'ok' },
  disabled: { label: '停用', cls: 'dim' },
  expired: { label: '过期', cls: 'err' },
  running: { label: '运行', cls: 'ok' },
  stopped: { label: '停止', cls: 'dim' },
  error: { label: '异常', cls: 'err' },
  creating: { label: '创建中', cls: 'warn' },
  pending: { label: '待建', cls: 'cyan' },
  removed: { label: '已删', cls: 'ghost' },
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
})
onUnmounted(() => clearInterval(timer))
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
  border: 1px solid rgba(52, 211, 153, 0.3);
  background: rgba(52, 211, 153, 0.06);
}
.live .dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--ok);
  box-shadow: 0 0 10px var(--ok);
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
}
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
  height: 130px;
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
  height: 8px;
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
</style>
