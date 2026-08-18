<template>
  <Teleport to="body">
    <div v-if="visible" class="confirm-mask" @click.self="cancel">
      <div class="confirm-modal">
        <div class="confirm-icon" :class="type">
          <span v-if="type === 'danger'">!</span>
          <span v-else>?</span>
        </div>
        <p class="confirm-msg">{{ message }}</p>
        <div class="confirm-btns">
          <button class="btn" @click="cancel">{{ cancelText || t('cancel') }}</button>
          <button :class="['btn', type === 'danger' ? 'btn-danger' : 'btn-primary']" @click="confirm">{{ confirmText || t('dialogConfirm') }}</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { inject } from 'vue'

const { t } = inject('i18n')

const props = defineProps({
  visible: Boolean,
  message: { type: String, default: '' },
  confirmText: { type: String, default: '' },
  cancelText: { type: String, default: '' },
  type: { type: String, default: 'primary' },
})

const emit = defineEmits(['confirm', 'cancel'])

function confirm() { emit('confirm') }
function cancel() { emit('cancel') }
</script>

<style scoped>
.confirm-mask {
  position: fixed;
  inset: 0;
  background: rgba(2, 4, 10, 0.7);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  animation: fadeIn 0.2s ease;
}
@keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
.confirm-modal {
  background: rgba(14, 20, 38, 0.95);
  backdrop-filter: blur(18px) saturate(140%);
  -webkit-backdrop-filter: blur(18px) saturate(140%);
  border: 1px solid var(--glass-border-strong);
  border-radius: var(--radius-lg);
  padding: 28px 32px;
  width: 400px;
  max-width: 90vw;
  text-align: center;
  box-shadow: 0 24px 80px rgba(0, 0, 0, 0.6), inset 0 1px 0 rgba(255, 255, 255, 0.08);
  animation: pop 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}
@keyframes pop {
  from { transform: scale(0.96) translateY(10px); opacity: 0; }
  to { transform: scale(1) translateY(0); opacity: 1; }
}
.confirm-icon {
  width: 48px;
  height: 48px;
  margin: 0 auto 16px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 22px;
  font-weight: 700;
}
.confirm-icon.primary {
  background: rgba(34, 211, 238, 0.12);
  color: var(--cyan);
  border: 1px solid rgba(34, 211, 238, 0.3);
}
.confirm-icon.danger {
  background: rgba(248, 113, 113, 0.12);
  color: var(--err);
  border: 1px solid rgba(248, 113, 113, 0.3);
}
.confirm-msg {
  color: var(--text-1);
  font-size: 14px;
  line-height: 1.6;
  margin: 0 0 24px;
}
.confirm-btns {
  display: flex;
  gap: 10px;
  justify-content: center;
}
.confirm-btns .btn {
  min-width: 80px;
  padding: 10px 20px;
}
</style>
