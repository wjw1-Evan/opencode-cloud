<template>
  <div>
    <div class="page-head">
      <h2>{{ t('templatesTitle') }}</h2>
      <div class="head-right">
        <span class="updated" v-if="updatedAt">{{ t('updatedAt') }} {{ updatedAt }}</span>
        <button class="btn btn-primary" @click="openCreate">{{ t('newTemplate') }}</button>
      </div>
    </div>

    <div class="card">
      <table>
        <thead>
          <tr><th>{{ t('thName') }}</th><th>{{ t('thImage') }}</th><th>{{ t('thPort') }}</th><th>{{ t('thCpu') }}</th><th>{{ t('thMem') }}</th><th>{{ t('thActions') }}</th></tr>
        </thead>
        <tbody>
          <tr v-for="tpl in templates" :key="tpl.id">
            <td :data-label="t('thName')">{{ tpl.name }}<span v-if="tpl.is_system" class="badge system">{{ t('system') }}</span></td>
            <td :data-label="t('thImage')"><code>{{ tpl.image }}</code></td>
            <td :data-label="t('thPort')">{{ tpl.internal_port }}{{ (tpl.extra_ports || []).length ? ' / ' + (tpl.extra_ports || []).join(',') : '' }}</td>
            <td :data-label="t('thCpu')">{{ tpl.cpu_limit }}</td>
            <td :data-label="t('thMem')">{{ fmtBytes(tpl.mem_limit) }}</td>
            <td class="ops" :data-label="t('thActions')">
              <button class="btn" @click="openEdit(tpl)">{{ t('edit') }}</button>
              <button class="btn btn-danger" :disabled="tpl.is_system" @click="del(tpl)">{{ t('delete') }}</button>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="!templates.length" class="empty">{{ t('noTemplates') }}</p>
    </div>

    <div v-if="showModal" class="modal-mask" @click.self="close">
      <div class="modal modal-lg">
        <h3>{{ editing ? t('editTemplate') : t('newTemplateModal') }}</h3>
        <div class="grid">
          <div><label>{{ t('nameLabel') }}</label><input v-model="f.name" :placeholder="t('namePlaceholder')" /></div>
          <div class="span2"><label>{{ t('imageLabel') }}</label><input v-model="f.image" :placeholder="t('imagePlaceholder')" /></div>
          <div><label>{{ t('mainPort') }}</label><input v-model.number="f.internal_port" type="number" /></div>
          <div><label>{{ t('extraPorts') }}</label><input v-model="extraPortsText" :placeholder="t('extraPortsPlaceholder')" /></div>
          <div><label>{{ t('cpuLimit') }}</label><input v-model.number="f.cpu_limit" type="number" step="0.1" /></div>
          <div><label>{{ t('memLimit') }}</label><input v-model.number="memGb" type="number" step="0.5" /></div>
          <div class="span2"><label>{{ t('workDir') }}</label><input v-model="f.workspace_dir" /></div>
        </div>
        <div class="hint" style="margin-top: 18px">
          {{ t('extraPortsHint') }}
        </div>
        <div style="margin-top: 18px">
          <label>{{ t('startCmd') }}</label>
          <input v-model="cmdText" :placeholder="t('startCmdPlaceholder')" />
        </div>
        <div style="margin-top: 18px">
          <label>{{ t('envVars') }}</label>
          <textarea v-model="envsText" rows="4" :placeholder="t('envVarsPlaceholder')"></textarea>
        </div>
        <div class="btns">
          <button class="btn btn-primary" @click="save" :disabled="busy">{{ editing ? t('save') : t('create') }}</button>
          <button class="btn" @click="close">{{ t('cancel') }}</button>
        </div>
      </div>
    </div>

    <ConfirmDialog
      :visible="confirmVisible"
      :message="confirmMessage"
      type="danger"
      @confirm="onConfirmDelete"
      @cancel="confirmVisible = false"
    />
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, inject } from 'vue'
import { api, fmtBytes } from '../../api'
import ConfirmDialog from '../../components/ConfirmDialog.vue'

const { t } = inject('i18n')
const notify = inject('notify')
const templates = ref([])
const busy = ref(false)
const editing = ref(null)
const showModal = ref(false)
const f = ref(blank())
const memGb = ref(1)
const envsText = ref('')
const cmdText = ref('')
const extraPortsText = ref('')
const updatedAt = ref('')
const POLL_MS = 5000
let timer = null

const confirmVisible = ref(false)
const confirmMessage = ref('')
let deleteTarget = null

function blank() {
  return { name: '', image: '', internal_port: 4096, extra_ports: [], cpu_limit: 0.5, mem_limit: 1 << 30, workspace_dir: '/workspace' }
}
function resetForm() {
  editing.value = null
  f.value = blank()
  memGb.value = 1
  envsText.value = ''
  cmdText.value = ''
  extraPortsText.value = ''
}

function openCreate() {
  resetForm()
  showModal.value = true
}

function openEdit(tpl) {
  editing.value = tpl
  f.value = { ...tpl }
  memGb.value = tpl.mem_limit / (1 << 30)
  envsText.value = Object.entries(tpl.envs || {}).map(([k, v]) => `${k}=${v}`).join('\n')
  cmdText.value = (tpl.command || []).join(' ')
  extraPortsText.value = (tpl.extra_ports || []).join(',')
  showModal.value = true
}

function close() {
  showModal.value = false
  resetForm()
}

onMounted(() => {
  refresh()
  timer = setInterval(refresh, POLL_MS)
})
onUnmounted(() => clearInterval(timer))

async function refresh() {
  if (document.hidden) return
  try {
    templates.value = await api.listTemplates()
    updatedAt.value = new Date().toLocaleTimeString(navigator.language || 'zh-CN', { hour12: false })
  } catch (e) {
    notify(e.message, 'err')
  }
}

async function save() {
  if (!f.value.name || !f.value.image) return notify(t('nameAndImageRequired'), 'err')
  busy.value = true
  const payload = {
    name: f.value.name,
    image: f.value.image,
    internal_port: f.value.internal_port,
    extra_ports: extraPortsText.value.split(',').map((s) => parseInt(s.trim(), 10)).filter((n) => Number.isInteger(n) && n > 0),
    cpu_limit: f.value.cpu_limit,
    mem_limit: Math.round(memGb.value * (1 << 30)),
    workspace_dir: f.value.workspace_dir,
    envs: Object.fromEntries(
      envsText.value.split('\n').map((l) => l.trim()).filter(Boolean).map((l) => l.split(/=(.*)/s).slice(0, 2))
    ),
    command: cmdText.value.split(/\s+/).filter(Boolean),
  }
  try {
    if (editing.value) {
      await api.updateTemplate(editing.value.id, payload)
      notify(t('saved'))
    } else {
      await api.createTemplate(payload)
      notify(t('created'))
    }
    templates.value = await api.listTemplates()
    close()
  } catch (e) {
    notify(e.message, 'err')
  } finally {
    busy.value = false
  }
}

function del(tpl) {
  confirmMessage.value = t('deleteConfirm', { name: tpl.name })
  deleteTarget = tpl
  confirmVisible.value = true
}

async function onConfirmDelete() {
  confirmVisible.value = false
  const tpl = deleteTarget
  deleteTarget = null
  if (!tpl) return
  try {
    await api.deleteTemplate(tpl.id)
    notify(t('deleted'))
    templates.value = await api.listTemplates()
  } catch (e) {
    notify(e.message, 'err')
  }
}
</script>

<style scoped>
.head-right { display: flex; align-items: center; gap: 14px; }
.updated { font-size: 12px; color: var(--text-2); font-family: var(--font-mono); }
.grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 14px 18px; }
.grid .span2 { grid-column: span 2; }
.grid > div { min-width: 0; }
.grid label { margin-bottom: 7px; }
.card {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}
table { min-width: 640px; }
.badge.system {
  margin-left: 8px;
  color: var(--cyan);
  border-color: rgba(34, 211, 238, 0.4);
  background: rgba(34, 211, 238, 0.08);
  font-size: 10.5px;
}
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
  td code { word-break: break-all; }
  .ops { justify-content: flex-end; }
  .grid { grid-template-columns: 1fr; }
  .grid .span2 { grid-column: span 1; }
}
</style>
