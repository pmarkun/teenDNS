import { type FormEvent, useState } from 'react'
import { api, type HouseRegistration, setToken } from '../api'
import { Logo } from '../components'

export function Register() {
  const [invitationCode, setInvitationCode] = useState('')
  const [houseName, setHouseName] = useState('')
  const [profileName, setProfileName] = useState('')
  const [created, setCreated] = useState<HouseRegistration | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setError('')
    try {
      const registration = await api.registerHouse(invitationCode, houseName, profileName)
      setToken(registration.admin_token)
      setCreated(registration)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Não foi possível criar a casa')
    } finally {
      setBusy(false)
    }
  }

  if (created) {
    return (
      <main className="register register--done">
        <Logo />
        <section className="register-card">
          <p className="kicker">CASA CRIADA</p>
          <h1>{created.house.name.toUpperCase()}</h1>
          <p>Guarde esta chave. Ela abre o painel da casa e não aparece de novo.</p>
          <div className="secret-card">
            <code>{created.admin_token}</code>
            <button type="button" onClick={() => void navigator.clipboard.writeText(created.admin_token)}>COPIAR CHAVE</button>
          </div>
          <div className="created-profile">
            <span>primeiro perfil</span>
            <strong>{created.profile.label}</strong>
            <code>{created.profile.hostname}</code>
          </div>
          <a className="button button--ink" href="/painel">ENTRAR NA CASA</a>
        </section>
      </main>
    )
  }

  return (
    <main className="register">
      <Logo />
      <form className="register-card" onSubmit={(event) => void submit(event)}>
        <p className="kicker">POR CONVITE</p>
        <h1>CRIAR UMA CASA</h1>
        <p>Uma casa reúne as pessoas, regras e conversas da família.</p>
        <label htmlFor="invitation-code">código do convite<input id="invitation-code" required autoComplete="one-time-code" value={invitationCode} onChange={(event) => setInvitationCode(event.target.value)} /></label>
        <label htmlFor="house-name">nome da casa<input id="house-name" required autoComplete="organization" placeholder="ex.: Casa Silva" value={houseName} onChange={(event) => setHouseName(event.target.value)} /></label>
        <label htmlFor="profile-name">quem vai usar primeiro?<input id="profile-name" required autoComplete="off" placeholder="ex.: Lia" value={profileName} onChange={(event) => setProfileName(event.target.value)} /></label>
        <button className="button button--ink" disabled={busy}>{busy ? 'CRIANDO…' : 'CRIAR CASA'}</button>
        {error && <p className="form-error" role="alert">{error}</p>}
        <a className="login-invite" href="/painel">já tenho uma casa →</a>
      </form>
    </main>
  )
}
