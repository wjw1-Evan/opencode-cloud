// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { downloadText, fmtBytes } from './api'

describe('fmtBytes', () => {
  it('formats sizes across units', () => {
    expect(fmtBytes(0)).toBe('0 B')
    expect(fmtBytes(512)).toBe('512.0 B')
    expect(fmtBytes(1024)).toBe('1.0 KB')
    expect(fmtBytes(1536)).toBe('1.5 KB')
    expect(fmtBytes(1048576)).toBe('1.0 MB')
    expect(fmtBytes(1073741824)).toBe('1.0 GB')
    expect(fmtBytes(undefined)).toBe('0 B')
  })
})

describe('downloadText', () => {
  let revokeMock

  beforeEach(() => {
    revokeMock = vi.fn()
    vi.stubGlobal('URL', {
      createObjectURL: vi.fn(() => 'blob:fake'),
      revokeObjectURL: revokeMock,
    })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('creates a link with the right filename and content type, then revokes the URL', async () => {
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    downloadText('accounts.csv', 'username,password\nstu001,abc', 'text/csv')

    expect(clickSpy).toHaveBeenCalledTimes(1)
    expect(URL.createObjectURL).toHaveBeenCalledTimes(1)

    // URL revocation is deferred so Safari keeps the download alive.
    await new Promise((resolve) => setTimeout(resolve, 1100))
    expect(revokeMock).toHaveBeenCalledWith('blob:fake')
  })
})
