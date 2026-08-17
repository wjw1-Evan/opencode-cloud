import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from './api'

function jsonResponse(body, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('silent token refresh', () => {
  let fetchMock

  beforeEach(() => {
    fetchMock = vi.fn()
    globalThis.fetch = fetchMock
    globalThis.window = { location: { href: '' } }
  })

  afterEach(() => {
    vi.restoreAllMocks()
    delete globalThis.window
  })

  it('retries the original request once after a successful silent refresh', async () => {
    const me = { user: { id: 'u1' } }
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ error: 'invalid token' }, 401))
      .mockResolvedValueOnce(jsonResponse({}, 200)) // /auth/refresh
      .mockResolvedValueOnce(jsonResponse({ data: me }))

    const result = await api.me()

    expect(result).toEqual(me)
    expect(fetchMock).toHaveBeenCalledTimes(3)
    expect(fetchMock.mock.calls[0][0]).toBe('/platform/auth/me')
    expect(fetchMock.mock.calls[1][0]).toBe('/platform/auth/refresh')
    expect(fetchMock.mock.calls[2][0]).toBe('/platform/auth/me')
    // the retry must not loop forever
    expect(globalThis.window.location.href).toBe('')
  })

  it('redirects to the login page when the refresh token is expired too', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ error: 'invalid token' }, 401))
      .mockResolvedValueOnce(jsonResponse({ error: 'invalid refresh token' }, 401))

    await expect(api.me()).rejects.toThrow('unauthorized')
    expect(globalThis.window.location.href).toBe('/')
  })

  it('redirects when the retried request still returns 401', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ error: 'invalid token' }, 401))
      .mockResolvedValueOnce(jsonResponse({}, 200)) // refresh succeeds
      .mockResolvedValueOnce(jsonResponse({ error: 'invalid token' }, 401))

    await expect(api.me()).rejects.toThrow('unauthorized')
    expect(globalThis.window.location.href).toBe('/')
  })

  it('does not trigger a refresh on non-401 errors', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ error: 'db error' }, 500))

    await expect(api.me()).rejects.toThrow('db error')
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock.mock.calls[0][1]).not.toEqual('/platform/auth/refresh')
  })

  it('does not trigger a refresh for login or initialized endpoints', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ error: 'invalid credentials' }, 401))

    await expect(api.login('stu', 'wrong')).rejects.toThrow('invalid credentials')
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('deduplicates concurrent refreshes into a single request', async () => {
    const me = { user: { id: 'u1' } }
    const ok = () => jsonResponse({ data: me })
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ error: 'invalid token' }, 401)) // me #1
      .mockResolvedValueOnce(jsonResponse({ error: 'invalid token' }, 401)) // me #2
      .mockResolvedValueOnce(jsonResponse({}, 200)) // one shared refresh
      .mockResolvedValueOnce(ok())
      .mockResolvedValueOnce(ok())

    const [a, b] = await Promise.all([api.me(), api.me()])

    expect(a).toEqual(me)
    expect(b).toEqual(me)
    const refreshCalls = fetchMock.mock.calls.filter(([p]) => p === '/platform/auth/refresh')
    expect(refreshCalls).toHaveLength(1)
  })

  it('retries the image upload after a silent refresh', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ error: 'invalid token' }, 401))
      .mockResolvedValueOnce(jsonResponse({}, 200))
      .mockResolvedValueOnce(jsonResponse({ data: { message: 'imported' } }))

    const result = await api.uploadImage(new File(['x'], 'img.tar'))

    expect(result.message).toBe('imported')
    expect(fetchMock).toHaveBeenCalledTimes(3)
    const uploads = fetchMock.mock.calls.filter(([p]) => p === '/platform/admin/images/import')
    expect(uploads).toHaveLength(2)
    expect(uploads[1][1].method).toBe('POST')
    expect(uploads[1][1].body).toBeInstanceOf(FormData)
  })
})
