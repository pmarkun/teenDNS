import { type FormEvent, useState } from 'react'
import { api, type Invitation, getToken, setToken } from '../api'
import { Logo } from '../components'

export function Invite() {
  const [authNeeded, setAuthNeeded] = useState(() => !getToken())

  if (authNeeded) return <OperatorLogin onSuccess={() => setAuthNeeded(false)} />
  return <InviteForm />
}

function OperatorLogin({ onSuccess }: { onSuccess: () => void }) {
  const [value, setValue] = useState('')
  function submit(event: FormEvent) {
    event.preventDefault()
    setToken(value)
    onSuccess()
  }
  return (
    <main className="login">
      <Logo />
      <form onSubmit={submit}>
        <h1>GERAR CONVITE</h1>
        <p>Use a chave administrativa do operador.</p>
        <label>chave <input type="password" value={value} onChange={(event) => setValue(event.target.value)} autoFocus /></label>
        <button className="button button--ink">ENTRAR</button>
      </form>
    </main>
  )
}

function InviteForm() {
  const [email, setEmail] = useState('')
  const [created, setCreated] = useState<Invitation | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setError('')
    setCreated(null)
    try {
      const invitation = await api.createInvitation(email)
      setCreated(invitation)
      setEmail('')
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Não foi possível gerar o convite')
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="register">
      <Logo />
      <form className="register-card" onSubmit={(event) => void submit(event)}>
        <p className="kicker">NOVO CONVITE</p>
        <h1>CONVIDAR UMA FAMÍLIA</h1>
        <p>Envia um link de uso único para o responsável criar a casa.</p>
        <label htmlFor="invite-email">e-mail do responsável
          <input id="invite-email" type="email" required autoComplete="email" placeholder="ex.: responsavel@exemplo.com" value={email} onChange={(event) => setEmail(event.target.value)} />
        </label>
        <button className="button button--ink" disabled={busy}>{busy ? 'ENVIANDO…' : 'ENVIAR CONVITE'}</button>
        {error && <p className="form-error" role="alert">{error}</p>}
      </form>

      {created && (
        <section className="register-card">
          <p className="kicker">{created.email_sent ? 'CONVITE ENVIADO' : 'CONVITE CRIADO, E-MAIL FALHOU'}</p>
          <p>{created.email}</p>
          {!created.email_sent && <p className="form-error" role="alert">Não deu para mandar o e-mail. Copie o link abaixo e envie manualmente.</p>}
          <div className="secret-card">
            <code>{created.link}</code>
            <button type="button" onClick={() => void navigator.clipboard.writeText(created.link)}>COPIAR LINK</button>
          </div>
        </section>
      )}
    </main>
  )
}
