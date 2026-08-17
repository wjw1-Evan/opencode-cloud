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
          <tr><th>{{ t('thName') }}</th><th>{{ t('thImageId') }}</th><th>{{ t('thSize') }}</th><th>{{ t('thCreatedAt') }}</th><th>{{ t('thActions') }}</th></tr>
        </thead>
        <tbody>
          <tr v-for="im in images" :key="im.id">
            <td>
              <span v-if="im.repo_tags && im.repo_tags.length && im.repo_tags[0] !== '<none>:<none>'">
                {{ im.repo_tags[0] }}
              </span>
              <span v-else class="none-tag">&lt;none&gt;</span>
            </td>
            <td><code>{{ shortId(im.id) }}</code></td>
            <td>{{ fmtBytes(im.size) }}</td>
            <td>{{ fmtTime(im.created) }}</td>
            <td class="ops">
              <button class="btn" @click="inspect(im)">{{ t('edit') }}</button>
              <button class="btn btn-danger" @click="del(im)">{{ t('delete') }}</button>
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
      <div class="modal modal-lg">
        <h3>{{ t('detailModalTitle') }}</h3>
        <div v-if="detail" class="detail-grid">
          <div class="detail-row"><span class="label">ID</span><code>{{ detail.id }}</code></div>
          <div class="detail-row"><span class="label">{{ t('architecture') }}</span>{{ detail.architecture }}</div>
          <div class="detail-row"><span class="label">{{ t('os') }}</span>{{ detail.os }}</div>
          <div class="detail-row"><span class="label">{{ t('thSize') }}</span>{{ fmtBytes(detail.size) }}</div>
          <div class="detail-row"><span class="label">{{ t('author') }}</span>{{ detail.author || '-' }}</div>
          <div class="detail-row"><span class="label">{{ t('workDir') }}</span>{{ detail.working_dir || '-' }}</div>
          <div class="detail-row full"><span class="label">{{ t('tags') }}</span>
            <span v-for="tag in detail.repo_tags" :key="tag" class="tag-badge">{{ tag }}</span>
            <span v-if="!detail.repo_tags || !detail.repo_tags.length">-</span>
          </div>
          <div class="detail-row full" v-if="detail.cmd && detail.cmd.length">
            <span class="label">CMD</span><code>{{ detail.cmd.join(' ') }}</code>
          </div>
          <div class="detail-row full" v-if="detail.entrypoint && detail.entrypoint.length">
            <span class="label">Entrypoint</span><code>{{ detail.entrypoint.join(' ') }}</code>
          </div>
          <div class="detail-row full" v-if="detail.env && detail.env.length">
            <span class="label">{{ t('envVars') }}</span>
            <div class="env-list">
              <div v-for="e in detail.env" :key="e" class="env-item"><code>{{ e }}</code></div>
            </div>
          </div>
          <div class="detail-row full" v-if="detail.layers && detail.layers.length">
            <span class="label">{{ t('layers') }}</span>{{ detail.layers.length }} {{ t('layerUnit') }}
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
import { ref, onMounted, onUnmounted, inject } from 'vue'
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
const POLL_MS = 1000
let timer = null

const confirmVisible = ref(false)
const confirmMessage = ref('')
let deleteTarget = null

function shortId(id) {
  return id ? id.replace('sha256:', '').substring(0, 12) : ''
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
  loading.value = true
  try {
    images.value = await api.listImages()
    updatedAt.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
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
.updated { font-size: 12px; color: var(--text-2); font-family: var(--font-mono); }
.none-tag { color: var(--text-2); font-style: italic; }
.hint code {
  background: rgba(255,255,255,0.06);
  padding: 1px 5px;
  border-radius: 4px;
  font-size: 11.5px;
}
.detail-grid { display: flex; flex-direction: column; gap: 10px; }
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
</style>
