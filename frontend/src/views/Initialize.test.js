// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { pushMock } = vi.hoisted(() => ({ pushMock: vi.fn() }))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushMock }),
}))

vi.mock('../api', () => ({
  api: { initialize: vi.fn() },
}))

import { api } from '../api'
import Initialize from './Initialize.vue'

const i18nStub = { t: (key) => key }

function mountInitialize() {
  return mount(Initialize, {
    global: { provide: { i18n: i18nStub } },
  })
}

describe('Initialize', () => {
  beforeEach(() => {
    pushMock.mockClear()
    api.initialize.mockReset()
  })

  it('renders the admin setup form', () => {
    const wrapper = mountInitialize()
    expect(wrapper.findAll('input').length).toBe(3)
    expect(wrapper.find('form').exists()).toBe(true)
  })

  it('rejects submission when passwords do not match', async () => {
    const wrapper = mountInitialize()
    const inputs = wrapper.findAll('input')
    await inputs[0].setValue('admin')
    await inputs[1].setValue('pass1234')
    await inputs[2].setValue('pass5678')
    await wrapper.find('form').trigger('submit')

    expect(wrapper.find('.err').text()).toBe('pwdMismatch')
    expect(api.initialize).not.toHaveBeenCalled()
  })

  it('initializes the admin and redirects to the login page', async () => {
    api.initialize.mockResolvedValue({ initialized: true })
    const wrapper = mountInitialize()
    const inputs = wrapper.findAll('input')
    await inputs[0].setValue('admin')
    await inputs[1].setValue('pass1234')
    await inputs[2].setValue('pass1234')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(api.initialize).toHaveBeenCalledWith('admin', 'pass1234')
    expect(pushMock).toHaveBeenCalledWith('/')
  })

  it('shows the error when initialization fails', async () => {
    api.initialize.mockRejectedValue(new Error('already initialized'))
    const wrapper = mountInitialize()
    const inputs = wrapper.findAll('input')
    await inputs[0].setValue('admin')
    await inputs[1].setValue('pass1234')
    await inputs[2].setValue('pass1234')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(wrapper.find('.err').text()).toBe('already initialized')
    expect(pushMock).not.toHaveBeenCalled()
  })
})
