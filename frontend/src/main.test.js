// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  createApp: vi.fn(() => ({
    provide: vi.fn().mockReturnThis(),
    use: vi.fn().mockReturnThis(),
    mount: vi.fn(),
  })),
  createRouter: vi.fn(() => ({ beforeEach: vi.fn() })),
  routes: null,
  guard: null,
  createWebHistory: vi.fn(() => ({})),
  me: vi.fn().mockResolvedValue({ user: { id: 'u1' } }),
  useI18n: vi.fn(() => ({ locale: { value: 'zh' }, setLocale: vi.fn(), t: vi.fn() })),
}))

// capture the route table and the navigation guard so the test can exercise them
mocks.createRouter.mockImplementation((opts) => {
  mocks.routes = opts.routes
  return {
    beforeEach: (fn) => {
      mocks.guard = fn
    },
  }
})

vi.mock('vue', () => ({ createApp: mocks.createApp }))
vi.mock('vue-router', () => ({
  createRouter: mocks.createRouter,
  createWebHistory: mocks.createWebHistory,
}))
vi.mock('./api', () => ({ api: { me: mocks.me } }))
vi.mock('./i18n', () => ({ useI18n: mocks.useI18n }))

describe('main bootstrap', () => {
  beforeEach(() => {
    vi.resetModules()
    mocks.createApp.mockClear()
    mocks.createRouter.mockClear()
    document.body.innerHTML = '<div id="app"></div>'
  })

  it('creates the router with the platform routes and mounts the app', async () => {
    await import('./main')

    expect(mocks.createWebHistory).toHaveBeenCalled()
    expect(mocks.createRouter).toHaveBeenCalled()
    expect(mocks.createApp).toHaveBeenCalled()
    const app = mocks.createApp.mock.results[0].value
    expect(app.provide).toHaveBeenCalled()
    expect(app.use).toHaveBeenCalled()
    expect(app.mount).toHaveBeenCalledWith('#app')

    // resolve every lazy route component so the dynamic imports run
    for (const route of mocks.routes) {
      if (route.component) await route.component()
    }
    for (const child of mocks.routes[3].children) {
      if (child.component) await child.component()
    }
  })

  it('runs the navigation guard for public and protected routes', async () => {
    await import('./main')

    expect(mocks.guard).toBeTypeOf('function')
    await expect(mocks.guard({ path: '/' })).resolves.toBe(true)

    mocks.me.mockResolvedValueOnce({ user: { id: 'u1' } })
    await expect(mocks.guard({ path: '/portal' })).resolves.toBe(true)

    mocks.me.mockRejectedValueOnce(new Error('no session'))
    await expect(mocks.guard({ path: '/admin/users' })).resolves.toEqual({ path: '/' })
  })
})
