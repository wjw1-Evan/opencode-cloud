<template>
  <div>
    <div class="page-head">
      <div class="head-left">
        <h2>用户与容器</h2>
        <span class="meta">用户与容器一一对应</span>
      </div>
      <div class="head-right">
        <span class="updated" v-if="updatedAt">更新于 {{ updatedAt }}</span>
        <button class="btn" @click="refresh" :disabled="loading">{{ loading ? '刷新中…' : '刷新' }}</button>
      </div>
    </div>

    <div class="card batch">
      <h3>批量创建账号 · 自动建容器</h3>
      <div class="row">
        <div class="span2"><label>课程（用于生成用户名，如"Python 基础"→ python001）</label><input v-model="course" placeholder="如 Python 基础 / 2026-春" required /></div>
        <div><label>数量</label><input v-model.number="count" type="number" min="1" max="500" /></div>
        <div><label>模板</label>
          <select v-model="tplId">
            <option v-for="t in templates" :key="t.id" :value="t.id">{{ t.name }}</option>
          </select>
        </div>
        <div class="row-btns">
          <button class="btn btn-primary" @click="batchCreate" :disabled="busy">{{ busy ? '创建中…' : '生成账号并建容器' }}</button>
        </div>
      </div>
      <p class="hint">用户名由课程名自动生成（取字母数字，如"Python 基础"→ python001），自动生成随机密码并立即建容器；完成后浏览器会下载 accounts.csv。</p>
    </div>

    <div class="card table-card">
      <div class="toolbar">
        <select v-model="courseFilter" class="course-filter">
          <option value="">全部课程</option>
          <option v-for="c in courses" :key="c" :value="c">{{ c }}</option>
        </select>
        <span class="sel-info">已选 {{ selected.size }} 个用户</span>
        <div class="toolbar-btns">
          <button class="btn" :disabled="!selected.size || busyProvision" @click="bulkProvision">{{ busyProvision ? '重建中…' : '重建' }}</button>
          <button class="btn" :disabled="!selected.size || !!busyAction" @click="bulkAction('start')">{{ busyAction === 'start' ? '启动中…' : '启动' }}</button>
          <button class="btn" :disabled="!selected.size || !!busyAction" @click="bulkAction('restart')">{{ busyAction === 'restart' ? '重启中…' : '重启' }}</button>
          <button class="btn" :disabled="!selected.size || !!busyAction" @click="bulkAction('stop')">{{ busyAction === 'stop' ? '停止中…' : '停止' }}</button>
          <button class="btn btn-danger" :disabled="!selected.size || !!busyAction" @click="bulkAction('delete')">{{ busyAction === 'delete' ? '删除中…' : '删除' }}</button>
        </div>
      </div>
      <table>
        <thead>
          <tr>
            <th class="chk"><input type="checkbox" :checked="allChecked" @change="toggleAll" /></th>
            <th>用户名</th><th>密码</th><th>课程</th><th>状态</th>
            <th>容器名</th><th>容器状态</th><th>端口</th><th>创建时间</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in visibleUsers" :key="u.id" :class="{ selected: selected.has(u.id) }">
            <td class="chk">
              <input v-if="u.role !== 'admin'" type="checkbox" :checked="selected.has(u.id)" @change="toggle(u.id)" />
            </td>
            <td class="uname">{{ u.username }}</td>
            <td><code v-if="u.password" class="pwd">{{ u.password }}</code><span v-else class="dim">-</span></td>
            <td><span v-if="u.course" class="badge course">{{ u.course }}</span><span v-else class="dim">-</span></td>
            <td><span class="badge" :class="u.status">{{ u.status }}</span></td>
            <td>
              <code v-if="cont(u.id)">{{ cont(u.id).container_name }}</code>
              <span v-else class="dim">-</span>
            </td>
            <td>
              <span v-if="cont(u.id)" class="badge" :class="cont(u.id).status">{{ cont(u.id).status }}</span>
              <span v-else class="dim">-</span>
            </td>
            <td>
              <span v-if="cont(u.id)" class="ports">
                {{ cont(u.id).internal_port }}{{ (cont(u.id).extra_ports || []).length ? ' / ' + cont(u.id).extra_ports.join(',') : '' }}
              </span>
              <span v-else class="dim">-</span>
            </td>
            <td class="time">{{ fmtTime(u.created_at) }}</td>
          </tr>
        </tbody>
      </table>
      <p v-if="!visibleUsers.length" class="empty">暂无用户</p>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, inject } from 'vue'
import { api, downloadText } from '../../api'

const notify = inject('notify')
const users = ref([])
const templates = ref([])
const containers = ref([])
const count = ref(1)
const course = ref('')
const courseFilter = ref('')
const tplId = ref('')
const busy = ref(false)
const busyProvision = ref(false)
const busyAction = ref('')
const selected = ref(new Set())
const loading = ref(false)
const updatedAt = ref('')
const POLL_MS = 1000
let timer = null

const courses = computed(() => {
  const set = new Set(users.value.map((u) => u.course).filter(Boolean))
  return [...set].sort()
})

const visibleUsers = computed(() => {
  if (!courseFilter.value) return users.value
  return users.value.filter((u) => u.course === courseFilter.value)
})

const allChecked = computed(() => {
  const selectable = visibleUsers.value.filter((u) => u.role !== 'admin')
  return selectable.length > 0 && selectable.every((u) => selected.value.has(u.id))
})

function toggle(id) {
  const s = new Set(selected.value)
  if (s.has(id)) s.delete(id)
  else s.add(id)
  selected.value = s
}

function toggleAll() {
  const s = new Set(selected.value)
  if (allChecked.value) {
    visibleUsers.value.forEach((u) => s.delete(u.id))
  } else {
    visibleUsers.value.forEach((u) => { if (u.role !== 'admin') s.add(u.id) })
  }
  selected.value = s
}

onMounted(() => {
  refresh()
  timer = setInterval(refresh, POLL_MS)
})
onUnmounted(() => clearInterval(timer))

async function refresh() {
  if (loading.value) return
  loading.value = true
  try {
    await load()
    updatedAt.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
  } finally {
    loading.value = false
  }
}

async function load() {
  users.value = await api.listUsers()
  templates.value = await api.listTemplates()
  containers.value = await api.listContainers()
  if (templates.value.length && !tplId.value) tplId.value = templates.value[0].id
}

function cont(userId) {
  return containers.value.find((c) => c.user_id === userId) || null
}

async function bulkProvision() {
  if (!selected.value.size) return notify('请先选择用户', 'err')
  if (!confirm(`重建选中的 ${selected.value.size} 个用户的容器？将删除并重建现有容器（数据卷保留，使用各自原模板）。`)) return
  busyProvision.value = true
  try {
    const pc = await api.provisionBatch({ template_id: tplId.value || '', user_ids: [...selected.value], force: true })
    const fails = (pc.results || []).filter((x) => !x.ok)
    if (fails.length) {
      notify(`已重建 ${pc.provisioned} 个，${fails.length} 个失败（如：${fails[0].error}）`, 'err')
    } else {
      notify(`已重建 ${pc.provisioned} 个容器`)
    }
    await load()
  } catch (e) {
    notify(e.message, 'err')
  } finally {
    busyProvision.value = false
  }
}

async function bulkAction(action) {
  if (!selected.value.size) return notify('请先选择用户', 'err')
  const names = [...selected.value].map((id) => users.value.find((u) => u.id === id)).filter(Boolean).map((u) => u.username)
  const label = { start: '启动', restart: '重启', stop: '停止', delete: '删除' }[action]
  if (!confirm(`确认对 ${names.length} 个用户执行"${label}"？`)) return
  busyAction.value = action
  try {
    const results = await api.batchUserAction({ user_ids: [...selected.value], action })
    const okN = results.filter((r) => r.ok).length
    notify(`${label}完成：成功 ${okN}，失败 ${results.length - okN}`)
    if (action === 'delete') selected.value = new Set()
    await load()
  } catch (e) {
    notify(e.message, 'err')
  } finally {
    busyAction.value = ''
  }
}

async function batchCreate() {
  if (!tplId.value) return notify('请先选择模板', 'err')
  if (!course.value.trim()) return notify('请填写课程名称', 'err')
  busy.value = true
  try {
    const r = await api.batchUsers({
      count: count.value,
      course: course.value.trim(),
      password_length: 12,
      expires_in_days: 0,
      cpu_limit: 0.5,
      mem_limit: 1 << 30,
    })
    if (!r.created) throw new Error('未创建任何账号')
    const ids = (r.users || []).map((u) => u.id)
    const pc = await api.provisionBatch({ template_id: tplId.value, user_ids: ids, force: false })
    const csv = r.accounts.map((a) => `${a.username},${a.password}`).join('\n')
    downloadText('accounts.csv', 'username,password\n' + csv, 'text/csv')
    const fails = (pc.results || []).filter((x) => !x.ok)
    if (fails.length) {
      notify(`已创建 ${r.created} 个账号，但 ${fails.length} 个容器创建失败（如：${fails[0].error}）`, 'err')
    } else {
      notify(`已创建 ${r.created} 个账号，容器就绪 ${pc.provisioned} 个，账号密码已下载`)
    }
    await load()
  } catch (e) {
    notify(e.message, 'err')
  } finally {
    busy.value = false
  }
}

function fmtTime(ts) {
  return new Date(ts).toLocaleString('zh-CN')
}
</script>

<style scoped>
.page-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}
.head-left { display: flex; align-items: baseline; gap: 12px; }
.head-right { display: flex; align-items: center; gap: 14px; }
.updated { font-size: 12px; color: var(--text-2); font-family: var(--font-mono); }
h2 { margin: 0; font-size: 20px; letter-spacing: 0.02em; }
.meta { color: var(--text-2); font-size: 12.5px; }
.batch { margin-bottom: 18px; }
h3 { margin: 0 0 14px; font-size: 14.5px; color: var(--text-0); letter-spacing: 0.03em; }
.row { display: flex; gap: 14px; align-items: flex-end; flex-wrap: wrap; }
.row > div { flex: 1; min-width: 140px; }
.row .span2 { flex: 2; min-width: 240px; }
.row-btns { display: flex; gap: 8px; flex: 2 !important; }
.hint { color: var(--text-2); font-size: 12px; margin-top: 12px; }
.toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}
.course-filter { width: auto; min-width: 140px; }
.sel-info { color: var(--text-2); font-size: 12.5px; margin-right: auto; }
.toolbar-btns { display: flex; gap: 8px; flex-wrap: wrap; }
.chk { width: 36px; }
.chk input { width: 15px; height: 15px; accent-color: var(--cyan); cursor: pointer; }
.uname { font-weight: 600; }
.pwd { font-family: var(--font-mono); font-size: 12px; }
.ports { font-family: var(--font-mono); font-size: 12px; color: var(--text-1); }
.time { color: var(--text-2); font-size: 12px; white-space: nowrap; }
.dim { color: var(--text-2); }
.empty { color: var(--text-2); font-size: 13px; padding: 16px; text-align: center; }
</style>
