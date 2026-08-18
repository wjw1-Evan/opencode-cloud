// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ConfirmDialog from '../components/ConfirmDialog.vue'

const { notifyMock } = vi.hoisted(() => ({ notifyMock: vi.fn() }))

vi.mock('../api', () => ({
  api: {
    me: vi.fn(),
    changePassword: vi.fn(),
  },
}))

import { api } from '../api'
import Portal from './Portal.vue'

const i18nStub = { t: (key) => key }

function mountPortal(meData) {
  api.me.mockResolvedValue(meData)
  return mount(Portal, {
    global: {
      provide: { i18n: i18nStub, notify: notifyMock },
      stubs: { teleport: true },
    },
  })
}

describe('Portal', () => {
  beforeEach(() => {
    notifyMock.mockClear()
    api.me.mockReset()
    api.changePassword.mockReset()
  })

  it('renders the user name and running container info', async () => {
    const wrapper = mountPortal({
      user: { username: 'stu001' },
      container: { status: 'running', internal_port: 4096, extra_ports: [], container_name: 'user-stu001' },
    })
    await flushPromises()

    expect(wrapper.find('.uname').text()).toBe('stu001')
    expect(wrapper.find('.status-panel').exists()).toBe(true)
    expect(wrapper.text()).toContain('user-stu001')
    expect(wrapper.find('.open').classes()).not.toContain('dim')
  })

  it('disables open-env and notifies when no container is assigned', async () => {
    const wrapper = mountPortal({ user: { username: 'stu001' }, container: null })
    await flushPromises()

    const link = wrapper.find('.open')
    expect(link.classes()).toContain('dim')
    await link.trigger('click')
    expect(notifyMock).toHaveBeenCalledWith('noContainer', 'err')
  })

  it('rejects a password change when the new passwords do not match', async () => {
    const wrapper = mountPortal({ user: { username: 'stu001' }, container: null })
    await flushPromises()

    await wrapper.findAll('.actions .btn')[2].trigger('click')
    const inputs = wrapper.findAll('.pwd-fields input')
    await inputs[0].setValue('oldpass1')
    await inputs[1].setValue('newpass123')
    await inputs[2].setValue('newpass456')
    await wrapper.find('.modal .btns .btn').trigger('click')

    expect(api.changePassword).not.toHaveBeenCalled()
    expect(wrapper.find('.pwd-error').text()).toBe('pwdMismatch')
  })

  it('changes the password successfully and closes the modal', async () => {
    api.changePassword.mockResolvedValue({ updated: true })
    const wrapper = mountPortal({ user: { username: 'stu001' }, container: null })
    await flushPromises()

    await wrapper.findAll('.actions .btn')[2].trigger('click')
    const inputs = wrapper.findAll('.pwd-fields input')
    await inputs[0].setValue('oldpass1')
    await inputs[1].setValue('newpass123')
    await inputs[2].setValue('newpass123')
    await wrapper.find('.modal .btns .btn').trigger('click')
    await flushPromises()

    expect(api.changePassword).toHaveBeenCalledWith('oldpass1', 'newpass123')
    expect(wrapper.find('.modal-mask').exists()).toBe(false)
  })

  it('asks for confirmation before logout and logs out on confirm', async () => {
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { href: '' },
    })
    const wrapper = mountPortal({ user: { username: 'stu001' }, container: null })
    await flushPromises()

    await wrapper.find('.top .btn').trigger('click')
    const dialog = wrapper.findComponent(ConfirmDialog)
    expect(dialog.props('visible')).toBe(true)

    await dialog.findAll('.confirm-btns .btn')[1].trigger('click')
    expect(window.location.href).toBe('/platform/auth/logout')
  })

  it('refreshes the status when the refresh button is clicked', async () => {
    const runningData = {
      user: { username: 'stu001' },
      container: { status: 'running', internal_port: 4096, extra_ports: [], container_name: 'user-stu001' },
    }
    const wrapper = mountPortal(runningData)
    await flushPromises()

    api.me.mockClear()
    api.me.mockResolvedValue(runningData)
    await wrapper.findAll('.actions .btn')[1].trigger('click') // refresh
    await flushPromises()
    expect(api.me).toHaveBeenCalled()
  })

  it('does not block open-env when a container exists', async () => {
    const wrapper = mountPortal({
      user: { username: 'stu001' },
      container: { status: 'running', internal_port: 4096, extra_ports: [], container_name: 'user-stu001' },
    })
    await flushPromises()

    await wrapper.find('.open').trigger('click')
    expect(notifyMock).not.toHaveBeenCalled()
  })

  it('shows the error message when changing the password fails', async () => {
    api.changePassword.mockRejectedValue(new Error('wrong password'))
    const wrapper = mountPortal({ user: { username: 'stu001' }, container: null })
    await flushPromises()

    await wrapper.findAll('.actions .btn')[2].trigger('click')
    const inputs = wrapper.findAll('.pwd-fields input')
    await inputs[0].setValue('oldpass1')
    await inputs[1].setValue('newpass123')
    await inputs[2].setValue('newpass123')
    await wrapper.find('.modal .btns .btn').trigger('click')
    await flushPromises()

    expect(wrapper.find('.pwd-error').text()).toBe('wrong password')
  })

  it('closes modals via the mask and the cancel buttons', async () => {
    const wrapper = mountPortal({ user: { username: 'stu001' }, container: null })
    await flushPromises()

    await wrapper.findAll('.actions .btn')[2].trigger('click')
    await wrapper.find('.modal-mask').trigger('click') // @click.self on the mask
    expect(wrapper.find('.modal-mask').exists()).toBe(false)

    await wrapper.findAll('.actions .btn')[2].trigger('click')
    await wrapper.find('.modal .btns .btn:last-child').trigger('click') // cancel
    expect(wrapper.find('.modal-mask').exists()).toBe(false)

    await wrapper.find('.top .btn').trigger('click')
    const dialog = wrapper.findComponent(ConfirmDialog)
    await dialog.findAll('.confirm-btns .btn')[0].trigger('click') // cancel logout
    expect(dialog.props('visible')).toBe(false)
  })
})
