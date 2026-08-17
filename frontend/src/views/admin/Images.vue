<template>
  <div>
    <div class="page-head">
      <h2>镜像管理</h2>
      <div class="head-btns">
        <button class="btn" @click="showImport = true">上传镜像</button>
        <button class="btn btn-primary" @click="showPull = true">拉取镜像</button>
      </div>
    </div>

    <div class="card">
      <table>
        <thead>
          <tr><th>名称</th><th>镜像 ID</th><th>大小</th><th>创建时间</th><th>操作</th></tr>
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
              <button class="btn" @click="inspect(im)">详情</button>
              <button class="btn btn-danger" @click="del(im)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="!images.length && !loading" class="empty">暂无镜像</p>
      <p v-if="loading" class="empty">加载中...</p>
    </div>

    <div v-if="showImport" class="modal-mask" @click.self="showImport = false">
      <div class="modal">
        <h3>上传镜像</h3>
        <p class="hint">选择通过 <code>docker save</code> 导出的 .tar 文件上传导入。文件最大 2GB。</p>
        <div style="margin-top: 14px">
          <input ref="fileInput" type="file" accept=".tar,.tar.gz" @change="onFileChange" style="display:none" />
          <button class="btn" @click="$refs.fileInput.click()">
            {{ uploadFile ? uploadFile.name : '选择文件...' }}
          </button>
        </div>
        <div v-if="uploading" class="progress-bar">
          <div class="progress-fill" :style="{ width: '100%' }"></div>
          <span>正在导入，请稍候...</span>
        </div>
        <div class="btns">
          <button class="btn btn-primary" @click="doImport" :disabled="!uploadFile || uploading">导入</button>
          <button class="btn" @click="showImport = false" :disabled="uploading">取消</button>
        </div>
      </div>
    </div>

    <div v-if="showPull" class="modal-mask" @click.self="showPull = false">
      <div class="modal">
        <h3>拉取远程镜像</h3>
        <p class="hint">从 Docker Hub 或其他 registry 拉取镜像，如 <code>nginx:latest</code>。</p>
        <div style="margin-top: 14px">
          <input v-model="pullName" placeholder="如 nginx:latest" @keyup.enter="doPull" />
        </div>
        <div v-if="pulling" class="progress-bar">
          <div class="progress-fill" :style="{ width: '100%' }"></div>
          <span>正在拉取，请稍候...</span>
        </div>
        <div class="btns">
          <button class="btn btn-primary" @click="doPull" :disabled="!pullName || pulling">拉取</button>
          <button class="btn" @click="showPull = false" :disabled="pulling">取消</button>
        </div>
      </div>
    </div>

    <div v-if="showDetail" class="modal-mask" @click.self="showDetail = false">
      <div class="modal modal-lg">
        <h3>镜像详情</h3>
        <div v-if="detail" class="detail-grid">
          <div class="detail-row"><span class="label">ID</span><code>{{ detail.id }}</code></div>
          <div class="detail-row"><span class="label">架构</span>{{ detail.architecture }}</div>
          <div class="detail-row"><span class="label">OS</span>{{ detail.os }}</div>
          <div class="detail-row"><span class="label">大小</span>{{ fmtBytes(detail.size) }}</div>
          <div class="detail-row"><span class="label">作者</span>{{ detail.author || '-' }}</div>
          <div class="detail-row"><span class="label">工作目录</span>{{ detail.working_dir || '-' }}</div>
          <div class="detail-row full"><span class="label">标签</span>
            <span v-for="t in detail.repo_tags" :key="t" class="tag-badge">{{ t }}</span>
            <span v-if="!detail.repo_tags || !detail.repo_tags.length">-</span>
          </div>
          <div class="detail-row full" v-if="detail.cmd && detail.cmd.length">
            <span class="label">CMD</span><code>{{ detail.cmd.join(' ') }}</code>
          </div>
          <div class="detail-row full" v-if="detail.entrypoint && detail.entrypoint.length">
            <span class="label">Entrypoint</span><code>{{ detail.entrypoint.join(' ') }}</code>
          </div>
          <div class="detail-row full" v-if="detail.env && detail.env.length">
            <span class="label">环境变量</span>
            <div class="env-list">
              <div v-for="e in detail.env" :key="e" class="env-item"><code>{{ e }}</code></div>
            </div>
          </div>
          <div class="detail-row full" v-if="detail.layers && detail.layers.length">
            <span class="label">层数</span>{{ detail.layers.length }} 层
          </div>
        </div>
        <div class="btns">
          <button class="btn" @click="showDetail = false">关闭</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, inject } from 'vue'
import { api, fmtBytes } from '../../api'

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

function shortId(id) {
  return id ? id.replace('sha256:', '').substring(0, 12) : ''
}

function fmtTime(ts) {
  if (!ts) return '-'
  const d = new Date(ts * 1000)
  const now = Date.now()
  const diff = now - d.getTime()
  if (diff < 60000) return '刚刚'
  if (diff < 3600000) return Math.floor(diff / 60000) + ' 分钟前'
  if (diff < 86400000) return Math.floor(diff / 3600000) + ' 小时前'
  return Math.floor(diff / 86400000) + ' 天前'
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

onMounted(load)

function onFileChange(e) {
  uploadFile.value = e.target.files[0] || null
}

async function doImport() {
  if (!uploadFile.value) return
  uploading.value = true
  try {
    await api.uploadImage(uploadFile.value)
    notify('镜像导入成功')
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
    notify('镜像拉取成功')
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
  if (!confirm(`确认删除镜像 ${name}？`)) return
  try {
    await api.deleteImage(im.id)
    notify('已删除')
    await load()
  } catch (e) {
    notify(e.message, 'err')
  }
}
</script>

<style scoped>
.page-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}
h2 { margin: 0; font-size: 20px; letter-spacing: 0.02em; }
h3 { margin: 0 0 14px; font-size: 15px; letter-spacing: 0.03em; }
.head-btns { display: flex; gap: 8px; }
.none-tag { color: var(--text-2); font-style: italic; }
.ops { display: flex; gap: 6px; }
.empty { color: var(--text-2); font-size: 13px; padding: 16px; text-align: center; }
.hint { color: var(--text-2); font-size: 12px; line-height: 1.6; }
.hint code {
  background: rgba(255,255,255,0.06);
  padding: 1px 5px;
  border-radius: 4px;
  font-size: 11.5px;
}
.modal-mask {
  position: fixed; inset: 0;
  background: rgba(2, 4, 10, 0.7);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  display: flex; align-items: center; justify-content: center; z-index: 100;
  animation: fadeIn 0.2s ease;
}
@keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
.modal {
  background: rgba(14, 20, 38, 0.92);
  backdrop-filter: blur(30px) saturate(150%);
  -webkit-backdrop-filter: blur(30px) saturate(150%);
  border: 1px solid var(--glass-border-strong);
  border-radius: var(--radius-lg);
  padding: 26px;
  width: 520px; max-width: 92vw;
  max-height: 90vh; overflow-y: auto;
  box-shadow: 0 24px 80px rgba(0, 0, 0, 0.6), inset 0 1px 0 rgba(255, 255, 255, 0.08);
  animation: pop 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}
.modal-lg { width: 640px; }
@keyframes pop {
  from { transform: scale(0.96) translateY(10px); opacity: 0; }
  to { transform: scale(1) translateY(0); opacity: 1; }
}
.btns { margin-top: 16px; display: flex; gap: 8px; }
.progress-bar {
  margin-top: 14px;
  position: relative;
  height: 28px;
  background: rgba(255,255,255,0.06);
  border-radius: 8px;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
}
.progress-fill {
  position: absolute;
  inset: 0;
  background: linear-gradient(90deg, rgba(34,211,238,0.3), rgba(139,92,246,0.3));
  animation: shimmer 1.5s ease-in-out infinite;
}
@keyframes shimmer {
  0%, 100% { opacity: 0.4; }
  50% { opacity: 0.8; }
}
.progress-bar span {
  position: relative;
  z-index: 1;
  font-size: 12px;
  color: var(--text-1);
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
