// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ConfirmDialog from '../../components/ConfirmDialog.vue'

vi.mock('../../api', () => ({
  api: { changePassword: vi.fn() },
}))

import { api } from '../../api'
import AdminLayout from './AdminLayout.vue'

const i18nStub = { t: (key) => key }

function mountLayout() {
  return mount(AdminLayout, {
    global: {
      provide: { i18n: i18nStub },
      stubs: {
        RouterLink: { template: '<a v-bind="$attrs"><slot /></a>' },
        teleport: true,
      },
    },
  })
}

describe('AdminLayout', () => {
  beforeEach(() => {
    document.body.style.overflow = ''
  })

  it('toggles the sidebar drawer with the menu button', async () => {
    const wrapper = mountLayout()
    const side = wrapper.find('.side')
    expect(side.classes()).not.toContain('open')

    await wrapper.find('.menu-btn').trigger('click')
    expect(side.classes()).toContain('open')
    expect(document.body.style.overflow).toBe('hidden')

    await wrapper.find('.side-mask').trigger('click')
    expect(side.classes()).not.toContain('open')
    expect(document.body.style.overflow).toBe('')
    wrapper.unmount()
  })

  it('asks for confirmation before logout', async () => {
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { href: '' },
    })
    const wrapper = mountLayout()

    await wrapper.find('.logout').trigger('click')
    const dialog = wrapper.findComponent(ConfirmDialog)
    expect(dialog.props('visible')).toBe(true)

    await dialog.findAll('.confirm-btns .btn')[1].trigger('click')
    expect(window.location.href).toBe('/platform/auth/logout')
  })

  it('rejects a password change when the new passwords do not match', async () => {
    const wrapper = mountLayout()
    await wrapper.find('.pwd-btn').trigger('click')
    const inputs = wrapper.findAll('.pwd-fields input')
    await inputs[0].setValue('oldpass1')
    await inputs[1].setValue('newpass123')
    await inputs[2].setValue('newpass456')
    await wrapper.find('.modal .btns .btn-primary').trigger('click')

    expect(wrapper.find('.pwd-error').text()).toBe('pwdMismatch')
    expect(api.changePassword).not.toHaveBeenCalled()
  })

  it('changes the password successfully and closes the modal', async () => {
    api.changePassword.mockResolvedValue({})
    const wrapper = mountLayout()
    await wrapper.find('.pwd-btn').trigger('click')
    const inputs = wrapper.findAll('.pwd-fields input')
    await inputs[0].setValue('oldpass1')
    await inputs[1].setValue('newpass123')
    await inputs[2].setValue('newpass123')
    await wrapper.find('.modal .btns .btn-primary').trigger('click')
    await flushPromises()

    expect(api.changePassword).toHaveBeenCalledWith('oldpass1', 'newpass123')
    expect(wrapper.find('.modal-mask').exists()).toBe(false)
  })

  it('shows the error when changing the password fails', async () => {
    api.changePassword.mockRejectedValue(new Error('bad'))
    const wrapper = mountLayout()
    await wrapper.find('.pwd-btn').trigger('click')
    const inputs = wrapper.findAll('.pwd-fields input')
    await inputs[0].setValue('oldpass1')
    await inputs[1].setValue('newpass123')
    await inputs[2].setValue('newpass123')
    await wrapper.find('.modal .btns .btn-primary').trigger('click')
    await flushPromises()

    expect(wrapper.find('.pwd-error').text()).toBe('bad')
  })

  it('closes menus and modals through every close affordance', async () => {
    const wrapper = mountLayout()
    await wrapper.find('.menu-btn').trigger('click')
    expect(wrapper.find('.side').classes()).toContain('open')

    for (const link of wrapper.findAll('nav a')) {
      await link.trigger('click')
    }
    await wrapper.find('.menu-btn').trigger('click')
    await wrapper.find('.side-close').trigger('click')
    expect(wrapper.find('.side').classes()).not.toContain('open')

    await wrapper.find('.menu-btn').trigger('click')
    await wrapper.find('.pwd-btn').trigger('click')
    await wrapper.find('.modal-mask').trigger('click') // @click.self close
    expect(wrapper.find('.modal-mask').exists()).toBe(false)

    await wrapper.find('.pwd-btn').trigger('click')
    await wrapper.find('.modal .btns .btn:last-child').trigger('click') // cancel
    expect(wrapper.find('.modal-mask').exists()).toBe(false)

    await wrapper.find('.logout').trigger('click')
    const dialog = wrapper.findComponent(ConfirmDialog)
    await dialog.findAll('.confirm-btns .btn')[0].trigger('click') // cancel logout
    expect(dialog.props('visible')).toBe(false)
    wrapper.unmount()
  })
})
