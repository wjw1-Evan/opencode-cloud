// Single in-flight refresh promise so concurrent 401s trigger only one call.
let refreshing = null

function refreshSession() {
  if (!refreshing) {
    refreshing = fetch('/platform/auth/refresh', { method: 'POST' })
      .then((resp) => {
        if (!resp.ok) throw new Error('refresh failed')
      })
      .finally(() => {
        refreshing = null
      })
  }
  return refreshing
}

async function request(method, path, body, retried = false) {
  const opts = { method, headers: {} }
  if (body !== undefined) {
    opts.headers['Content-Type'] = 'application/json'
    opts.body = JSON.stringify(body)
  }
  const resp = await fetch(path, opts)
  const authRoute = path.includes('/auth/login') || path.includes('/auth/initialized') || path.includes('/auth/refresh')
  if (resp.status === 401 && !authRoute) {
    if (!retried) {
      // Access token expired: silently refresh via the refresh cookie and retry
      // the original request once. Only bounce to the login page if the refresh
      // itself fails (e.g. the 24h refresh token has expired too).
      try {
        await refreshSession()
      } catch {
        window.location.href = '/'
        throw new Error('unauthorized')
      }
      return request(method, path, body, true)
    }
    // The refresh succeeded but the retried request still 401s: the session is
    // gone, bounce to the login page.
    window.location.href = '/'
    throw new Error('unauthorized')
  }
  const data = await resp.json().catch(() => ({}))
  if (!resp.ok) {
    const err = new Error(data.error || resp.statusText)
    err.status = resp.status
    throw err
  }
  return data.data
}

export const api = {
  initialized: () => request('GET', '/platform/auth/initialized'),
  initialize: (username, password) => request('POST', '/platform/auth/initialize', { username, password }),
  login: (username, password) => request('POST', '/platform/auth/login', { username, password }),
  me: () => request('GET', '/platform/auth/me'),
  changePassword: (oldPassword, newPassword) => request('POST', '/platform/auth/change-password', { old_password: oldPassword, new_password: newPassword }),
  // users
  listUsers: () => request('GET', '/platform/admin/users'),
  batchUsers: (payload) => request('POST', '/platform/admin/users/batch', payload),
  batchUserAction: (payload) => request('POST', '/platform/admin/users/batch/action', payload),
  updateUser: (id, payload) => request('PATCH', `/platform/admin/users/${id}`, payload),
  deleteUser: (id) => request('DELETE', `/platform/admin/users/${id}`),
  // containers
  listContainers: () => request('GET', '/platform/admin/containers'),
  provisionBatch: (payload) => request('POST', '/platform/admin/containers/batch', payload),
  containerAction: (id, action) => request('POST', `/platform/admin/containers/${id}/${action}`),
  containerStats: (id) => request('GET', `/platform/admin/containers/${id}/stats`),
  allStats: () => request('GET', '/platform/admin/containers/stats/all'),
  // images
  listImages: () => request('GET', '/platform/admin/images'),
  getImage: (id) => request('GET', `/platform/admin/images/${id}`),
  deleteImage: (id) => request('DELETE', `/platform/admin/images/${id}`),
  pullImage: (image) => request('POST', '/platform/admin/images/pull', { image }),
  uploadImage: async (file) => {
    const form = new FormData()
    form.append('file', file)
    const doUpload = async (retried) => {
      const resp = await fetch('/platform/admin/images/import', { method: 'POST', body: form })
      if (resp.status === 401 && !retried) {
        try {
          await refreshSession()
        } catch {
          window.location.href = '/'
          throw new Error('unauthorized')
        }
        return doUpload(true)
      }
      const data = await resp.json().catch(() => ({}))
      if (!resp.ok) { const err = new Error(data.error || resp.statusText); err.status = resp.status; throw err }
      return data.data
    }
    return doUpload(false)
  },
  // templates
  listTemplates: () => request('GET', '/platform/admin/templates'),
  createTemplate: (payload) => request('POST', '/platform/admin/templates', payload),
  updateTemplate: (id, payload) => request('PUT', `/platform/admin/templates/${id}`, payload),
  deleteTemplate: (id) => request('DELETE', `/platform/admin/templates/${id}`),
  // stats
  platform: () => request('GET', '/platform/admin/platform'),
  dashboard: () => request('GET', '/platform/admin/stats/dashboard'),
}

export function downloadText(filename, text, mime = 'text/plain') {
  const blob = new Blob([text], { type: mime })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  // Defer revocation so Safari keeps the download alive.
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}

export function fmtBytes(n) {
  if (!n) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(1)} ${units[i]}`
}
