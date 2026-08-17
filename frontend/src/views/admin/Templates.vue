<template>
  <div>
    <div class="page-head">
      <h2>镜像模板</h2>
      <button class="btn btn-primary" @click="openCreate">+ 新建模板</button>
    </div>

    <div class="card">
      <table>
        <thead>
          <tr><th>名称</th><th>镜像</th><th>端口</th><th>CPU</th><th>内存</th><th>操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="t in templates" :key="t.id">
            <td>{{ t.name }}<span v-if="t.is_system" class="badge system">系统</span></td>
            <td><code>{{ t.image }}</code></td>
            <td>{{ t.internal_port }}{{ (t.extra_ports || []).length ? ' / ' + (t.extra_ports || []).join(',') : '' }}</td>
            <td>{{ t.cpu_limit }}</td>
            <td>{{ fmtBytes(t.mem_limit) }}</td>
            <td class="ops">
              <button class="btn" @click="openEdit(t)">编辑</button>
              <button class="btn btn-danger" :disabled="t.is_system" @click="del(t)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="!templates.length" class="empty">暂无模板，请先创建</p>
    </div>

    <div v-if="showModal" class="modal-mask" @click.self="close">
      <div class="modal">
        <h3>{{ editing ? '编辑模板' : '新建模板' }}</h3>
        <div class="grid">
          <div><label>名称</label><input v-model="f.name" placeholder="如 default" /></div>
          <div class="span2"><label>镜像</label><input v-model="f.image" placeholder="如 ghcr.io/anomalyco/opencode:latest" /></div>
          <div><label>主端口</label><input v-model.number="f.internal_port" type="number" /></div>
          <div><label>附加端口（逗号分隔，可选）</label><input v-model="extraPortsText" placeholder="如 3000,5173,8000" /></div>
          <div><label>CPU 限制（核）</label><input v-model.number="f.cpu_limit" type="number" step="0.1" /></div>
          <div><label>内存限制（GB）</label><input v-model.number="memGb" type="number" step="0.5" /></div>
          <div><label>工作目录</label><input v-model="f.workspace_dir" /></div>
        </div>
        <div class="hint" style="margin-top: 12px">
          附加端口用于运行/调试应用（如前端 dev server、后端 API），用户通过 /port/{port}/ 访问。
        </div>
        <div style="margin-top: 12px">
          <label>启动命令（以空格分隔，可选，默认镜像 CMD）</label>
          <input v-model="cmdText" placeholder="如 web --hostname 0.0.0.0 --port 4096" />
        </div>
        <div style="margin-top: 12px">
          <label>环境变量（每行 KEY=VALUE）</label>
          <textarea v-model="envsText" rows="4" placeholder="MODEL_PROVIDER=openrouter"></textarea>
        </div>
        <div class="btns">
          <button class="btn btn-primary" @click="save" :disabled="busy">{{ editing ? '保存' : '创建' }}</button>
          <button class="btn" @click="close">取消</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, inject } from 'vue'
import { api, fmtBytes } from '../../api'

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

function openEdit(t) {
  editing.value = t
  f.value = { ...t }
  memGb.value = t.mem_limit / (1 << 30)
  envsText.value = Object.entries(t.envs || {}).map(([k, v]) => `${k}=${v}`).join('\n')
  cmdText.value = (t.command || []).join(' ')
  extraPortsText.value = (t.extra_ports || []).join(',')
  showModal.value = true
}

function close() {
  showModal.value = false
  resetForm()
}

onMounted(async () => {
  templates.value = await api.listTemplates()
})

async function save() {
  if (!f.value.name || !f.value.image) return notify('名称和镜像必填', 'err')
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
      notify('已保存')
    } else {
      await api.createTemplate(payload)
      notify('已创建')
    }
    templates.value = await api.listTemplates()
    close()
  } catch (e) {
    notify(e.message, 'err')
  } finally {
    busy.value = false
  }
}

async function del(t) {
  if (!confirm(`确认删除模板 ${t.name}？`)) return
  try {
    await api.deleteTemplate(t.id)
    notify('已删除')
    templates.value = await api.listTemplates()
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
.grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; }
.grid .span2 { grid-column: span 2; }
.btns { margin-top: 16px; display: flex; gap: 8px; }
.ops { display: flex; gap: 6px; }
.empty { color: var(--text-2); font-size: 13px; padding: 16px; text-align: center; }
.badge.system {
  margin-left: 8px;
  color: var(--cyan);
  border-color: rgba(34, 211, 238, 0.4);
  background: rgba(34, 211, 238, 0.08);
  font-size: 10.5px;
}
.hint { color: var(--text-2); font-size: 12px; }
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
  width: 640px; max-width: 92vw;
  max-height: 90vh; overflow-y: auto;
  box-shadow: 0 24px 80px rgba(0, 0, 0, 0.6), inset 0 1px 0 rgba(255, 255, 255, 0.08);
  animation: pop 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}
@keyframes pop {
  from { transform: scale(0.96) translateY(10px); opacity: 0; }
  to { transform: scale(1) translateY(0); opacity: 1; }
}
</style>
