<template>
  <div>
    <div class="page-head">
      <h2>{{ t('imagesTitle') }}</h2>
      <div class="head-btns">
        <span class="updated" v-if="updatedAt">{{ t('updatedAt') }} {{ updatedAt }}</span>
        <button class="btn" @click="showImport = true">{{ t('uploadImage') }}</button>
        <button class="btn btn-primary" @click="showPull = true">{{ t('pullImage') }}</button>
      </div>
    </div>

    <div class="card">
      <table>
        <thead>
          <tr><th>{{ t('thName') }}</th><th>{{ t('thImageId') }}</th><th>{{ t('thSize') }}</th><th>{{ t('thUsage') }}</th><th>{{ t('thCreatedAt') }}</th><th>{{ t('thActions') }}</th></tr>
        </thead>
        <tbody>
          <tr v-for="im in images" :key="im.id">
            <td :data-label="t('thName')">
              <span v-if="im.repo_tags && im.repo_tags.length && im.repo_tags[0] !== '<none>:<none>'">
                {{ im.repo_tags[0] }}
              </span>
              <span v-else class="none-tag">&lt;none&gt;</span>
            </td>
            <td :data-label="t('thImageId')"><code>{{ shortId(im.id) }}</code></td>
            <td :data-label="t('thSize')">{{ fmtBytes(im.size) }}</td>
            <td :data-label="t('thUsage')">
              <span class="badge" :class="im.in_use ? 'inuse' : 'free'" :title="inUseTitle(im)">
                {{ im.in_use ? t('inUse') : t('notInUse') }}
              </span>
            </td>
            <td :data-label="t('thCreatedAt')">{{ fmtTime(im.created) }}</td>
            <td class="ops" :data-label="t('thActions')">
              <button class="btn" @click="inspect(im)">{{ t('detail') }}</button>
              <button class="btn btn-danger" :disabled="im.in_use" :title="im.in_use ? t('inUseTip') : ''" @click="del(im)">{{ t('delete') }}</button>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="!images.length && !loading" class="empty">{{ t('noImages') }}</p>
      <p v-if="!images.length && loading" class="empty">{{ t('loading') }}</p>
    </div>

    <div v-if="showImport" class="modal-mask" @click.self="showImport = false">
      <div class="modal">
        <h3>{{ t('uploadModalTitle') }}</h3>
        <p class="hint" v-html="t('uploadHint')"></p>
        <div style="margin-top: 14px">
          <input ref="fileInput" type="file" accept=".tar,.tar.gz" @change="onFileChange" style="display:none" />
          <button class="btn" @click="$refs.fileInput.click()">
            {{ uploadFile ? uploadFile.name : t('chooseFile') }}
          </button>
        </div>
        <div v-if="uploading" class="progress-bar">
          <div class="progress-fill" :style="{ width: '100%' }"></div>
          <span>{{ t('loadingTip') }}</span>
        </div>
        <div class="btns">
          <button class="btn btn-primary" @click="doImport" :disabled="!uploadFile || uploading">{{ t('importBtn') }}</button>
          <button class="btn" @click="showImport = false" :disabled="uploading">{{ t('cancel') }}</button>
        </div>
      </div>
    </div>

    <div v-if="showPull" class="modal-mask" @click.self="showPull = false">
      <div class="modal">
        <h3>{{ t('pullModalTitle') }}</h3>
        <p class="hint" v-html="t('pullHint')"></p>
        <div style="margin-top: 14px">
          <input v-model="pullName" :placeholder="t('pullPlaceholder')" @keyup.enter="doPull" />
        </div>
        <div v-if="pulling" class="progress-bar">
          <div class="progress-fill" :style="{ width: '100%' }"></div>
          <span>{{ t('loadingPull') }}</span>
        </div>
        <div class="btns">
          <button class="btn btn-primary" @click="doPull" :disabled="!pullName || pulling">{{ t('pull') }}</button>
          <button class="btn" @click="showPull = false" :disabled="pulling">{{ t('cancel') }}</button>
        </div>
      </div>
    </div>

    <div v-if="showDetail" class="modal-mask" @click.self="showDetail = false">
      <div class="modal modal-lg detail-modal">
        <h3>{{ t('detailModalTitle') }}</h3>
        <div v-if="detail">
          <div class="detail-head">
            <span class="badge" :class="detail.in_use ? 'inuse' : 'free'">{{ detail.in_use ? t('inUse') : t('notInUse') }}</span>
            <span class="detail-head-name">{{ detail.repo_tags && detail.repo_tags[0] ? detail.repo_tags[0] : shortId(detail.id) }}</span>
          </div>

          <div class="detail-section">
            <div class="section-title">{{ t('basicInfo') }}</div>
            <div class="detail-grid">
              <div class="detail-row"><span class="label">ID</span><code>{{ shortId(detail.id) }}</code></div>
              <div class="detail-row"><span class="label">{{ t('tags') }}</span>
                <span v-for="tag in detail.repo_tags" :key="tag" class="tag-badge">{{ tag }}</span>
                <span v-if="!detail.repo_tags || !detail.repo_tags.length">-</span>
              </div>
              <div class="detail-row full"><span class="label">{{ t('repoDigests') }}</span>
                <div v-if="detail.repo_digests && detail.repo_digests.length" class="digest-list">
                  <code v-for="d in detail.repo_digests" :key="d">{{ d }}</code>
                </div>
                <span v-else>-</span>
              </div>
              <div class="detail-row"><span class="label">{{ t('architecture') }}</span>{{ detail.architecture }}{{ detail.variant ? ' / ' + detail.variant : '' }}</div>
              <div class="detail-row"><span class="label">{{ t('os') }}</span>{{ detail.os }}</div>
              <div class="detail-row"><span class="label">{{ t('thCreatedAt') }}</span>{{ fmtCreated(detail.created) }}</div>
              <div class="detail-row"><span class="label">{{ t('thSize') }}</span>{{ fmtBytes(detail.size) }}</div>
              <div class="detail-row"><span class="label">{{ t('author') }}</span>{{ detail.author || '-' }}</div>
            </div>
          </div>

          <div class="detail-section">
            <div class="section-title">{{ t('runConfig') }}</div>
            <div class="detail-grid">
              <div class="detail-row"><span class="label">{{ t('runUser') }}</span>{{ detail.user || '-' }}</div>
              <div class="detail-row"><span class="label">{{ t('workDir') }}</span>{{ detail.working_dir || '-' }}</div>
              <div class="detail-row"><span class="label">{{ t('exposedPorts') }}</span>
                <span v-for="p in detail.exposed_ports || []" :key="p" class="tag-badge">{{ p }}</span>
                <span v-if="!detail.exposed_ports || !detail.exposed_ports.length">-</span>
              </div>
              <div class="detail-row"><span class="label">{{ t('volumes') }}</span>
                <span v-for="v in detail.volumes || []" :key="v" class="tag-badge">{{ v }}</span>
                <span v-if="!detail.volumes || !detail.volumes.length">-</span>
              </div>
              <div class="detail-row"><span class="label">{{ t('stopSignal') }}</span>{{ detail.stop_signal || '-' }}</div>
              <div class="detail-row full"><span class="label">{{ t('healthcheck') }}</span>{{ detail.healthcheck || '-' }}</div>
            </div>
          </div>

          <div class="detail-section" v-if="(detail.entrypoint && detail.entrypoint.length) || (detail.cmd && detail.cmd.length)">
            <div class="section-title">{{ t('startConfig') }}</div>
            <div class="detail-grid">
              <div class="detail-row full" v-if="detail.entrypoint && detail.entrypoint.length">
                <span class="label">Entrypoint</span><code>{{ detail.entrypoint.join(' ') }}</code>
              </div>
              <div class="detail-row full" v-if="detail.cmd && detail.cmd.length">
                <span class="label">CMD</span><code>{{ detail.cmd.join(' ') }}</code>
              </div>
            </div>
          </div>

          <div class="detail-section" v-if="detail.env && detail.env.length">
            <div class="section-title">{{ t('envVars') }}</div>
            <div class="env-list">
              <div v-for="e in detail.env" :key="e" class="env-item"><code>{{ e }}</code></div>
            </div>
          </div>

          <div class="detail-section" v-if="labelItems.length">
            <div class="section-title">{{ t('labels') }}</div>
            <div class="label-list">
              <div v-for="item in labelItems" :key="item.k" class="label-item">
                <code class="label-key">{{ item.k }}</code>
                <code class="label-val">{{ item.v }}</code>
              </div>
            </div>
          </div>

          <div class="detail-section">
            <div class="section-title">{{ t('layers') }}</div>
            <span class="layers-count">{{ detail.layers ? detail.layers.length : 0 }} {{ t('layerUnit') }}</span>
          </div>
        </div>
        <div class="btns">
          <button class="btn" @click="showDetail = false">{{ t('close') }}</button>
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
import { ref, computed, onMounted, onUnmounted, inject } from 'vue'
import { api, fmtBytes } from '../../api'
import ConfirmDialog from '../../components/ConfirmDialog.vue'

const { t } = inject('i18n')
const notify = inject('notify')
const images = ref([])
const loading = ref(false)
const showImport = ref(false)
const showPull = ref(false)
const showDetail = ref(false)
const uploadFile = ref(null)
const uploading = ref(false)
const pullName = ref('')
const pulling = ref(false)
const detail = ref(null)
const updatedAt = ref('')
const POLL_MS = 5000
let timer = null

const confirmVisible = ref(false)
const confirmMessage = ref('')
let deleteTarget = null

function shortId(id) {
  return id ? id.replace('sha256:', '').substring(0, 12) : ''
}

const labelItems = computed(() => {
  if (!detail.value || !detail.value.labels) return []
  return Object.entries(detail.value.labels).map(([k, v]) => ({ k, v }))
})

function fmtCreated(ts) {
  if (!ts) return '-'
  const d = new Date(ts)
  if (isNaN(d.getTime())) return '-'
  return d.toLocaleString(navigator.language || 'zh-CN', { hour12: false })
}

function inUseTitle(im) {
  if (!im.in_use) return t('notInUse')
  const by = (im.used_by || []).join(', ')
  return by ? t('inUseBy', { list: by }) : t('inUse')
}

function fmtTime(ts) {
  if (!ts) return '-'
  const d = new Date(ts * 1000)
  const now = Date.now()
  const diff = now - d.getTime()
  if (diff < 60000) return t('justNow')
  if (diff < 3600000) return t('minutesAgo', { n: Math.floor(diff / 60000) })
  if (diff < 86400000) return t('hoursAgo', { n: Math.floor(diff / 3600000) })
  return t('daysAgo', { n: Math.floor(diff / 86400000) })
}

async function load() {
  loading.value = true
  try {
    images.value = await api.listImages()
  } catch (e) {
    notify(e.message, 'err')
  } finally {
    loading.value = false
  }
}

async function refresh() {
  if (loading.value || uploading.value || pulling.value) return
  if (document.hidden) return
  loading.value = true
  try {
    images.value = await api.listImages()
    updatedAt.value = new Date().toLocaleTimeString(navigator.language || 'zh-CN', { hour12: false })
  } catch (e) {
    notify(e.message, 'err')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  refresh()
  timer = setInterval(refresh, POLL_MS)
})
onUnmounted(() => clearInterval(timer))

function onFileChange(e) {
  uploadFile.value = e.target.files[0] || null
}

async function doImport() {
  if (!uploadFile.value) return
  uploading.value = true
  try {
    await api.uploadImage(uploadFile.value)
    notify(t('imageImported'))
    showImport.value = false
    uploadFile.value = null
    await load()
  } catch (e) {
    notify(e.message, 'err')
  } finally {
    uploading.value = false
  }
}

async function doPull() {
  if (!pullName.value) return
  pulling.value = true
  try {
    await api.pullImage(pullName.value)
    notify(t('imagePulled'))
    showPull.value = false
    pullName.value = ''
    await load()
  } catch (e) {
    notify(e.message, 'err')
  } finally {
    pulling.value = false
  }
}

async function inspect(im) {
  try {
    detail.value = await api.getImage(im.id)
    showDetail.value = true
  } catch (e) {
    notify(e.message, 'err')
  }
}

async function del(im) {
  if (im.in_use) {
    notify(t('inUseTip'), 'err')
    return
  }
  const name = (im.repo_tags && im.repo_tags[0]) || shortId(im.id)
  confirmMessage.value = t('deleteImageConfirm', { name })
  deleteTarget = im
  confirmVisible.value = true
}

async function onConfirmDelete() {
  confirmVisible.value = false
  const im = deleteTarget
  deleteTarget = null
  if (!im) return
  try {
    await api.deleteImage(im.id)
    notify(t('deleted'))
    await load()
  } catch (e) {
    notify(e.message, 'err')
  }
}
</script>

<style scoped>
.head-btns { display: flex; gap: 8px; align-items: center; }
.card {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}
table { min-width: 640px; }
.updated { font-size: 12px; color: var(--text-2); font-family: var(--font-mono); }
.none-tag { color: var(--text-2); font-style: italic; }
.badge.inuse { color: var(--ok); border-color: rgba(52, 211, 153, 0.35); background: rgba(52, 211, 153, 0.08); }
.badge.free { color: var(--text-2); }
.hint code {
  background: rgba(255,255,255,0.06);
  padding: 1px 5px;
  border-radius: 4px;
  font-size: 11.5px;
}
.detail-grid { display: flex; flex-direction: column; gap: 10px; }
.detail-modal { width: 680px; }
.detail-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
}
.detail-head-name {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--text-0);
  word-break: break-all;
}
.detail-section { margin-bottom: 18px; }
.section-title {
  font-size: 11.5px;
  font-weight: 600;
  letter-spacing: 0.09em;
  text-transform: uppercase;
  color: var(--text-2);
  margin-bottom: 10px;
  padding-bottom: 6px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.07);
}
.detail-row {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
}
.detail-row.full { flex-direction: column; align-items: flex-start; gap: 4px; }
.detail-row .label {
  min-width: 60px;
  color: var(--text-2);
  font-size: 12px;
  flex-shrink: 0;
}
.detail-row code {
  background: rgba(255,255,255,0.06);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 12px;
  word-break: break-all;
}
.detail-row.full > code { width: 100%; word-break: break-all; }
.digest-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  width: 100%;
}
.digest-list code { word-break: break-all; }
.tag-badge {
  display: inline-block;
  padding: 2px 8px;
  margin: 2px 4px 2px 0;
  background: rgba(34, 211, 238, 0.1);
  border: 1px solid rgba(34, 211, 238, 0.25);
  border-radius: 6px;
  font-size: 12px;
  color: var(--cyan);
}
.env-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 200px;
  overflow-y: auto;
}
.env-item code {
  background: rgba(255,255,255,0.04);
  padding: 3px 8px;
  border-radius: 4px;
  font-size: 11.5px;
  display: block;
  word-break: break-all;
}
.label-list {
  display: flex;
  flex-direction: column;
  gap: 5px;
  max-height: 180px;
  overflow-y: auto;
}
.label-item {
  display: flex;
  gap: 10px;
  align-items: baseline;
  font-size: 12px;
}
.label-key { color: var(--violet); flex-shrink: 0; }
.label-val { color: var(--text-1); word-break: break-all; }
.layers-count { color: var(--text-1); font-size: 13px; }
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
  .detail-row { flex-wrap: wrap; }
  .detail-modal { width: auto; }
}
</style>
