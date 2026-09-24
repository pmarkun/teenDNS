export type Action = 'allow' | 'block' | 'observe'

export type Rule = {
  domain: string
  include_subdomains: boolean
  action: Action
  category?: string
  reason?: string
}

export type RuleGroup = {
  id: string
  name: string
  action: Action
  category?: string
  reason?: string
  domains: string[]
  default_domains?: string[]
  domain_source?: string
  customized?: boolean
}

export type Profile = {
  id: string
  house_id?: string
  label?: string
  hostname: string
  disabled?: boolean
  default_action: Action
  version: number
  rules: Rule[]
  groups?: RuleGroup[]
}

export type CatalogPackage = {
  id: string
  name: string
  category: string
  reason: string
  domain_count: number
  suggested_action: Action
}

export type HouseRegistration = {
  house: { id: string; name: string }
  admin_token: string
  profile: Profile
}

export type Invitation = {
  email: string
  expires_at: string
  code: string
  link: string
  email_sent: boolean
}

export type EventSummary = {
  profile_id: string
  total: number
  blocked: number
  observed: number
  unknown: number
  categories: Array<{ category: string; count: number }>
}

export type PairingChallenge = {
  id: string
  dns_name: string
  expires_at: string
}

export type PairingStatus = {
  paired: boolean
  session_token?: string
  expires_at?: string
}

export type YouthProfile = {
  label: string
  rules: Array<{ name: string; reason: string }>
}

const tokenKey = 'teendns-admin-token'
const pairingTokenKey = 'teendns-pairing-token'

export function getToken() {
  return sessionStorage.getItem(tokenKey) || import.meta.env.VITE_ADMIN_TOKEN || ''
}

export function setToken(value: string) {
  sessionStorage.setItem(tokenKey, value)
}

export function getPairingToken() {
  return sessionStorage.getItem(pairingTokenKey) || ''
}

export function setPairingToken(value: string) {
  if (value) sessionStorage.setItem(pairingTokenKey, value)
  else sessionStorage.removeItem(pairingTokenKey)
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
  createInvitation(email: string) {
    return request<Invitation>('/api/v1/invitations', {
      method: 'POST',
      body: JSON.stringify({ email }),
    })
  },
  registerHouse(invitationCode: string, houseName: string, profileName: string, preset: string) {
    return request<HouseRegistration>('/api/v1/houses', {
      method: 'POST',
      body: JSON.stringify({
        invitation_code: invitationCode,
        house_name: houseName,
        profile_name: profileName,
        preset,
      }),
    })
  },
  async profiles() {
    const result = await request<{ profiles: Profile[] }>('/api/v1/profiles')
    return result.profiles
  },
  async packages() {
    const result = await request<{ packages: CatalogPackage[] }>('/api/v1/catalog/packages')
    return result.packages
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
        groups: profile.groups || [],
      }),
    })
  },
  setPackage(profileID: string, packageID: string, enabled: boolean, action: Action) {
    return request<Profile>(`/api/v1/profiles/${profileID}/packages/${packageID}`, {
      method: 'PUT',
      body: JSON.stringify({ enabled, action }),
    })
  },
  rotate(profileID: string) {
    return request<Profile>(`/api/v1/profiles/${profileID}/rotate-endpoint`, { method: 'POST' })
  },
  summary(profileID: string) {
    return request<EventSummary>(`/api/v1/profiles/${profileID}/summary`)
  },
  createPairingChallenge() {
    return request<PairingChallenge>('/api/v1/pairing/challenges', { method: 'POST' })
  },
  pairingStatus(challengeID: string) {
    return request<PairingStatus>(`/api/v1/pairing/challenges/${challengeID}`)
  },
  youthProfile(sessionToken: string) {
    return request<YouthProfile>('/api/v1/youth/profile', {
      headers: { Authorization: `Bearer ${sessionToken}` },
    })
  },
}
