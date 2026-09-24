import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react'
import { Action, api, CatalogPackage, EventSummary, clearToken, getToken, PairingOutcome, Profile, RuleGroup, setToken, SetupInfo } from '../api'
import { Drawer, Logo } from '../components'

const wait = (milliseconds: number) => new Promise((resolve) => window.setTimeout(resolve, milliseconds))

export function Panel() {
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [packages, setPackages] = useState<CatalogPackage[]>([])
  const [selectedID, setSelectedID] = useState('')
  const [summary, setSummary] = useState<EventSummary | null>(null)
  const [drawer, setDrawer] = useState<'group' | 'packages' | 'details' | 'advanced' | 'profile' | 'setup' | null>(null)
  const [editingGroup, setEditingGroup] = useState<RuleGroup | null>(null)
  const [status, setStatus] = useState('carregando')
  const [error, setError] = useState('')
  const [authNeeded, setAuthNeeded] = useState(() => !getToken())

  const selected = useMemo(
    () => profiles.find((profile) => profile.id === selectedID) || profiles.find((profile) => !profile.disabled),
    [profiles, selectedID],
  )
  const groups = selected?.groups || []
  const activePackageCount = packages.filter((item) => groups.some((group) => group.id === item.id && group.domains.length > 0)).length

  const load = useCallback(async () => {
    try {
      setStatus('carregando')
      const [next, availablePackages] = await Promise.all([api.profiles(), api.packages()])
      setProfiles(next)
      setPackages(availablePackages)
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

  useEffect(() => {
    if (getToken()) void load()
  }, [load])
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

  function logout() {
    clearToken()
    setProfiles([])
    setSelectedID('')
    setAuthNeeded(true)
  }

  if (authNeeded) return <Login error={error} onSuccess={load} />

  return (
    <main className="panel">
      <header className="panel-header">
        <Logo />
        <nav><a href="/#como">entenda</a><a href="/meu-dns">visão jovem</a><a href="/#configurar">ajuda</a></nav>
        <div className="panel-header-actions">
          <div className={`service-status ${status !== 'tá rodando' ? 'service-status--busy' : ''}`} aria-live="polite">
            <i /> {status}
          </div>
          <button className="panel-logout" onClick={logout}>SAIR</button>
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

          <button className="setup-button" onClick={() => setDrawer('setup')}>CONFIGURAR UM APARELHO →</button>

          <div className="rules-heading"><h2>REGRAS, POR ENQUANTO</h2><span>{groups.length}</span></div>
          <div className="rule-list">
            {groups.map((group, index) => {
              const domainCount = group.domains?.length || 0
              return (
                <div className="rule-row" key={group.id}>
                  <button className="group-title" onClick={() => { setEditingGroup(group); setDrawer('group') }}>
                    <strong>{group.name}</strong>
                    <small>{domainCount} {domainCount === 1 ? 'domínio' : 'domínios'}</small>
                  </button>
                  <select name={`rule-${group.id}-action`} className={`action action--${group.action}`} value={group.action} onChange={(event) => void updateGroup(index, event.target.value as Action)} aria-label={`Ação para ${group.name}`}>
                    <option value="block">Proteger</option>
                    <option value="observe">Observar</option>
                    <option value="allow">Permitir</option>
                  </select>
                </div>
              )
            })}
          </div>
          <div className="rule-actions">
            <button onClick={() => { setEditingGroup(null); setDrawer('group') }}>+ botar outra regra</button>
            <button onClick={() => setDrawer('packages')}>pacotes prontos <small>{activePackageCount}/{packages.length}</small> →</button>
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
      {drawer === 'packages' && selected && <PackagesDrawer profile={selected} packages={packages} onClose={() => setDrawer(null)} onSaved={(saved) => setProfiles((all) => all.map((item) => item.id === saved.id ? saved : item))} />}
      {drawer === 'profile' && <ProfileDrawer onClose={() => setDrawer(null)} onCreated={(profile) => { setProfiles((all) => [...all, profile]); setSelectedID(profile.id); setDrawer(null) }} />}
      {drawer === 'details' && <DetailsDrawer summary={summary} onClose={() => setDrawer(null)} />}
      {drawer === 'advanced' && selected && <AdvancedDrawer profile={selected} onClose={() => setDrawer(null)} onRotated={(profile) => setProfiles((all) => all.map((item) => item.id === profile.id ? profile : item))} />}
      {drawer === 'setup' && selected && <SetupDrawer profile={selected} onClose={() => setDrawer(null)} />}
      {error && <button className="toast" onClick={() => setError('')}>{error} ×</button>}
    </main>
  )
}

function Login({ error, onSuccess }: { error: string; onSuccess: () => Promise<void> }) {
  const [mode, setMode] = useState<'email' | 'token'>('email')
  return (
    <main className="login">
      <Logo />
      {mode === 'email'
        ? <EmailLogin onUseToken={() => setMode('token')} />
        : <TokenLogin error={error} onSuccess={onSuccess} onBack={() => setMode('email')} />}
    </main>
  )
}

function EmailLogin({ onUseToken }: { onUseToken: () => void }) {
  const [email, setEmail] = useState('')
  const [status, setStatus] = useState<'idle' | 'sent' | 'waitlisted'>('idle')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setError('')
    try {
      const result = await api.requestMagicLink(email)
      setStatus(result.status === 'waitlisted' ? 'waitlisted' : 'sent')
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Não foi possível enviar o link')
    } finally {
      setBusy(false)
    }
  }

  if (status === 'sent') {
    return (
      <div className="login-card">
        <h1>CONFIRA SEU E-MAIL</h1>
        <p>Mandamos um link de acesso para {email}. Ele funciona por 15 minutos.</p>
      </div>
    )
  }
  if (status === 'waitlisted') {
    return (
      <div className="login-card">
        <h1>VOCÊ ENTROU NA LISTA</h1>
        <p>Ainda não achamos uma casa com esse e-mail. Avisamos assim que houver um convite.</p>
      </div>
    )
  }

  return (
    <form onSubmit={(event) => void submit(event)}>
      <h1>ABRIR O PAINEL</h1>
      <p>Informe o e-mail cadastrado na sua casa.</p>
      <label>e-mail <input type="email" value={email} onChange={(event) => setEmail(event.target.value)} autoFocus required /></label>
      <button className="button button--ink" disabled={busy}>{busy ? 'ENVIANDO…' : 'ENVIAR LINK'}</button>
      {error && <small>{error}</small>}
      <button type="button" className="login-invite" onClick={onUseToken}>ou cole sua chave administrativa →</button>
      <a className="login-invite" href="/comecar">tenho um convite →</a>
    </form>
  )
}

function TokenLogin({ error, onSuccess, onBack }: { error: string; onSuccess: () => Promise<void>; onBack: () => void }) {
  const [value, setValue] = useState('')
  function submit(event: FormEvent) {
    event.preventDefault()
    setToken(value)
    void onSuccess()
  }
  return (
    <form onSubmit={submit}>
      <h1>COLAR CHAVE</h1>
      <p>Use a chave administrativa da sua casa.</p>
      <label>chave <input type="password" value={value} onChange={(event) => setValue(event.target.value)} autoFocus /></label>
      <button className="button button--ink">ENTRAR</button>
      {error && <small>{error}</small>}
      <button type="button" className="login-invite" onClick={onBack}>← voltar para o e-mail</button>
    </form>
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

function PackagesDrawer({ profile, packages, onClose, onSaved }: { profile: Profile; packages: CatalogPackage[]; onClose: () => void; onSaved: (profile: Profile) => void }) {
  const [busyID, setBusyID] = useState('')
  const [packageError, setPackageError] = useState('')

  async function save(item: CatalogPackage, enabled: boolean, action: Action) {
    setBusyID(item.id)
    setPackageError('')
    try {
      onSaved(await api.setPackage(profile.id, item.id, enabled, action))
    } catch (cause) {
      setPackageError(cause instanceof Error ? cause.message : 'Não foi possível mudar o pacote')
    } finally {
      setBusyID('')
    }
  }

  return <Drawer title="PACOTES PRONTOS" onClose={onClose}>
    <div className="packages-intro">
      <p>Ligue só o que faz sentido para esta casa. Os domínios já vêm cuidados pelo teenDNS.</p>
      <span>{packages.length} pacotes disponíveis</span>
    </div>
    <div className="package-list">
      {packages.map((item) => {
        const group = (profile.groups || []).find((candidate) => candidate.id === item.id)
        const enabled = Boolean(group && group.domains.length > 0)
        const action = group?.action || item.suggested_action
        const busy = busyID === item.id
        return <article className={`package-row ${enabled ? 'is-enabled' : ''}`} key={item.id}>
          <div className="package-copy">
            <strong>{item.name}</strong>
            <span>{item.domain_count} {item.domain_count === 1 ? 'domínio' : 'domínios'}</span>
            <details><summary>por quê?</summary><p>{item.reason}</p></details>
          </div>
          <div className="package-controls">
            {enabled && <select name={`package-${item.id}-action`} className={`package-action package-action--${action}`} value={action} onChange={(event) => void save(item, true, event.target.value as Action)} disabled={busy} aria-label={`Ação para o pacote ${item.name}`}>
              <option value="block">Proteger</option>
              <option value="observe">Observar</option>
              <option value="allow">Permitir</option>
            </select>}
            <button className="package-toggle" disabled={busy} onClick={() => void save(item, !enabled, action)} aria-pressed={enabled}>
              {busy ? '…' : enabled ? 'DESLIGAR' : 'LIGAR'}
            </button>
          </div>
        </article>
      })}
    </div>
    {packageError && <p className="form-error" role="alert">{packageError}</p>}
  </Drawer>
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

type SetupKind = 'windows.bat' | 'windows-remove.bat' | 'apple.mobileconfig'
type TestState = 'idle' | 'running' | 'ok' | 'other' | 'timeout' | 'error'

function SetupDrawer({ profile, onClose }: { profile: Profile; onClose: () => void }) {
  const [info, setInfo] = useState<SetupInfo | null>(null)
  const [error, setError] = useState('')
  const [busyKind, setBusyKind] = useState<SetupKind | ''>('')
  const [testState, setTestState] = useState<TestState>('idle')
  const [testMessage, setTestMessage] = useState('')

  useEffect(() => {
    api.setupInfo(profile.id)
      .then(setInfo)
      .catch((cause) => setError(cause instanceof Error ? cause.message : 'Configuração indisponível'))
  }, [profile.id])

  async function download(kind: SetupKind) {
    setBusyKind(kind)
    setError('')
    try {
      const file = await api.downloadSetupFile(profile.id, kind)
      const url = URL.createObjectURL(new Blob([file.text], { type: 'text/plain' }))
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = file.filename
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
      URL.revokeObjectURL(url)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Não foi possível baixar o arquivo')
    } finally {
      setBusyKind('')
    }
  }

  async function runTest() {
    setTestState('running')
    setTestMessage('')
    setError('')
    try {
      const challenge = await api.createPairingChallenge()
      const image = new Image()
      image.referrerPolicy = 'no-referrer'
      image.src = `https://${challenge.dns_name}/check.gif?t=${Date.now()}`
      for (let count = 0; count < 30; count += 1) {
        await wait(1_500)
        const outcome: PairingOutcome = await api.pairingOutcome(challenge.id)
        if (!outcome.observed) continue
        if (outcome.profile_id === profile.id) {
          setTestState('ok')
          setTestMessage(`Configuração funcionando: um aparelho respondeu pelo DNS de ${profile.label || profile.id}.`)
        } else {
          setTestState('other')
          setTestMessage(outcome.profile_id
            ? `Outro perfil respondeu (${outcome.profile_id}). Este aparelho ainda não está usando o DNS deste perfil.`
            : 'Outro aparelho respondeu por um perfil diferente.')
        }
        return
      }
      setTestState('timeout')
      setTestMessage('Nenhum aparelho respondeu ainda. Confira se o aparelho configurou o DNS teenDNS desta casa.')
    } catch (cause) {
      setTestState('error')
      setTestMessage(cause instanceof Error ? cause.message : 'Não foi possível testar a conexão')
    }
  }

  return <Drawer title="CONFIGURAR UM APARELHO" onClose={onClose}>
    <div className="setup-intro">
      <p>Cada aparelho da casa recebe só o endereço do DNS — sem login e sem histórico. Trocar de aparelho não muda nada no painel.</p>
    </div>
    {error && <p className="form-error" role="alert">{error}</p>}

    <section className="setup-device">
      <h3>WINDOWS</h3>
      <p>Para Windows 11, versão 24H2 ou mais nova. O arquivo aplica o DNS seguro (DoT) sozinho e pede confirmação de administrador.</p>
      <div className="setup-downloads">
        <button className="button button--ink" disabled={busyKind === 'windows.bat'} onClick={() => void download('windows.bat')}>{busyKind === 'windows.bat' ? 'GERANDO…' : 'BAIXAR INSTALADOR (.bat)'}</button>
        <button className="button button--outline" disabled={busyKind === 'windows-remove.bat'} onClick={() => void download('windows-remove.bat')}>{busyKind === 'windows-remove.bat' ? 'GERANDO…' : 'BAIXAR REMOVIDOR (.bat)'}</button>
      </div>
      <small>Em versões antigas do Windows o instalador não roda — aí vale a configuração manual de <b>OUTROS</b>.</small>
    </section>

    <section className="setup-device">
      <h3>IPHONE, IPAD E MAC</h3>
      <p>Baixe o perfil e instale nos Ajustes. {info && !info.ip ? 'Como este DNS não tem endereço IP fixo público, o perfil usa só o nome do servidor.' : ''}</p>
      <div className="setup-downloads">
        <button className="button button--ink" disabled={busyKind === 'apple.mobileconfig'} onClick={() => void download('apple.mobileconfig')}>{busyKind === 'apple.mobileconfig' ? 'GERANDO…' : 'BAIXAR PERFIL (.mobileconfig)'}</button>
      </div>
      <details><summary>passo a passo</summary><ol>
        <li><b>iPhone/iPad:</b> toque em <b>Ajustes</b> → <b>Perfil baixado</b> → <b>Instalar</b>, confirmando com o código do aparelho se pedir.</li>
        <li><b>Mac:</b> abra <b>Ajustes do Sistema</b> → <b>Perfis</b> (ou <b>Preferências do Sistema</b> → <b>Perfis</b>) → <b>Instalar</b>.</li>
        <li>Pronto: o aparelho usa o DNS da casa em todas as redes.</li>
      </ol></details>
    </section>

    {info && <section className="setup-device">
      <h3>ANDROID</h3>
      <p>Não precisa instalar nada. Nas <b>Configurações</b>, procure <b>DNS privado</b>, escolha <b>Nome do host do provedor de DNS</b> e cole:</p>
      <div className="setup-copy"><code>{info.hostname}</code><button onClick={() => void navigator.clipboard.writeText(info.hostname)}>COPIAR</button></div>
      <details><summary>passo a passo</summary><ol>
        <li>Abra <b>Configurações</b> → <b>Rede e internet</b>.</li>
        <li>Toque em <b>DNS privado</b> (em alguns aparelhos fica em <b>Wi-Fi</b> → rede atual → ícone de lápis).</li>
        <li>Escolha <b>Nome do host do provedor de DNS</b> e cole o endereço acima.</li>
      </ol></details>
    </section>}

    {info && <section className="setup-device">
      <h3>OUTROS (LINUX, ROTEADOR)</h3>
      <p>Configure o servidor DNS manualmente com os valores abaixo.{info.ip ? '' : ' Este DNS não expõe IP fixo público — use só o nome do servidor se o cliente suportar.'}</p>
      <dl className="setup-values">
        <div><dt>Nome do servidor</dt><dd><code>{info.hostname}</code></dd></div>
        {info.ip && <div><dt>Endereço IP</dt><dd><code>{info.ip}</code></dd></div>}
        <div><dt>Porta</dt><dd><code>{info.port}</code></dd></div>
        <div><dt>Domínio de teste</dt><dd><code>{info.test_domain}</code></dd></div>
      </dl>
      <small>Conexão criptografada (DoT): o nome do servidor é obrigatório na maioria dos clientes.</small>
    </section>}

    <section className="setup-test">
      <h3>JÁ INSTALOU? TESTAR</h3>
      <p>O teenDNS pergunta ao próprio DNS se um aparelho desta casa está por perto. Pode levar alguns segundos.</p>
      <button className="button button--acid" disabled={testState === 'running'} onClick={() => void runTest()}>{testState === 'running' ? 'PERGUNTANDO…' : 'TESTAR CONEXÃO'}</button>
      {testState !== 'idle' && testMessage && <p className={`setup-test--${testState}`} role="status">{testMessage}</p>}
    </section>
  </Drawer>
}
