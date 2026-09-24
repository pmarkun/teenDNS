import { type FormEvent, useCallback, useEffect, useState } from 'react'
import { api, type HouseSummary, type Invitation, type WaitlistEntry, getToken, setToken } from '../api'
import { Drawer, Logo } from '../components'

export function Admin() {
  const [authNeeded, setAuthNeeded] = useState(() => !getToken())

  if (authNeeded) return <OperatorLogin onSuccess={() => setAuthNeeded(false)} />
  return <AdminConsole />
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
        <h1>ADMIN</h1>
        <p>Use a chave administrativa do operador.</p>
        <label>chave <input type="password" value={value} onChange={(event) => setValue(event.target.value)} autoFocus /></label>
        <button className="button button--ink">ENTRAR</button>
      </form>
    </main>
  )
}

function AdminConsole() {
  const [prefillEmail, setPrefillEmail] = useState('')
  return (
    <main className="register">
      <Logo />
      <InviteForm email={prefillEmail} setEmail={setPrefillEmail} />
      <HousesSection />
      <WaitlistSection onInvite={setPrefillEmail} />
    </main>
  )
}

function InviteForm({ email, setEmail }: { email: string; setEmail: (value: string) => void }) {
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
    <>
      <form id="invite-form" className="register-card" onSubmit={(event) => void submit(event)}>
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
    </>
  )
}

function HousesSection() {
  const [houses, setHouses] = useState<HouseSummary[]>([])
  const [error, setError] = useState('')
  const [deleting, setDeleting] = useState<HouseSummary | null>(null)

  const load = useCallback(async () => {
    try {
      setHouses(await api.listHouses())
      setError('')
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Não foi possível carregar as casas')
    }
  }, [])

  useEffect(() => { void load() }, [load])

  return (
    <section className="register-card admin-list">
      <p className="kicker">CASAS</p>
      <h2>QUEM JÁ TEM CASA</h2>
      {error && <p className="form-error" role="alert">{error}</p>}
      {houses.length === 0 ? (
        <p>Nenhuma casa cadastrada ainda.</p>
      ) : (
        <ul className="admin-rows">
          {houses.map((house) => (
            <li key={house.id}>
              <div>
                <strong>{house.name}</strong>
                <small>{house.email || 'sem e-mail'} · {house.profile_count} perfil(is)</small>
              </div>
              <button type="button" className="button button--danger" onClick={() => setDeleting(house)}>apagar</button>
            </li>
          ))}
        </ul>
      )}
      {deleting && (
        <DeleteHouseDrawer house={deleting} onClose={() => setDeleting(null)} onDeleted={() => { setDeleting(null); void load() }} />
      )}
    </section>
  )
}

function DeleteHouseDrawer({ house, onClose, onDeleted }: { house: HouseSummary; onClose: () => void; onDeleted: () => void }) {
  const [confirmText, setConfirmText] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const matches = confirmText.trim() === house.name

  async function confirmDelete() {
    setBusy(true)
    setError('')
    try {
      await api.deleteHouse(house.id)
      onDeleted()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Não foi possível apagar')
      setBusy(false)
    }
  }

  return (
    <Drawer title={`Apagar ${house.name}?`} onClose={onClose}>
      <p>Isso remove a casa, todos os perfis e o resumo semanal acumulado. Não tem como desfazer, e o DNS da família para de funcionar na hora.</p>
      <label>digite o nome da casa para confirmar
        <input value={confirmText} onChange={(event) => setConfirmText(event.target.value)} autoFocus />
      </label>
      <button className="button button--danger" disabled={!matches || busy} onClick={() => void confirmDelete()}>
        {busy ? 'APAGANDO…' : 'APAGAR DE VEZ'}
      </button>
      {error && <p className="form-error" role="alert">{error}</p>}
    </Drawer>
  )
}

function WaitlistSection({ onInvite }: { onInvite: (email: string) => void }) {
  const [entries, setEntries] = useState<WaitlistEntry[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    api.listWaitlist().then(setEntries).catch((cause) => setError(cause instanceof Error ? cause.message : 'Não foi possível carregar a lista de espera'))
  }, [])

  function invite(email: string) {
    onInvite(email)
    document.getElementById('invite-form')?.scrollIntoView({ behavior: 'smooth' })
  }

  return (
    <section className="register-card admin-list">
      <p className="kicker">LISTA DE ESPERA</p>
      <h2>QUEM PEDIU ACESSO</h2>
      {error && <p className="form-error" role="alert">{error}</p>}
      {entries.length === 0 ? (
        <p>Ninguém na lista de espera.</p>
      ) : (
        <ul className="admin-rows">
          {entries.map((entry) => (
            <li key={entry.email}>
              <div><strong>{entry.email}</strong></div>
              <button type="button" className="button button--ink" onClick={() => invite(entry.email)}>convidar</button>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
