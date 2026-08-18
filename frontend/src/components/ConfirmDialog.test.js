// @vitest-environment happy-dom
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import ConfirmDialog from './ConfirmDialog.vue'

const i18nStub = {
  t: (key) => ({ cancel: 'Cancel', dialogConfirm: 'Confirm' }[key] || key),
}

function mountDialog(props = {}) {
  return mount(ConfirmDialog, {
    props: { visible: true, ...props },
    global: {
      provide: { i18n: i18nStub },
      stubs: { teleport: true },
    },
  })
}

describe('ConfirmDialog', () => {
  it('renders nothing when not visible', () => {
    const wrapper = mount(ConfirmDialog, {
      props: { visible: false },
      global: { provide: { i18n: i18nStub }, stubs: { teleport: true } },
    })
    expect(wrapper.find('.confirm-mask').exists()).toBe(false)
  })

  it('renders the message', () => {
    const wrapper = mountDialog({ message: '确认退出登录？' })
    expect(wrapper.find('.confirm-msg').text()).toBe('确认退出登录？')
  })

  it('shows a danger icon for danger type', () => {
    const wrapper = mountDialog({ type: 'danger' })
    expect(wrapper.find('.confirm-icon').text()).toBe('!')
  })

  it('shows a question icon for non-danger type', () => {
    const wrapper = mountDialog({ type: 'primary' })
    expect(wrapper.find('.confirm-icon').text()).toBe('?')
  })

  it('uses localized button labels', () => {
    const wrapper = mountDialog()
    expect(wrapper.text()).toContain('Cancel')
    expect(wrapper.text()).toContain('Confirm')
  })

  it('emits confirm when the confirm button is clicked', async () => {
    const wrapper = mountDialog()
    await wrapper.findAll('.confirm-btns .btn')[1].trigger('click')
    expect(wrapper.emitted('confirm')).toBeTruthy()
  })

  it('emits cancel when the cancel button is clicked', async () => {
    const wrapper = mountDialog()
    await wrapper.findAll('.confirm-btns .btn')[0].trigger('click')
    expect(wrapper.emitted('cancel')).toBeTruthy()
  })
})
