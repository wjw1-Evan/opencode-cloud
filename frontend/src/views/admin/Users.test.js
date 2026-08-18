// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ConfirmDialog from '../../components/ConfirmDialog.vue'

const { notifyMock, downloadTextMock } = vi.hoisted(() => ({
  notifyMock: vi.fn(),
  downloadTextMock: vi.fn(),
}))

vi.mock('../../api', () => ({
  api: {
    listUsers: vi.fn(),
    listTemplates: vi.fn(),
    listContainers: vi.fn(),
    platform: vi.fn(),
    batchUsers: vi.fn(),
    provisionBatch: vi.fn(),
    batchUserAction: vi.fn(),
    updateUser: vi.fn(),
    deleteUser: vi.fn(),
  },
  downloadText: downloadTextMock,
}))

import { api } from '../../api'
import Users from './Users.vue'

const users = [
  { id: 'u1', username: 'stu001', role: 'user', course: 'Python', created_at: '2026-01-01T00:00:00Z', manual_disabled: false, expires_at: '2027-01-01T00:00:00Z' },
  { id: 'u2', username: 'stu002', role: 'user', course: 'Java', created_at: '2026-01-02T00:00:00Z', manual_disabled: false, expires_at: null },
]
const templates = [{ id: 't1', name: 'opencode' }]
const containers = [{
  id: 'c1',
  user_id: 'u1',
  template_id: 't1',
  status: 'running',
  internal_port: 4096,
  extra_ports: [],
  container_name: 'user-stu001',
  container_id: 'c1',
  created_at: '2026-01-01T00:00:00Z',
  started_at: '2026-01-02T00:00:00Z',
}]

function mountUsers(list = users, tpls = templates, conts = containers) {
  api.listUsers.mockResolvedValue(list)
  api.listTemplates.mockResolvedValue(tpls)
  api.listContainers.mockResolvedValue(conts)
  api.platform.mockResolvedValue({ network: 'user-net' })
  return mount(Users, {
    global: {
      provide: { i18n: { t: (key) => key }, notify: notifyMock },
      stubs: { teleport: true },
    },
  })
}

function userWith(id, username, overrides = {}) {
  return {
    id,
    username,
    role: 'user',
    course: 'Python',
    created_at: '2026-01-01T00:00:00Z',
    manual_disabled: false,
    expires_at: null,
    ...overrides,
  }
}

describe('Users', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    notifyMock.mockClear()
  })

  it('renders users with their container status', async () => {
    const wrapper = mountUsers()
    await flushPromises()

    const rows = wrapper.findAll('tbody tr')
    expect(rows.length).toBe(2)
    expect(rows[0].find('.uname').text()).toBe('stu001')
    expect(rows[0].text()).toContain('running')
    expect(wrapper.find('.course-filter').exists()).toBe(true)
  })

  it('filters the list by course', async () => {
    const wrapper = mountUsers()
    await flushPromises()

    await wrapper.find('.course-filter').setValue('Python')
    const rows = wrapper.findAll('tbody tr')
    expect(rows.length).toBe(1)
    expect(rows[0].find('.uname').text()).toBe('stu001')
  })

  it('requires a template before batch create', async () => {
    const wrapper = mountUsers()
    await flushPromises()

    await wrapper.find('.batch select').setValue('')
    await wrapper.findAll('.batch input')[0].setValue('Python 基础')
    await wrapper.find('.batch .btn-primary').trigger('click')
    await flushPromises()

    expect(notifyMock).toHaveBeenCalledWith('selectTemplate', 'err')
    expect(api.batchUsers).not.toHaveBeenCalled()
  })

  it('batch-creates accounts and provisions containers', async () => {
    api.batchUsers.mockResolvedValue({
      created: 2,
      users: [{ id: 'u1' }, { id: 'u2' }],
      accounts: [
        { username: 'python001', password: 'abc' },
        { username: 'python002', password: 'def' },
      ],
    })
    api.provisionBatch.mockResolvedValue({ provisioned: 2, results: [{ ok: true }, { ok: true }] })
    const wrapper = mountUsers()
    await flushPromises()

    await wrapper.findAll('.batch input')[0].setValue('Python 基础')
    await wrapper.find('.batch .btn-primary').trigger('click')
    await flushPromises()

    expect(api.batchUsers).toHaveBeenCalledWith(expect.objectContaining({ count: 1, course: 'Python 基础' }))
    expect(api.provisionBatch).toHaveBeenCalled()
    expect(downloadTextMock).toHaveBeenCalledWith(
      'accounts.csv',
      'username,password\npython001,abc\npython002,def',
      'text/csv',
    )
  })

  it('disables bulk actions until a user is selected', async () => {
    const wrapper = mountUsers()
    await flushPromises()

    expect(wrapper.findAll('.toolbar-btns .btn')[1].attributes('disabled')).toBeDefined()
  })

  it('rebuilds selected containers with force', async () => {
    api.provisionBatch.mockResolvedValue({ provisioned: 1, results: [{ ok: true }] })
    const wrapper = mountUsers()
    await flushPromises()

    await wrapper.findAll('tbody tr')[0].find('.chk input').setValue(true)
    await wrapper.findAll('.toolbar-btns .btn')[0].trigger('click') // rebuild
    await wrapper.findComponent(ConfirmDialog).findAll('.confirm-btns .btn')[1].trigger('click')
    await flushPromises()

    expect(api.provisionBatch).toHaveBeenCalledWith({ template_id: 't1', user_ids: ['u1'], force: true })
  })

  it('disables a user through the bulk action', async () => {
    const wrapper = mountUsers()
    await flushPromises()

    await wrapper.findAll('tbody tr')[0].find('.chk input').setValue(true)
    await wrapper.findAll('.toolbar-btns .btn')[4].trigger('click') // disable
    await wrapper.findComponent(ConfirmDialog).findAll('.confirm-btns .btn')[1].trigger('click')
    await flushPromises()

    expect(api.batchUserAction).toHaveBeenCalledWith({ user_ids: ['u1'], action: 'disable' })
  })

  it('opens the status detail and saves status/expiry changes', async () => {
    api.updateUser.mockResolvedValue({ id: 'u1', manual_disabled: true, expires_at: '2025-01-01T00:00:00Z' })
    const wrapper = mountUsers()
    await flushPromises()

    await wrapper.findAll('tbody tr')[0].find('.badge.clickable').trigger('click')
    expect(wrapper.find('.modal-mask').exists()).toBe(true)

    await wrapper.find('.switch input').setValue(false) // disable the account
    await wrapper.find('.expiry-in').setValue('2025-01-01T10:00')
    await wrapper.find('.modal .btns .btn').trigger('click') // save
    await flushPromises()

    expect(api.updateUser).toHaveBeenCalledWith('u1', expect.objectContaining({
      status: 'disabled',
      expires_at: expect.any(String),
    }))
    expect(notifyMock).toHaveBeenCalled()
  })

  it('shows status without editing controls for admins', async () => {
    const wrapper = mountUsers([{ id: 'a1', username: 'root', role: 'admin', course: '', created_at: '2026-01-01T00:00:00Z', manual_disabled: false, expires_at: null }])
    await flushPromises()

    await wrapper.find('tbody tr .badge.clickable').trigger('click')
    expect(wrapper.find('.switch').exists()).toBe(false)
    expect(wrapper.find('.expiry-in').exists()).toBe(false)
  })

  it('reports partial container provisioning failures', async () => {
    api.batchUsers.mockResolvedValue({
      created: 2,
      users: [{ id: 'u1' }, { id: 'u2' }],
      accounts: [
        { username: 'python001', password: 'abc' },
        { username: 'python002', password: 'def' },
      ],
    })
    api.provisionBatch.mockResolvedValue({
      provisioned: 1,
      results: [{ ok: true }, { ok: false, error: 'boom' }],
    })
    const wrapper = mountUsers()
    await flushPromises()

    await wrapper.findAll('.batch input')[0].setValue('Python 基础')
    await wrapper.find('.batch .btn-primary').trigger('click')
    await flushPromises()

    expect(notifyMock).toHaveBeenCalledWith('createPartial', 'err')
  })

  it('runs start, restart, stop, enable and delete bulk actions', async () => {
    const wrapper = mountUsers()
    await flushPromises()
    await wrapper.findAll('tbody tr')[0].find('.chk input').setValue(true)

    const actions = [
      [1, 'start'],
      [2, 'restart'],
      [3, 'stop'],
      [5, 'enable'],
      [6, 'delete'],
    ]
    for (const [idx, action] of actions) {
      api.batchUserAction.mockClear()
      api.batchUserAction.mockResolvedValue([{ ok: true }])
      await wrapper.findAll('.toolbar-btns .btn')[idx].trigger('click')
      const dialog = wrapper.findComponent(ConfirmDialog)
      expect(dialog.props('visible')).toBe(true)
      await wrapper.findComponent(ConfirmDialog).findAll('.confirm-btns .btn')[1].trigger('click')
      await flushPromises()
      expect(api.batchUserAction).toHaveBeenCalledWith({ user_ids: ['u1'], action })
    }
    // delete clears the selection
    expect(wrapper.findAll('tbody .chk input')[0].element.checked).toBe(false)
    wrapper.unmount()
  })

  it('cancels the confirm dialog and clears the expiry with the x button', async () => {
    const wrapper = mountUsers()
    await flushPromises()

    await wrapper.findAll('tbody tr')[0].find('.chk input').setValue(true)
    await wrapper.findAll('.toolbar-btns .btn')[6].trigger('click') // delete
    await wrapper.findComponent(ConfirmDialog).findAll('.confirm-btns .btn')[0].trigger('click') // cancel
    expect(api.batchUserAction).not.toHaveBeenCalled()

    await wrapper.findAll('tbody tr')[0].find('.badge.clickable').trigger('click')
    await wrapper.find('.expiry-in').setValue('2027-01-01T10:00')
    await wrapper.find('.btn-sm').trigger('click') // ×
    expect(wrapper.find('.expiry-in').element.value).toBe('')
  })

  it('toggles selections individually and via the header checkbox', async () => {
    api.batchUserAction.mockResolvedValue([{ ok: true }, { ok: true }])
    const wrapper = mountUsers()
    await flushPromises()

    const rowCheck = wrapper.findAll('tbody tr')[0].find('.chk input')
    await rowCheck.setValue(true)
    await rowCheck.setValue(false)
    expect(wrapper.findAll('tbody .chk input')[0].element.checked).toBe(false)

    const headerCheck = wrapper.find('thead .chk input')
    await headerCheck.setValue(true)
    await wrapper.findAll('.toolbar-btns .btn')[1].trigger('click') // start both
    await wrapper.findComponent(ConfirmDialog).findAll('.confirm-btns .btn')[1].trigger('click')
    await flushPromises()
    expect(api.batchUserAction).toHaveBeenCalledWith({ user_ids: ['u1', 'u2'], action: 'start' })

    await headerCheck.setValue(false)
    const checked = wrapper.findAll('tbody .chk input').filter((i) => i.element.checked)
    expect(checked.length).toBe(0)
  })

  it('shows a disabled badge, reactivates the account and tolerates invalid expiry', async () => {
    const disabledUser = userWith('u9', 'stu009', { manual_disabled: true, expires_at: 'garbage' })
    api.updateUser.mockResolvedValue({ id: 'u9', manual_disabled: false })
    const wrapper = mountUsers([disabledUser])
    await flushPromises()

    expect(wrapper.find('tbody tr .badge.clickable').classes()).toContain('disabled')
    await wrapper.find('tbody tr .badge.clickable').trigger('click')
    expect(wrapper.find('.expiry-in').element.value).toBe('') // invalid expiry -> empty draft

    await wrapper.find('.switch input').setValue(true) // activate
    await wrapper.find('.modal .btns .btn').trigger('click')
    await flushPromises()
    expect(api.updateUser).toHaveBeenCalledWith('u9', expect.objectContaining({ status: 'active' }))
    expect(notifyMock).toHaveBeenCalledWith('accountReactivated')
  })

  it('notifies when an account is saved as disabled', async () => {
    api.updateUser.mockResolvedValue({ id: 'u1', manual_disabled: true })
    const wrapper = mountUsers()
    await flushPromises()

    await wrapper.findAll('tbody tr')[0].find('.badge.clickable').trigger('click')
    await wrapper.find('.switch input').setValue(false)
    await wrapper.find('.modal .btns .btn').trigger('click')
    await flushPromises()
    expect(notifyMock).toHaveBeenCalledWith('savedDisabled')
  })

  it('validates the expiry input and handles save outcomes', async () => {
    api.updateUser.mockResolvedValue({})
    const wrapper = mountUsers()
    await flushPromises()
    await wrapper.findAll('tbody tr')[0].find('.badge.clickable').trigger('click')

    await wrapper.find('.expiry-in').setValue('')
    await wrapper.find('.modal .btns .btn').trigger('click')
    await flushPromises()
    expect(api.updateUser).toHaveBeenCalledWith('u1', expect.objectContaining({ expires_in_days: 0 }))
    expect(notifyMock).toHaveBeenCalledWith('expirySaved')

    api.updateUser.mockRejectedValue(new Error('db'))
    await wrapper.findAll('tbody tr')[0].find('.badge.clickable').trigger('click')
    await wrapper.find('.modal .btns .btn').trigger('click')
    await flushPromises()
    expect(notifyMock).toHaveBeenCalledWith('db', 'err')
  })

  it('reports partial rebuild failures', async () => {
    api.provisionBatch.mockResolvedValue({ provisioned: 1, results: [{ ok: false, error: 'boom' }] })
    const wrapper = mountUsers()
    await flushPromises()

    await wrapper.findAll('tbody tr')[0].find('.chk input').setValue(true)
    await wrapper.findAll('.toolbar-btns .btn')[0].trigger('click') // rebuild
    await wrapper.findComponent(ConfirmDialog).findAll('.confirm-btns .btn')[1].trigger('click')
    await flushPromises()
    expect(notifyMock).toHaveBeenCalledWith('rebuildPartial', 'err')
  })

  it('shows the error when a bulk action fails', async () => {
    api.batchUserAction.mockRejectedValue(new Error('boom'))
    const wrapper = mountUsers()
    await flushPromises()

    await wrapper.findAll('tbody tr')[0].find('.chk input').setValue(true)
    await wrapper.findAll('.toolbar-btns .btn')[1].trigger('click')
    await wrapper.findComponent(ConfirmDialog).findAll('.confirm-btns .btn')[1].trigger('click')
    await flushPromises()
    expect(notifyMock).toHaveBeenCalledWith('boom', 'err')
  })

  it('requires a course before batch create', async () => {
    const wrapper = mountUsers()
    await flushPromises()

    await wrapper.findAll('.batch input')[0].setValue('   ')
    await wrapper.find('.batch .btn-primary').trigger('click')
    await flushPromises()
    expect(notifyMock).toHaveBeenCalledWith('fillCourse', 'err')
  })

  it('handles batch create failures', async () => {
    api.batchUsers.mockResolvedValue({ created: 0, users: [], accounts: [] })
    const wrapper = mountUsers()
    await flushPromises()

    await wrapper.findAll('.batch input')[0].setValue('Python')
    await wrapper.find('.batch .btn-primary').trigger('click')
    await flushPromises()
    expect(notifyMock).toHaveBeenCalledWith('noAccountsCreated', 'err')

    api.batchUsers.mockRejectedValue(new Error('boom'))
    await wrapper.find('.batch .btn-primary').trigger('click')
    await flushPromises()
    expect(notifyMock).toHaveBeenCalledWith('boom', 'err')
  })

  it('shows template details including env vars and memory sizes', async () => {
    const smallTpl = { id: 't1', name: 'small', image: 'img', internal_port: 8080, extra_ports: [], cpu_limit: 0.5, mem_limit: 524288000, is_system: false, workspace_dir: '/workspace', envs: { A: 'B' }, command: ['run'] }
    const wrapper = mountUsers(users, [smallTpl])
    await flushPromises()

    await wrapper.findAll('tbody tr')[0].find('.badge.tpl').trigger('click')
    expect(wrapper.find('.modal-mask').exists()).toBe(true)
    expect(wrapper.text()).toContain('small')
    expect(wrapper.text()).toContain('A=B')
    expect(wrapper.text()).toContain('500 MB')
  })

  it('formats container uptime in several ranges', async () => {
    const cases = [
      { label: 'uptime10s', started_at: new Date(Date.now() - 10 * 1000).toISOString() },
      { label: 'uptime10m', started_at: new Date(Date.now() - 10 * 60 * 1000).toISOString() },
      { label: 'uptime10h0m', started_at: new Date(Date.now() - 10 * 3600 * 1000).toISOString() },
      { label: 'future', started_at: new Date(Date.now() + 3600 * 1000).toISOString(), future: true },
      { label: '-', started_at: null, dash: true },
    ]
    for (const c of cases) {
      const u = userWith('u' + c.label, 'stu' + c.label)
      const cont = {
        id: 'c1',
        user_id: u.id,
        template_id: 't1',
        status: 'running',
        internal_port: 4096,
        extra_ports: [],
        container_name: 'user-stu',
        container_id: 'c1',
        created_at: '2026-01-01T00:00:00Z',
        started_at: c.started_at,
      }
      const wrapper = mountUsers([u], templates, [cont])
      await flushPromises()
      await wrapper.find('tbody tr .badge.clickable.running').trigger('click')
      expect(wrapper.text()).toContain('user-stu')
      expect(wrapper.text()).toContain('c1')
      if (c.dash) {
        expect(wrapper.text()).toContain('uptime-')
      } else if (c.future) {
        expect(wrapper.text()).toContain('uptime')
        expect(wrapper.text()).not.toContain('uptime-')
      } else {
        expect(wrapper.text()).toContain(c.label)
      }
      wrapper.unmount()
    }
  })
})
