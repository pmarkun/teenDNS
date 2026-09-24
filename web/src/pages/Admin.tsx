import { type FormEvent, useCallback, useEffect, useState } from 'react'
import { api, type HouseSummary, type Invitation, type WaitlistEntry, clearToken, setToken } from '../api'
import { Drawer, Logo } from '../components'

export function Admin() {
  const [state, setState] = useState<'checking' | 'login' | 'console'>('checking')

  useEffect(() => {
    let cancelled = false
    void api.adminGuard().then((ok) => {
      if (!cancelled) setState(ok ? 'console' : 'login')
    })
    return () => { cancelled = true }
  }, [])

  if (state === 'checking') return <main className="login"><Logo /></main>
  if (state === 'login') return <OperatorLogin onSuccess={() => setState('console')} />
  return <AdminConsole />
}

function OperatorLogin({ onSuccess }: { onSuccess: () => void }) {
  const [value, setValue] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setError('')
    setToken(value)
    const ok = await api.adminGuard()
    if (ok) onSuccess()
    else {
      clearToken()
      setError('Chave inválida. Tente de novo.')
    }
    setBusy(false)
    setValue('')
  }
  return (
    <main className="login">
      <Logo />
      <form onSubmit={(event) => void submit(event)}>
        <h1>ADMIN</h1>
        <p>Use a chave administrativa do operador.</p>
        <label>chave <input type="password" value={value} onChange={(event) => setValue(event.target.value)} autoFocus /></label>
        <button className="button button--ink" disabled={busy}>{busy ? 'CONFERINDO…' : 'ENTRAR'}</button>
        {error && <p className="form-error" role="alert">{error}</p>}
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
  const [editingEmails, setEditingEmails] = useState<HouseSummary | null>(null)

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
                <small>{((house.emails || []).length > 0 ? house.emails.join(' · ') : 'sem e-mails')} · {house.profile_count} perfil(is)</small>
              </div>
              <div className="admin-row-actions">
                <button type="button" className="button button--ink" onClick={() => setEditingEmails(house)}>e-mails</button>
                <button type="button" className="button button--danger" onClick={() => setDeleting(house)}>apagar</button>
              </div>
            </li>
          ))}
        </ul>
      )}
      {deleting && (
        <DeleteHouseDrawer house={deleting} onClose={() => setDeleting(null)} onDeleted={() => { setDeleting(null); void load() }} />
      )}
      {editingEmails && (
        <EmailsDrawer
          house={editingEmails}
          onClose={() => setEditingEmails(null)}
          onSaved={(updated) => {
            setHouses((current) => current.map((item) => item.id === updated.id ? { ...item, email: updated.email, emails: updated.emails } : item))
            setEditingEmails(null)
          }}
        />
      )}
    </section>
  )
}

function EmailsDrawer({ house, onClose, onSaved }: { house: HouseSummary; onClose: () => void; onSaved: (updated: { id: string; email?: string; emails: string[] }) => void }) {
  const [value, setValue] = useState((house.emails || []).join('\n'))
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setError('')
    try {
      const emails = [...new Set(value.split(/[\n,]/).map((email) => email.trim()).filter(Boolean))]
      onSaved(await api.updateHouseEmails(house.id, emails))
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Não foi possível salvar os e-mails')
      setBusy(false)
    }
  }

  return (
    <Drawer title={`E-MAILS DE ${house.name.toUpperCase()}`} onClose={onClose}>
      <form className="drawer-form" onSubmit={(event) => void submit(event)}>
        <p>Quem recebe o link de acesso do painel desta casa. O primeiro e-mail também recebe o resumo semanal.</p>
        <label>e-mails <small>um por linha</small><textarea className="domain-list" required placeholder={'responsavel@exemplo.com\noutro-responsavel@exemplo.com'} value={value} onChange={(event) => setValue(event.target.value)} /></label>
        <div className="drawer-actions">
          <button className="button button--ink" disabled={busy}>{busy ? 'SALVANDO…' : 'SALVAR E-MAILS'}</button>
        </div>
        {error && <p className="form-error" role="alert">{error}</p>}
      </form>
    </Drawer>
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
