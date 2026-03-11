const API_BASE = '/api/v1'

export async function apiFetch(path, options = {}) {
  const headers = { ...options.headers }

  if (options.body && typeof options.body === 'object') {
    headers['Content-Type'] = 'application/json'
    options.body = JSON.stringify(options.body)
  }

  try {
    const res = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers,
      credentials: 'same-origin', // send session cookie
    })

    if (res.status === 401) {
      // Session expired or not authenticated — trigger re-render
      window.dispatchEvent(new Event('pagefire:unauthorized'))
      return { data: null, error: 'unauthorized' }
    }

    if (res.status === 204) {
      return { data: null, error: null }
    }

    const data = await res.json()

    if (res.ok) {
      return { data, error: null }
    }

    return { data: null, error: data.error || `HTTP ${res.status}` }
  } catch (err) {
    return { data: null, error: err.message }
  }
}

export function apiGet(path) {
  return apiFetch(path)
}

export function apiPost(path, body) {
  return apiFetch(path, { method: 'POST', body })
}

export function apiPut(path, body) {
  return apiFetch(path, { method: 'PUT', body })
}

export function apiDelete(path) {
  return apiFetch(path, { method: 'DELETE' })
}
