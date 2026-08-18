// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { pushMock } = vi.hoisted(() => ({ pushMock: vi.fn() }))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushMock }),
}))

vi.mock('../api', () => ({
  api: {
    initialized: vi.fn().mockResolvedValue({ initialized: true }),
    login: vi.fn(),
  },
}))

import { api } from '../api'
import Login from './Login.vue'

const i18nStub = {
  t: (key) => key,
}

function mountLogin() {
  return mount(Login, {
    global: { provide: { i18n: i18nStub } },
  })
}

describe('Login', () => {
  beforeEach(() => {
    pushMock.mockClear()
    api.login.mockReset()
  })

  it('renders username, password fields and the submit button', () => {
    const wrapper = mountLogin()
    expect(wrapper.find('input[autocomplete="username"]').exists()).toBe(true)
    expect(wrapper.find('input[type="password"]').exists()).toBe(true)
    expect(wrapper.find('form').exists()).toBe(true)
  })

  it('submits credentials and sends an admin to the dashboard', async () => {
    api.login.mockResolvedValue({ user: { role: 'admin' } })
    const wrapper = mountLogin()
    await wrapper.find('input[autocomplete="username"]').setValue('admin')
    await wrapper.find('input[type="password"]').setValue('admin123')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(api.login).toHaveBeenCalledWith('admin', 'admin123')
    expect(pushMock).toHaveBeenCalledWith('/admin/dashboard')
  })

  it('sends a student to the portal', async () => {
    api.login.mockResolvedValue({ user: { role: 'user' } })
    const wrapper = mountLogin()
    await wrapper.find('input[autocomplete="username"]').setValue('stu001')
    await wrapper.find('input[type="password"]').setValue('pass12345')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(api.login).toHaveBeenCalledWith('stu001', 'pass12345')
    expect(pushMock).toHaveBeenCalledWith('/portal')
  })

  it('shows the error message when login fails', async () => {
    api.login.mockRejectedValue(new Error('invalid credentials'))
    const wrapper = mountLogin()
    await wrapper.find('input[autocomplete="username"]').setValue('admin')
    await wrapper.find('input[type="password"]').setValue('wrong')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(wrapper.find('.err').text()).toBe('invalid credentials')
    expect(pushMock).not.toHaveBeenCalled()
  })

  it('redirects to the initialization page when no admin exists', async () => {
    api.initialized.mockResolvedValue({ initialized: false })
    const wrapper = mountLogin()
    await flushPromises()
    expect(pushMock).toHaveBeenCalledWith('/initialize')
  })
})
