import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react'
import { Action, api, EventSummary, getToken, Profile, RuleGroup, setToken } from '../api'
import { Drawer, Logo } from '../components'

export function Panel() {
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [selectedID, setSelectedID] = useState('')
  const [summary, setSummary] = useState<EventSummary | null>(null)
  const [drawer, setDrawer] = useState<'group' | 'details' | 'advanced' | 'profile' | null>(null)
  const [editingGroup, setEditingGroup] = useState<RuleGroup | null>(null)
  const [status, setStatus] = useState('carregando')
  const [error, setError] = useState('')
  const [authNeeded, setAuthNeeded] = useState(!getToken())

  const selected = useMemo(
    () => profiles.find((profile) => profile.id === selectedID) || profiles.find((profile) => !profile.disabled),
    [profiles, selectedID],
  )
  const groups = selected?.groups || []

  const load = useCallback(async () => {
    try {
      setStatus('carregando')
      const next = await api.profiles()
      setProfiles(next)
      const active = next.find((profile) => !profile.disabled)
      setSelectedID((current) => current || active?.id || '')
      setAuthNeeded(false)
      setError('')
      setStatus('tá rodando')
    } catch (reason) {
      setStatus('fora do ar')
      setAuthNeeded(true)
      setError(reason instanceof Error ? reason.message : 'Não foi possível abrir o painel')
    }
  }, [])

  useEffect(() => { void load() }, [load])
  useEffect(() => {
    if (!selected) return
    void api.summary(selected.id).then(setSummary).catch(() => setSummary(null))
  }, [selected])

  async function updateGroup(index: number, action: Action) {
    if (!selected) return
    const optimistic = { ...selected, groups: groups.map((group, position) => position === index ? { ...group, action } : group) }
    setProfiles((current) => current.map((profile) => profile.id === selected.id ? optimistic : profile))
    setStatus('salvando')
    try {
      const saved = await api.updateProfile(optimistic)
      setProfiles((current) => current.map((profile) => profile.id === saved.id ? saved : profile))
      setStatus('tá rodando')
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Não foi possível salvar')
      setStatus('erro ao salvar')
      await load()
    }
  }

  if (authNeeded) return <Login error={error} onSuccess={load} />

  return (
    <main className="panel">
      <header className="panel-header">
        <Logo />
        <nav><a href="/#como">entenda</a><a href="/#configurar">ajuda</a></nav>
        <div className={`service-status ${status !== 'tá rodando' ? 'service-status--busy' : ''}`} aria-live="polite">
          <i /> {status}
        </div>
      </header>

      <aside className="profiles" aria-label="Perfis">
        <h2>quem usa?</h2>
        {profiles.filter((profile) => !profile.disabled).map((profile) => (
          <button key={profile.id} className={profile.id === selected?.id ? 'is-selected' : ''} onClick={() => setSelectedID(profile.id)}>
            {profile.label || profile.id}
          </button>
        ))}
        <button className="profile-add" onClick={() => setDrawer('profile')}>+ novo</button>
        <img className="panel-cat" src="/assets/zine-cat.png" alt="" />
        <p>internet melhor.<br />gente real.</p>
      </aside>

      {selected ? (
        <section className="rules-area">
          <h1>{(selected.label || selected.id).toUpperCase()}</h1>
          <p className="rules-intro">Escolha o que observar e o que proteger.</p>

          <div className="endpoint">
            <b>DNS privado</b>
            <code>{selected.hostname}</code>
            <button onClick={() => void navigator.clipboard.writeText(selected.hostname)}>COPIAR</button>
          </div>

          <div className="rules-heading"><h2>REGRAS, POR ENQUANTO</h2><span>{groups.length}</span></div>
          <div className="rule-list">
            {groups.map((group, index) => (
              <div className="rule-row" key={group.id}>
                <button className="group-title" onClick={() => { setEditingGroup(group); setDrawer('group') }}>
                  <strong>{group.name}</strong>
                  <small>{group.domains.length} {group.domains.length === 1 ? 'domínio' : 'domínios'}</small>
                </button>
                <select className={`action action--${group.action}`} value={group.action} onChange={(event) => void updateGroup(index, event.target.value as Action)} aria-label={`Ação para ${group.name}`}>
                  <option value="block">Proteger</option>
                  <option value="observe">Observar</option>
                  <option value="allow">Permitir</option>
                </select>
              </div>
            ))}
          </div>
          <div className="rule-actions">
            <button onClick={() => { setEditingGroup(null); setDrawer('group') }}>+ botar outra regra</button>
            <button onClick={() => setDrawer('advanced')}>mexer nas partes nerds →</button>
          </div>
        </section>
      ) : <section className="rules-area"><h1>NENHUM PERFIL</h1><button className="button button--ink" onClick={() => setDrawer('profile')}>CRIAR O PRIMEIRO</button></section>}

      <aside className="today">
        <h2>HOJE</h2>
        <p><b>{summary?.blocked || 0}</b> proteções</p>
        <p><b>{summary?.observed || 0}</b> observações</p>
        <button onClick={() => setDrawer('details')}>ver o rolê</button>
        <blockquote>regra boa é regra que dá pra conversar.</blockquote>
      </aside>

      {drawer === 'group' && selected && <GroupDrawer profile={selected} group={editingGroup} onClose={() => setDrawer(null)} onSaved={(saved) => { setProfiles((all) => all.map((item) => item.id === saved.id ? saved : item)); setDrawer(null) }} />}
      {drawer === 'profile' && <ProfileDrawer onClose={() => setDrawer(null)} onCreated={(profile) => { setProfiles((all) => [...all, profile]); setSelectedID(profile.id); setDrawer(null) }} />}
      {drawer === 'details' && <DetailsDrawer summary={summary} onClose={() => setDrawer(null)} />}
      {drawer === 'advanced' && selected && <AdvancedDrawer profile={selected} onClose={() => setDrawer(null)} onRotated={(profile) => setProfiles((all) => all.map((item) => item.id === profile.id ? profile : item))} />}
      {error && <button className="toast" onClick={() => setError('')}>{error} ×</button>}
    </main>
  )
}

function Login({ error, onSuccess }: { error: string; onSuccess: () => Promise<void> }) {
  const [value, setValue] = useState('')
  function submit(event: FormEvent) {
    event.preventDefault()
    setToken(value)
    void onSuccess()
  }
  return (
    <main className="login">
      <Logo />
      <form onSubmit={submit}>
        <h1>ABRIR O PAINEL</h1>
        <p>Use a chave administrativa da sua casa.</p>
        <label>chave <input type="password" value={value} onChange={(event) => setValue(event.target.value)} autoFocus /></label>
        <button className="button button--ink">ENTRAR</button>
        {error && <small>{error}</small>}
      </form>
    </main>
  )
}

function parseDomains(value: string) {
  return [...new Set(value.split(/[\n,]/).map((domain) => domain.trim().toLowerCase()).filter(Boolean))]
}

function GroupDrawer({ profile, group, onClose, onSaved }: { profile: Profile; group: RuleGroup | null; onClose: () => void; onSaved: (profile: Profile) => void }) {
  const [domainsText, setDomainsText] = useState((group?.domains || []).join('\n'))
  const [name, setName] = useState(group?.name || '')
  const [action, setAction] = useState<Action>(group?.action || 'block')
  const [reason, setReason] = useState(group?.reason || '')
  const [customized, setCustomized] = useState(group?.customized || false)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState('')
  const domains = parseDomains(domainsText)
  const needsName = Boolean(group) || domains.length > 1
  const defaults = group?.default_domains || []

  async function submit(event: FormEvent) {
    event.preventDefault()
    setSaving(true)
    setSaveError('')
    try {
      const nextGroup: RuleGroup = {
        id: group?.id || `custom-${crypto.randomUUID()}`,
        name: needsName ? name.trim() : domains[0],
        action,
        reason,
        category: group?.category,
        domains,
        default_domains: defaults,
        domain_source: group?.domain_source,
        customized: group ? customized : true,
      }
      const nextGroups = group
        ? (profile.groups || []).map((item) => item.id === group.id ? nextGroup : item)
        : [...(profile.groups || []), nextGroup]
      const saved = await api.updateProfile({ ...profile, groups: nextGroups })
      onSaved(saved)
    } catch (cause) {
      setSaveError(cause instanceof Error ? cause.message : 'Não foi possível salvar a regra')
      setSaving(false)
    }
  }
  function restoreDefaults() {
    setDomainsText(defaults.join('\n'))
    setCustomized(false)
  }

  return <Drawer title={group ? `EDITAR ${group.name.toUpperCase()}` : 'BOTAR OUTRA REGRA'} onClose={onClose}><form className="drawer-form" onSubmit={(event) => void submit(event)}>
    <label>domínios <small>{domains.length} {domains.length === 1 ? 'domínio' : 'domínios'}</small><textarea className="domain-list" required placeholder={'exemplo.com\noutro-exemplo.com'} value={domainsText} onChange={(event) => { setDomainsText(event.target.value); setCustomized(true) }} /><small>um por linha — você também pode colar uma lista</small></label>
    {needsName && <label>nome da regra<input required placeholder="ex.: Jogos e apostas" value={name} onChange={(event) => setName(event.target.value)} /></label>}
    <label>o que fazer<select value={action} onChange={(event) => setAction(event.target.value as Action)}><option value="block">Proteger</option><option value="observe">Observar</option><option value="allow">Permitir</option></select></label>
    <label>por quê?<textarea required placeholder="Um motivo que faça sentido para a família." value={reason} onChange={(event) => setReason(event.target.value)} /></label>
    <div className="drawer-actions">
      <button className="button button--ink" disabled={saving || domains.length === 0 || (needsName && !name.trim())}>{saving ? 'SALVANDO…' : 'SALVAR REGRA'}</button>
      {defaults.length > 0 && <button type="button" className="restore-button" onClick={restoreDefaults}>RESTAURAR PADRÃO <small>{defaults.length} domínios</small></button>}
    </div>
    {saveError && <p className="form-error" role="alert">{saveError}</p>}
  </form></Drawer>
}

function ProfileDrawer({ onClose, onCreated }: { onClose: () => void; onCreated: (profile: Profile) => void }) {
  const [label, setLabel] = useState('')
  async function submit(event: FormEvent) { event.preventDefault(); onCreated(await api.createProfile(label)) }
  return <Drawer title="QUEM VAI USAR?" onClose={onClose}><form className="drawer-form" onSubmit={(event) => void submit(event)}><label>nome do perfil<input required value={label} onChange={(event) => setLabel(event.target.value)} placeholder="Casa, estudos…" /></label><button className="button button--ink">CRIAR PERFIL</button></form></Drawer>
}

function DetailsDrawer({ summary, onClose }: { summary: EventSummary | null; onClose: () => void }) {
  return <Drawer title="O ROLÊ DE HOJE" onClose={onClose}><div className="details"><p>O teenDNS guarda contagens, não um histórico de navegação.</p><dl><div><dt>Proteções</dt><dd>{summary?.blocked || 0}</dd></div><div><dt>Observações</dt><dd>{summary?.observed || 0}</dd></div><div><dt>Sem categoria</dt><dd>{summary?.unknown || 0}</dd></div></dl></div></Drawer>
}

function AdvancedDrawer({ profile, onClose, onRotated }: { profile: Profile; onClose: () => void; onRotated: (profile: Profile) => void }) {
  const [busy, setBusy] = useState(false)
  async function rotate() { setBusy(true); onRotated(await api.rotate(profile.id)); setBusy(false) }
  return <Drawer title="PARTES NERDS" onClose={onClose}><div className="details"><p>Troque o endereço apenas se ele tiver sido compartilhado sem querer. Depois disso, o endereço anterior para de funcionar.</p><code>{profile.hostname}</code><button className="button button--danger" disabled={busy} onClick={() => void rotate()}>{busy ? 'TROCANDO…' : 'GERAR OUTRO ENDEREÇO'}</button></div></Drawer>
}
