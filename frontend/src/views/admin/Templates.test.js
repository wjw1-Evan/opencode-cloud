// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ConfirmDialog from '../../components/ConfirmDialog.vue'

const { notifyMock } = vi.hoisted(() => ({ notifyMock: vi.fn() }))

vi.mock('../../api', () => ({
  api: {
    listTemplates: vi.fn(),
    createTemplate: vi.fn(),
    updateTemplate: vi.fn(),
    deleteTemplate: vi.fn(),
  },
  fmtBytes: (n) => `${n} B`,
}))

import { api } from '../../api'
import Templates from './Templates.vue'

const templates = [
  { id: 't1', name: 'opencode', image: 'ghcr.io/anomalyco/opencode:latest', internal_port: 4096, extra_ports: [3000], cpu_limit: 0.5, mem_limit: 1073741824, is_system: true, workspace_dir: '/workspace', envs: {}, command: ['serve'] },
]
const customTpl = { id: 't2', name: 'custom', image: 'nginx:latest', internal_port: 8080, extra_ports: [], cpu_limit: 0.5, mem_limit: 1073741824, is_system: false, workspace_dir: '/workspace', envs: {}, command: [] }

function mountTemplates(list = templates) {
  api.listTemplates.mockResolvedValue(list)
  return mount(Templates, {
    global: {
      provide: { i18n: { t: (key) => key }, notify: notifyMock },
      stubs: { teleport: true },
    },
  })
}

describe('Templates', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    notifyMock.mockClear()
  })

  it('renders the template table with a system badge', async () => {
    const wrapper = mountTemplates()
    await flushPromises()

    const rows = wrapper.findAll('tbody tr')
    expect(rows.length).toBe(1)
    expect(rows[0].text()).toContain('opencode')
    expect(rows[0].find('.badge.system').exists()).toBe(true)
  })

  it('requires name and image before creating', async () => {
    const wrapper = mountTemplates()
    await flushPromises()

    await wrapper.find('.head-right .btn-primary').trigger('click')
    await wrapper.find('.modal .btns .btn-primary').trigger('click')
    await flushPromises()

    expect(notifyMock).toHaveBeenCalledWith('nameAndImageRequired', 'err')
    expect(api.createTemplate).not.toHaveBeenCalled()
  })

  it('creates a template with parsed ports, envs and command', async () => {
    api.createTemplate.mockResolvedValue({ id: 't2' })
    const wrapper = mountTemplates()
    await flushPromises()

    await wrapper.find('.head-right .btn-primary').trigger('click')
    const inputs = wrapper.findAll('.grid input')
    await inputs[0].setValue('my-tpl') // name
    await inputs[1].setValue('nginx:latest') // image
    await inputs[3].setValue('3000,5173') // extra ports
    await inputs[5].setValue('2') // mem GB
    await wrapper.findAll('.modal input')[7].setValue('web --port 4096') // command (after the 7 grid fields)
    await wrapper.find('.modal textarea').setValue('K=V\nA=B') // envs
    await wrapper.find('.modal .btns .btn-primary').trigger('click')
    await flushPromises()

    expect(api.createTemplate).toHaveBeenCalledWith(expect.objectContaining({
      name: 'my-tpl',
      image: 'nginx:latest',
      extra_ports: [3000, 5173],
      envs: { K: 'V', A: 'B' },
      command: ['web', '--port', '4096'],
      mem_limit: 2147483648,
    }))
  })

  it('prefills the edit form (including env vars) and saves via updateTemplate', async () => {
    api.updateTemplate.mockResolvedValue({})
    const envTpl = { id: 't3', name: 'env-tpl', image: 'img', internal_port: 8080, extra_ports: [3000], cpu_limit: 0.5, mem_limit: 1073741824, is_system: false, workspace_dir: '/workspace', envs: { A: 'B' }, command: ['run'] }
    const wrapper = mountTemplates([envTpl])
    await flushPromises()

    await wrapper.find('tbody tr .ops .btn').trigger('click') // edit
    expect(wrapper.find('.grid input').element.value).toBe('env-tpl')
    expect(wrapper.find('.modal textarea').element.value).toContain('A=B')

    await wrapper.find('.grid input').setValue('env-tpl-v2')
    await wrapper.find('.modal .btns .btn-primary').trigger('click')
    await flushPromises()

    expect(api.updateTemplate).toHaveBeenCalledWith('t3', expect.objectContaining({ name: 'env-tpl-v2' }))
  })

  it('deletes a template after confirmation', async () => {
    api.deleteTemplate.mockResolvedValue({})
    const wrapper = mountTemplates([customTpl])
    await flushPromises()

    await wrapper.find('tbody tr .ops .btn-danger').trigger('click')
    await flushPromises()
    expect(wrapper.findComponent(ConfirmDialog).props('visible')).toBe(true)

    await wrapper.findComponent(ConfirmDialog).findAll('.confirm-btns .btn')[1].trigger('click')
    await flushPromises()
    expect(api.deleteTemplate).toHaveBeenCalledWith('t2')
  })

  it('cancelling the delete confirmation keeps the template', async () => {
    const wrapper = mountTemplates([customTpl])
    await flushPromises()

    await wrapper.find('tbody tr .ops .btn-danger').trigger('click')
    await wrapper.findComponent(ConfirmDialog).findAll('.confirm-btns .btn')[0].trigger('click')
    expect(api.deleteTemplate).not.toHaveBeenCalled()
  })

  it('shows the error when the template list refresh fails', async () => {
    api.listTemplates.mockRejectedValue(new Error('db down'))
    const wrapper = mount(Templates, {
      global: {
        provide: { i18n: { t: (key) => key }, notify: notifyMock },
        stubs: { teleport: true },
      },
    })
    await flushPromises()
    expect(notifyMock).toHaveBeenCalledWith('db down', 'err')
  })

  it('shows the error when saving a template fails', async () => {
    api.createTemplate.mockRejectedValue(new Error('boom'))
    const wrapper = mountTemplates()
    await flushPromises()

    await wrapper.find('.head-right .btn-primary').trigger('click')
    const inputs = wrapper.findAll('.grid input')
    await inputs[0].setValue('t')
    await inputs[1].setValue('img')
    await wrapper.find('.modal .btns .btn-primary').trigger('click')
    await flushPromises()

    expect(notifyMock).toHaveBeenCalledWith('boom', 'err')
  })

  it('renders every field of the create modal', async () => {
    const wrapper = mountTemplates()
    await flushPromises()

    await wrapper.find('.head-right .btn-primary').trigger('click')
    expect(wrapper.findAll('.grid input').length).toBe(7)
    expect(wrapper.findAll('.modal input').length).toBe(8)
    expect(wrapper.find('.modal textarea').exists()).toBe(true)
    wrapper.unmount()
  })

  it('shows the error when deleting a template fails', async () => {
    api.deleteTemplate.mockRejectedValue(new Error('in use'))
    const wrapper = mountTemplates([customTpl])
    await flushPromises()

    await wrapper.find('tbody tr .ops .btn-danger').trigger('click')
    await wrapper.findComponent(ConfirmDialog).findAll('.confirm-btns .btn')[1].trigger('click')
    await flushPromises()
    expect(notifyMock).toHaveBeenCalledWith('in use', 'err')
  })

  it('skips polling while the tab is hidden', async () => {
    const hiddenSpy = vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
    const wrapper = mountTemplates()
    await flushPromises()
    expect(api.listTemplates).toHaveBeenCalledTimes(0)
    hiddenSpy.mockRestore()
    wrapper.unmount()
  })
})
