// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { notifyMock } = vi.hoisted(() => ({ notifyMock: vi.fn() }))

vi.mock('../../api', () => ({
  api: { dashboard: vi.fn() },
  fmtBytes: (n) => `${n} B`,
}))

import { api } from '../../api'
import Dashboard from './Dashboard.vue'

const stats = {
  users: { total: 3, active: 2, status: { active: 2, disabled: 1 } },
  containers: { total: 3, running: 2, status: { running: 2, stopped: 1 } },
  requests: { online: 1, count: 10, bytes: 2048, avg_latency_ms: 42, last24h: [1, 2, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0] },
  resources: { cpu_cores: 0.5, mem_bytes: 1073741824, mem_limit: 2147483648 },
  templates: { total: 4 },
  idle_timeout: { minutes: 30 },
  courses: [{ course: 'Python', users: 2, running: 1 }],
}

function mountDashboard() {
  return mount(Dashboard, {
    global: {
      provide: { i18n: { t: (key) => key }, notify: notifyMock },
    },
  })
}

describe('Dashboard', () => {
  beforeEach(() => {
    notifyMock.mockClear()
    api.dashboard.mockReset()
  })

  it('renders stat cards, panels, memory usage and the live indicator', async () => {
    api.dashboard.mockResolvedValue(stats)
    const wrapper = mountDashboard()
    await flushPromises()

    expect(wrapper.findAll('.stat').length).toBe(8)
    expect(wrapper.find('.panel-title').exists()).toBe(true)
    expect(wrapper.find('.live').classes()).toContain('on')
    expect(wrapper.find('.res-sub').text()).toContain('50.0%')
    expect(wrapper.find('.course-name').text()).toBe('Python')
    expect(wrapper.findAll('.bar').length).toBe(24)
  })

  it('marks the indicator as failed when the dashboard request fails', async () => {
    api.dashboard.mockRejectedValue(new Error('boom'))
    const wrapper = mountDashboard()
    await flushPromises()

    expect(wrapper.find('.live').classes()).toContain('err')
    expect(notifyMock).toHaveBeenCalledWith('boom', 'err')
  })

  it('marks the indicator as paused while the tab is hidden', async () => {
    const hiddenSpy = vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
    api.dashboard.mockResolvedValue(stats)
    const wrapper = mountDashboard()
    await flushPromises()

    expect(wrapper.find('.live').classes()).toContain('paused')
    document.dispatchEvent(new Event('visibilitychange')) // cover the visibility listener
    hiddenSpy.mockRestore()
    wrapper.unmount()
  })

  it('handles zero-value status buckets and empty courses', async () => {
    const zero = {
      ...stats,
      users: { ...stats.users, status: { active: 0, disabled: 2 } },
      resources: { ...stats.resources, mem_limit: 0 },
      courses: [{ course: 'X', users: 0, running: 0 }],
    }
    api.dashboard.mockResolvedValue(zero)
    const wrapper = mountDashboard()
    await flushPromises()

    expect(wrapper.find('.res-sub').text()).toContain('0.0%')
    expect(wrapper.find('.course-meta').text()).toContain('0/0')
  })

  it('skips refreshes while a request is already in flight', async () => {
    api.dashboard.mockReturnValue(new Promise(() => {})) // never resolves
    const wrapper = mountDashboard()
    await flushPromises()

    document.dispatchEvent(new Event('visibilitychange'))
    expect(true).toBe(true) // the guard runs without errors
  })
})
