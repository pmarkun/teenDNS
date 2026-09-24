export type Action = 'allow' | 'block' | 'observe'

export type Rule = {
  domain: string
  include_subdomains: boolean
  action: Action
  category?: string
  reason?: string
}

export type Profile = {
  id: string
  label?: string
  hostname: string
  disabled?: boolean
  default_action: Action
  version: number
  rules: Rule[]
}

export type EventSummary = {
  profile_id: string
  total: number
  blocked: number
  observed: number
  unknown: number
  categories: Array<{ category: string; count: number }>
}

const tokenKey = 'teendns-admin-token'

export function getToken() {
  return sessionStorage.getItem(tokenKey) || import.meta.env.VITE_ADMIN_TOKEN || ''
}

export function setToken(value: string) {
  sessionStorage.setItem(tokenKey, value)
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${getToken()}`,
      ...init?.headers,
    },
  })
  if (!response.ok) {
    const body = await response.json().catch(() => ({}))
    throw new Error(body.error || `Falha ${response.status}`)
  }
  return response.json() as Promise<T>
}

export const api = {
  async profiles() {
    const result = await request<{ profiles: Profile[] }>('/api/v1/profiles')
    return result.profiles
  },
  createProfile(label: string) {
    return request<Profile>('/api/v1/profiles', {
      method: 'POST',
      body: JSON.stringify({ label }),
    })
  },
  updateProfile(profile: Profile) {
    return request<Profile>(`/api/v1/profiles/${profile.id}`, {
      method: 'PUT',
      body: JSON.stringify({
        label: profile.label || profile.id,
        default_action: profile.default_action,
        rules: profile.rules,
      }),
    })
  },
  rotate(profileID: string) {
    return request<Profile>(`/api/v1/profiles/${profileID}/rotate-endpoint`, { method: 'POST' })
  },
  summary(profileID: string) {
    return request<EventSummary>(`/api/v1/profiles/${profileID}/summary`)
  },
}
