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
      <p class="hint">{{ t('batchHint') }}</p>
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
          <button class="btn btn-danger" :disabled="!selected.size || !!busyAction" @click="bulkAction('delete')">{{ busyAction === 'delete' ? t('deleting') : t('delete') }}</button>
        </div>
      </div>
      <table>
        <thead>
          <tr>
            <th class="chk"><input type="checkbox" :checked="allChecked" @change="toggleAll" /></th>
            <th>{{ t('thUsername') }}</th><th>{{ t('thPassword') }}</th><th>{{ t('thCourse') }}</th><th>{{ t('thStatus') }}</th>
            <th>{{ t('thContainerName') }}</th><th>{{ t('thContainerStatus') }}</th><th>{{ t('thPort') }}</th><th>{{ t('thCreatedAt') }}</th>
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
              <code v-if="u.container">{{ u.container.container_name }}</code>
              <span v-else class="dim">-</span>
            </td>
            <td>
              <span v-if="u.container" class="badge" :class="u.container.status">{{ u.container.status }}</span>
              <span v-else class="dim">-</span>
            </td>
            <td>
              <span v-if="u.container" class="ports">
                {{ u.container.internal_port }}{{ (u.container.extra_ports || []).length ? ' / ' + u.container.extra_ports.join(',') : '' }}
              </span>
              <span v-else class="dim">-</span>
            </td>
            <td class="time">{{ fmtTime(u.created_at) }}</td>
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
const POLL_MS = 1000
let timer = null

const confirmVisible = ref(false)
const confirmMessage = ref('')
let confirmAction = null

const containerMap = computed(() => {
  const m = new Map()
  for (const c of containers.value) m.set(c.user_id, c)
  return m
})

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
  loading.value = true
  try {
    await load()
    updatedAt.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
  } finally {
    loading.value = false
  }
}

async function load() {
  const [u, t, c] = await Promise.all([api.listUsers(), api.listTemplates(), api.listContainers()])
  users.value = u
  templates.value = t
  containers.value = c
  if (templates.value.length && !tplId.value) tplId.value = templates.value[0].id
}

async function bulkAction(action) {
  if (!selected.value.size) return notify(t('selectFirst'), 'err')
  const names = [...selected.value].map((id) => users.value.find((u) => u.id === id)).filter(Boolean).map((u) => u.username)
  const actionLabel = { rebuild: t('rebuild'), start: t('start'), restart: t('restart'), stop: t('stop'), delete: t('delete') }[action]
  confirmMessage.value = t('actionConfirm', { n: names.length, action: actionLabel })
  confirmAction = action
  confirmVisible.value = true
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
      const actionLabel = { start: t('start'), restart: t('restart'), stop: t('stop'), delete: t('delete') }[action]
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

function fmtTime(ts) {
  return new Date(ts).toLocaleString('zh-CN')
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
.pwd { font-family: var(--font-mono); font-size: 12px; }
.ports { font-family: var(--font-mono); font-size: 12px; color: var(--text-1); }
.time { color: var(--text-2); font-size: 12px; white-space: nowrap; }
.dim { color: var(--text-2); }
</style>
