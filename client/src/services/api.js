const API_BASE = import.meta.env.VITE_API_BASE || 'http://127.0.0.1:8000'

async function request(path, options = {}) {
  const response = await fetch(`${API_BASE}${path}`, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(options.headers || {}),
    },
    ...options,
  })

  const contentType = response.headers.get('content-type') || ''
  const body = contentType.includes('application/json') ? await response.json() : null

  if (!response.ok) {
    throw new Error(body?.detail || '请求失败')
  }
  return body
}

export const api = {
  base: API_BASE,
  me: () => request('/api/auth/me'),
  login: (payload) => request('/api/auth/login', { method: 'POST', body: JSON.stringify(payload) }),
  register: (payload) => request('/api/auth/register', { method: 'POST', body: JSON.stringify(payload) }),
  logout: () => request('/api/auth/logout', { method: 'POST', body: JSON.stringify({}) }),
  images: (sort) => request(`/api/images?sort=${encodeURIComponent(sort)}`),
  createImage: (prompt) => request('/api/images', { method: 'POST', body: JSON.stringify({ prompt }) }),
  jobEventsUrl: (id) => `${API_BASE}/api/images/${id}/events`,
  like: (id) => request(`/api/images/${id}/like`, { method: 'POST', body: JSON.stringify({}) }),
  favorite: (id) => request(`/api/images/${id}/favorite`, { method: 'POST', body: JSON.stringify({}) }),
  adminMe: () => request('/api/admin/me'),
  adminLogin: (payload) => request('/api/admin/login', { method: 'POST', body: JSON.stringify(payload) }),
  adminLogout: () => request('/api/admin/logout', { method: 'POST', body: JSON.stringify({}) }),
  adminDashboard: () => request('/api/admin/dashboard'),
  adminUsers: () => request('/api/admin/users'),
  adminSetUserAdmin: (id, is_admin) =>
    request(`/api/admin/users/${id}/admin`, {
      method: 'PUT',
      body: JSON.stringify({ is_admin }),
    }),
  adminGenerations: () => request('/api/admin/generations'),
  adminDeleteGeneration: (id) => request(`/api/admin/generations/${id}`, { method: 'DELETE' }),
  adminProviders: () => request('/api/admin/providers'),
  adminSaveProvider: (payload, id) => {
    const path = id ? `/api/admin/providers/${id}` : '/api/admin/providers'
    return request(path, { method: id ? 'PUT' : 'POST', body: JSON.stringify(payload) })
  },
  adminSetConcurrency: (concurrency) =>
    request('/api/admin/settings/concurrency', {
      method: 'PUT',
      body: JSON.stringify({ concurrency }),
    }),
}

export function mediaUrl(path) {
  if (!path) return ''
  if (path.startsWith('http')) return path
  return `${API_BASE}${path}`
}
