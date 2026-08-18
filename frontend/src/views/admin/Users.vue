<template>
  <div>
    <div class="page-head">
      <div class="head-left">
        <h2>{{ t('usersTitle') }}</h2>
        <span class="meta">{{ t('usersSub') }}</span>
      </div>
      <div class="head-right">
        <span class="updated" v-if="updatedAt">{{ t('updatedAt') }} {{ updatedAt }}</span>
      </div>
    </div>

    <div class="card batch">
      <h3>{{ t('batchCreate') }}</h3>
      <div class="row">
        <div class="span2"><label>{{ t('course') }}{{ t('courseHint') }}</label><input v-model="course" :placeholder="t('coursePlaceholder')" required /></div>
        <div><label>{{ t('count') }}</label><input v-model.number="count" type="number" min="1" max="500" /></div>
        <div><label>{{ t('template') }}</label>
          <select v-model="tplId">
            <option v-for="t in templates" :key="t.id" :value="t.id">{{ t.name }}</option>
          </select>
        </div>
        <div class="row-btns">
          <button class="btn btn-primary" @click="batchCreate" :disabled="busy">{{ busy ? t('createAndProvisioning') : t('createAndProvision') }}</button>
        </div>
      </div>
      <p class="hint">
        {{ t('batchHint') }}
        <span v-if="platformNetwork" class="net-hint">{{ t('platformNetwork') }} <code>{{ platformNetwork }}</code></span>
      </p>
    </div>

    <div class="card table-card">
      <div class="toolbar">
        <select v-model="courseFilter" class="course-filter">
          <option value="">{{ t('allCourses') }}</option>
          <option v-for="c in courses" :key="c" :value="c">{{ c }}</option>
        </select>
        <span class="sel-info">{{ t('selectedCount', { n: selected.size }) }}</span>
        <div class="toolbar-btns">
          <button class="btn" :disabled="!selected.size || !!busyAction" @click="bulkAction('rebuild')">{{ busyAction === 'rebuild' ? t('rebuilding') : t('rebuild') }}</button>
          <button class="btn" :disabled="!selected.size || !!busyAction" @click="bulkAction('start')">{{ busyAction === 'start' ? t('starting') : t('start') }}</button>
          <button class="btn" :disabled="!selected.size || !!busyAction" @click="bulkAction('restart')">{{ busyAction === 'restart' ? t('restarting') : t('restart') }}</button>
          <button class="btn" :disabled="!selected.size || !!busyAction" @click="bulkAction('stop')">{{ busyAction === 'stop' ? t('stopping') : t('stop') }}</button>
          <button class="btn" :disabled="!selected.size || !!busyAction" @click="bulkAction('disable')">{{ busyAction === 'disable' ? t('disabling') : t('disable') }}</button>
          <button class="btn" :disabled="!selected.size || !!busyAction" @click="bulkAction('enable')">{{ busyAction === 'enable' ? t('enabling') : t('enable') }}</button>
          <button class="btn btn-danger" :disabled="!selected.size || !!busyAction" @click="bulkAction('delete')">{{ busyAction === 'delete' ? t('deleting') : t('delete') }}</button>
        </div>
      </div>
      <table>
        <thead>
          <tr>
            <th class="chk"><input type="checkbox" :checked="allChecked" @change="toggleAll" /></th>
            <th>{{ t('thUsername') }}</th><th>{{ t('thPassword') }}</th><th>{{ t('thCourse') }}</th><th>{{ t('thStatus') }}</th>
            <th>{{ t('thTemplate') }}</th><th>{{ t('thContainerStatus') }}</th><th>{{ t('thCreatedAt') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in visibleUsers" :key="u.id" :class="{ selected: selected.has(u.id) }">
            <td class="chk">
              <input v-if="u.role !== 'admin'" type="checkbox" :checked="selected.has(u.id)" @change="toggle(u.id)" />
            </td>
            <td class="uname" :data-label="t('thUsername')">{{ u.username }}</td>
            <td :data-label="t('thPassword')"><code v-if="u.password" class="pwd">{{ u.password }}</code><span v-else class="dim">-</span></td>
            <td :data-label="t('thCourse')"><span v-if="u.course" class="course-txt">{{ u.course }}</span><span v-else class="dim">-</span></td>
            <td :data-label="t('thStatus')">
              <span class="badge clickable" :class="effStatus(u)" @click="openDetail('status', u)">{{ statusLabel(effStatus(u)) }}</span>
            </td>
            <td :data-label="t('thTemplate')">
              <span v-if="u.container" class="badge tpl clickable" @click="openDetail('template', u)">{{ templateById.get(u.container.template_id)?.name || u.container.template_id }}</span>
              <span v-else class="dim">-</span>
            </td>
            <td :data-label="t('thContainerStatus')">
              <span v-if="u.container" class="badge clickable" :class="u.container.status" @click="openDetail('container', u)">{{ u.container.status }}</span>
              <span v-else class="dim">-</span>
            </td>
            <td class="time" :data-label="t('thCreatedAt')">{{ fmtTime(u.created_at) }}</td>
          </tr>
        </tbody>
      </table>
      <p v-if="!visibleUsers.length" class="empty">{{ t('noUsersYet') }}</p>
    </div>

    <ConfirmDialog
      :visible="confirmVisible"
      :message="confirmMessage"
      type="danger"
      @confirm="onConfirmAction"
      @cancel="confirmVisible = false"
    />

    <div v-if="detailUser" class="modal-mask" @click.self="closeDetail">
      <div class="modal">
        <h3>{{ detailTitle }}</h3>

        <template v-if="detailMode === 'status'">
          <div class="kv">
            <template v-if="detailUser.role !== 'admin'">
              <span class="k">{{ t('enable') }}</span>
              <span class="v">
                <label class="switch" :class="{ on: statusDraft[detailUser.id] }">
                  <input type="checkbox" v-model="statusDraft[detailUser.id]" />
                  <span class="track"><span class="thumb"></span></span>
                </label>
              </span>
            </template>
            <template v-else>
              <span class="k">{{ t('thStatus') }}</span>
              <span class="v">{{ statusLabel(effStatus(detailUser)) }}</span>
            </template>
          </div>
          <template v-if="detailUser.role !== 'admin'">
            <div class="kv">
              <span class="k">{{ t('expiresAt') }}</span>
              <span class="v expiry-row">
                <input v-model="expiryDraft[detailUser.id]" type="datetime-local" class="expiry-in" />
                <button class="btn btn-sm" title="清除" @click="expiryDraft[detailUser.id] = ''">×</button>
              </span>
            </div>
          </template>
          <div v-else class="kv"><span class="k">{{ t('expiresAt') }}</span><span class="v">{{ detailUser.expires_at ? fmtTime(detailUser.expires_at) : t('longTerm') }}</span></div>
        </template>

        <template v-if="detailMode === 'template' && detailTpl">
          <div class="kv"><span class="k">{{ t('thName') }}</span><span class="v">{{ detailTpl.name }}</span></div>
          <div class="kv"><span class="k">{{ t('thImage') }}</span><span class="v"><code>{{ detailTpl.image }}</code></span></div>
          <div class="kv"><span class="k">{{ t('thPort') }}</span><span class="v"><code>{{ portStr(detailTpl.internal_port, detailTpl.extra_ports) }}</code></span></div>
          <div class="kv"><span class="k">{{ t('thCpu') }}</span><span class="v">{{ detailTpl.cpu_limit }}</span></div>
          <div class="kv"><span class="k">{{ t('thMem') }}</span><span class="v">{{ fmtMem(detailTpl.mem_limit) }}</span></div>
          <div class="kv"><span class="k">{{ t('workDir') }}</span><span class="v"><code>{{ detailTpl.workspace_dir }}</code></span></div>
          <div class="kv"><span class="k">{{ t('startCmd') }}</span><span class="v"><code>{{ (detailTpl.command || []).join(' ') || '-' }}</code></span></div>
          <div class="kv"><span class="k">{{ t('envVars') }}</span><span class="v"><code>{{ envStr(detailTpl.envs) }}</code></span></div>
        </template>

        <template v-if="detailMode === 'container'">
          <div class="kv"><span class="k">{{ t('thContainerName') }}</span><span class="v"><code>{{ detailUser.container.container_name }}</code></span></div>
          <div class="kv"><span class="k">{{ t('thStatus') }}</span><span class="v">{{ detailUser.container.status }}</span></div>
          <div class="kv"><span class="k">{{ t('thNetwork') }}</span><span class="v"><code :class="{ warn: detailUser.container.network !== networkName }">{{ detailUser.container.network || '-' }}</code></span></div>
          <div class="kv"><span class="k">{{ t('thPort') }}</span><span class="v"><code>{{ portStr(detailUser.container.internal_port, detailUser.container.extra_ports) }}</code></span></div>
          <div class="kv"><span class="k">{{ t('thContainerId') }}</span><span class="v"><code>{{ detailUser.container.container_id || '-' }}</code></span></div>
          <div class="kv"><span class="k">{{ t('startedAt') }}</span><span class="v">{{ fmtTime(detailUser.container.started_at) }}</span></div>
          <div class="kv"><span class="k">{{ t('uptime') }}</span><span class="v">{{ fmtUp(detailUser.container.started_at) }}</span></div>
          <div class="kv"><span class="k">{{ t('thCreatedAt') }}</span><span class="v">{{ fmtTime(detailUser.container.created_at) }}</span></div>
        </template>

        <div class="btns">
          <template v-if="detailMode === 'status' && detailUser.role !== 'admin'">
            <button class="btn" :disabled="busy" @click="saveDetail(detailUser)">{{ t('save') }}</button>
          </template>
          <button class="btn" @click="closeDetail">{{ t('close') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, inject } from 'vue'
import { api, downloadText } from '../../api'
import ConfirmDialog from '../../components/ConfirmDialog.vue'

const { t } = inject('i18n')
const notify = inject('notify')
const users = ref([])
const templates = ref([])
const containers = ref([])
const count = ref(1)
const course = ref('')
const courseFilter = ref('')
const tplId = ref('')
const busy = ref(false)
const busyAction = ref('')
const selected = ref(new Set())
const loading = ref(false)
const updatedAt = ref('')
const POLL_MS = 5000
let timer = null

const confirmVisible = ref(false)
const confirmMessage = ref('')
let confirmAction = null

const expiryDraft = ref({})
const statusDraft = ref({})
const detailUser = ref(null)
const detailMode = ref('')

const detailTitle = computed(() => {
  if (!detailUser.value) return ''
  if (detailMode.value === 'status') return `${t('thStatus')} · ${detailUser.value.username}`
  if (detailMode.value === 'template') return `${t('thTemplate')} · ${detailUser.value.username}`
  return `${t('thContainerStatus')} · ${detailUser.value.username}`
})

const detailTpl = computed(() => {
  if (!detailUser.value || detailMode.value !== 'template') return null
  return templateById.value.get(detailUser.value.container.template_id) || null
})

function openDetail(mode, u) {
  detailMode.value = mode
  detailUser.value = u
  if (mode === 'status') initDraft(u)
}

function closeDetail() {
  detailUser.value = null
}

const containerMap = computed(() => {
  const m = new Map()
  for (const c of containers.value) m.set(c.user_id, c)
  return m
})

const templateById = computed(() => {
  const m = new Map()
  for (const t of templates.value) m.set(t.id, t)
  return m
})

const networkName = computed(() => platformNetwork.value || (containers.value[0] && containers.value[0].expected_network) || '')
const platformNetwork = ref('')

const courses = computed(() => {
  const set = new Set(users.value.map((u) => u.course).filter(Boolean))
  return [...set].sort()
})

const visibleUsers = computed(() => {
  const rows = courseFilter.value ? users.value.filter((u) => u.course === courseFilter.value) : users.value
  return rows.map((u) => ({ ...u, container: containerMap.value.get(u.id) || null }))
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
  if (document.hidden) return
  loading.value = true
  try {
    await load()
    updatedAt.value = new Date().toLocaleTimeString(navigator.language || 'zh-CN', { hour12: false })
  } finally {
    loading.value = false
  }
}

async function load() {
  const [u, t, c] = await Promise.all([api.listUsers(), api.listTemplates(), api.listContainers()])
  users.value = u
  templates.value = t
  containers.value = c
  if (!platformNetwork.value) {
    try {
      const pf = await api.platform()
      platformNetwork.value = (pf && pf.network) || ''
    } catch { /* non-fatal */ }
  }
  if (templates.value.length && !tplId.value) tplId.value = templates.value[0].id
}

async function bulkAction(action) {
  if (!selected.value.size) return notify(t('selectFirst'), 'err')
  const names = [...selected.value].map((id) => users.value.find((u) => u.id === id)).filter(Boolean).map((u) => u.username)
  const actionLabel = { rebuild: t('rebuild'), start: t('start'), restart: t('restart'), stop: t('stop'), disable: t('disable'), enable: t('enable'), delete: t('delete') }[action]
  confirmMessage.value = t('actionConfirm', { n: names.length, action: actionLabel })
  confirmAction = action
  confirmVisible.value = true
}

// effStatus derives the account state the same way the backend does:
// expired (expires_at passed) > disabled (manual ban) > active.
function effStatus(u) {
  if (u.expires_at && new Date(u.expires_at).getTime() <= Date.now()) return 'expired'
  if (u.manual_disabled) return 'disabled'
  return 'active'
}

function statusLabel(s) {
  return { active: t('statusActive'), disabled: t('statusDisabled'), expired: t('statusExpired') }[s] || s
}

// saveDetail persists the status switch and the expiry in one request.
async function saveDetail(u) {
  const raw = expiryDraft.value[u.id]
  const payload = {}
  if (raw) {
    const ts = new Date(raw).getTime()
    if (Number.isNaN(ts)) return notify(t('invalidDate'), 'err')
    payload.expires_at = new Date(ts).toISOString()
  } else {
    payload.expires_in_days = 0
  }
  if (statusDraft.value[u.id] !== !u.manual_disabled) {
    payload.status = statusDraft.value[u.id] ? 'active' : 'disabled'
  }
  busy.value = true
  try {
    const updated = await api.updateUser(u.id, payload)
    const eff = updated ? effStatus(updated) : ''
    if (eff === 'expired') {
      notify(t('accountExpired'), 'err')
    } else if (updated && updated.manual_disabled) {
      notify(t('savedDisabled'))
    } else if (effStatus(u) !== 'active' && eff === 'active') {
      notify(t('accountReactivated'))
    } else {
      notify(t('expirySaved'))
    }
  } catch (e) {
    notify(e.message, 'err')
    busy.value = false
    return
  }
  busy.value = false
  closeDetail()
  await load()
}

function initDraft(u) {
  expiryDraft.value[u.id] = u.expires_at ? toLocalInput(u.expires_at) : ''
  statusDraft.value[u.id] = !u.manual_disabled
}

function toLocalInput(iso) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

async function onConfirmAction() {
  confirmVisible.value = false
  const action = confirmAction
  confirmAction = null
  if (!action) return
  busyAction.value = action
  try {
    if (action === 'rebuild') {
      const pc = await api.provisionBatch({ template_id: tplId.value || '', user_ids: [...selected.value], force: true })
      const fails = (pc.results || []).filter((x) => !x.ok)
      if (fails.length) {
        notify(t('rebuildPartial', { n: pc.provisioned, f: fails.length, err: fails[0].error }), 'err')
      } else {
        notify(t('rebuildSuccess', { n: pc.provisioned }))
      }
    } else {
      const results = await api.batchUserAction({ user_ids: [...selected.value], action })
      const okN = results.filter((r) => r.ok).length
      const actionLabel = { start: t('start'), restart: t('restart'), stop: t('stop'), disable: t('disable'), enable: t('enable'), delete: t('delete') }[action]
      notify(t('actionComplete', { action: actionLabel, ok: okN, fail: results.length - okN }))
      if (action === 'delete') selected.value = new Set()
    }
    await load()
  } catch (e) {
    notify(e.message, 'err')
  } finally {
    busyAction.value = ''
  }
}

async function batchCreate() {
  if (!tplId.value) return notify(t('selectTemplate'), 'err')
  if (!course.value.trim()) return notify(t('fillCourse'), 'err')
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
    if (!r.created) throw new Error(t('noAccountsCreated'))
    const ids = (r.users || []).map((u) => u.id)
    const pc = await api.provisionBatch({ template_id: tplId.value, user_ids: ids, force: false })
    const csv = r.accounts.map((a) => `${a.username},${a.password}`).join('\n')
    downloadText('accounts.csv', 'username,password\n' + csv, 'text/csv')
    const fails = (pc.results || []).filter((x) => !x.ok)
    if (fails.length) {
      notify(t('createPartial', { n: r.created, f: fails.length, err: fails[0].error }), 'err')
    } else {
      notify(t('createSuccess', { n: r.created, p: pc.provisioned }))
    }
    await load()
  } catch (e) {
    notify(e.message, 'err')
  } finally {
    busy.value = false
  }
}

function portStr(port, extra) {
  return `${port || '-'}${(extra || []).length ? ' / ' + extra.join(',') : ''}`
}

function envStr(envs) {
  if (!envs || !Object.keys(envs).length) return '-'
  return Object.entries(envs).map(([k, v]) => `${k}=${v}`).join('\n')
}

function fmtMem(bytes) {
  if (!bytes) return '-'
  const gb = bytes / (1 << 30)
  return gb >= 1 ? gb.toFixed(2) + ' GB' : Math.round(bytes / (1 << 20)) + ' MB'
}

function fmtTime(ts) {
  if (!ts) return '-'
  return new Date(ts).toLocaleString(navigator.language || 'zh-CN')
}

function fmtUp(ts) {
  if (!ts) return '-'
  const start = new Date(ts).getTime()
  if (Number.isNaN(start) || start > Date.now()) return fmtTime(ts)
  const s = Math.max(0, Math.floor((Date.now() - start) / 1000))
  if (s < 60) return s + 's'
  const m = Math.floor(s / 60)
  if (m < 60) return m + 'm'
  const h = Math.floor(m / 60)
  if (h < 24) return h + 'h' + (m % 60) + 'm'
  return Math.floor(h / 24) + 'd' + (h % 24) + 'h'
}
</script>

<style scoped>
.head-left { display: flex; align-items: baseline; gap: 12px; }
.head-right { display: flex; align-items: center; gap: 14px; }
.updated { font-size: 12px; color: var(--text-2); font-family: var(--font-mono); }
.meta { color: var(--text-2); font-size: 12.5px; }
.batch { margin-bottom: 18px; }
h3 { margin: 0 0 14px; font-size: 14.5px; color: var(--text-0); letter-spacing: 0.03em; }
.row { display: flex; gap: 14px; align-items: flex-end; flex-wrap: wrap; }
.row > div { flex: 1; min-width: 140px; }
.row .span2 { flex: 2; min-width: 240px; }
.row-btns { display: flex; gap: 8px; flex: 2 !important; }
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
.course-txt { color: var(--violet); font-weight: 500; }
.pwd { font-family: var(--font-mono); font-size: 12px; }
.badge.tpl { color: var(--cyan); border-color: rgba(34, 211, 238, 0.4); background: rgba(34, 211, 238, 0.08); }
.badge.clickable { cursor: pointer; transition: transform 0.15s ease, box-shadow 0.15s ease; }
.badge.clickable:hover { transform: translateY(-1px); box-shadow: 0 0 14px rgba(34, 211, 238, 0.25); }
.kv { display: flex; gap: 12px; padding: 7px 0; font-size: 13px; line-height: 1.7; }
.kv .k { color: var(--text-2); min-width: 72px; flex-shrink: 0; }
.kv .v { color: var(--text-0); word-break: break-all; white-space: pre-line; }
.kv .v code { font-family: var(--font-mono); font-size: 12px; color: var(--cyan); white-space: pre-wrap; word-break: break-all; }
.kv .v code.warn { color: var(--err); }
.expiry-row { display: flex; align-items: center; gap: 6px; }
.switch { display: inline-flex; align-items: center; gap: 8px; cursor: pointer; user-select: none; }
.switch input { display: none; }
.switch .track {
  width: 40px; height: 22px; border-radius: 999px;
  background: rgba(255, 255, 255, 0.12);
  border: 1px solid var(--glass-border);
  position: relative; transition: background 0.2s ease;
}
.switch .thumb {
  position: absolute; top: 2px; left: 2px;
  width: 16px; height: 16px; border-radius: 50%;
  background: var(--text-1); transition: transform 0.2s ease;
}
.switch.on .track { background: var(--ok); border-color: rgba(52, 211, 153, 0.4); }
.switch.on .thumb { transform: translateX(18px); background: #fff; }
.time { color: var(--text-2); font-size: 12px; white-space: nowrap; }
.expiry-in {
  font-family: var(--font-mono);
  font-size: 12px;
  color-scheme: dark;
  background: var(--bg-2);
  border: 1px solid var(--glass-border);
  border-radius: 6px;
  padding: 4px 6px;
  color: var(--text-0);
}
.dim { color: var(--text-2); }
.net-hint { margin-left: 14px; color: var(--text-2); }
.net-hint code { font-family: var(--font-mono); font-size: 12px; color: var(--cyan); }
.table-card {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}
table { min-width: 760px; }
@media (max-width: 720px) {
  table { min-width: 0; }
  thead { display: none; }
  tbody, tr, td { display: block; width: 100%; }
  tr {
    margin: 0 0 12px;
    padding: 10px 14px;
    border: 1px solid var(--glass-border);
    border-radius: var(--radius-md);
    background: rgba(255, 255, 255, 0.03);
  }
  td {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 7px 0;
    border-bottom: 1px dashed rgba(255, 255, 255, 0.06);
    text-align: right;
  }
  td:last-child { border-bottom: none; }
  td::before {
    content: attr(data-label);
    color: var(--text-2);
    font-size: 11.5px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    flex-shrink: 0;
  }
  td.chk { justify-content: flex-start; }
  td.chk::before { display: none; }
  td code { word-break: break-all; }
  .row > div, .row .span2 { min-width: 100%; }
  .toolbar { flex-direction: column; align-items: stretch; }
  .course-filter { width: 100%; }
  .toolbar-btns { width: 100%; }
  .toolbar-btns .btn { flex: 1 1 auto; }
}
</style>
