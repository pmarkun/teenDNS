import { useEffect, useState } from 'react'
import { api, getPairingToken, setPairingToken, YouthProfile } from '../api'
import { Logo } from '../components'

type ScreenState = 'pairing' | 'ready' | 'error'

const wait = (milliseconds: number) => new Promise((resolve) => window.setTimeout(resolve, milliseconds))

function askConfiguredDNS(dnsName: string) {
  const image = new Image()
  image.referrerPolicy = 'no-referrer'
  image.src = `https://${dnsName}/parear.gif?t=${Date.now()}`
}

export function Youth() {
  const [screen, setScreen] = useState<ScreenState>('pairing')
  const [profile, setProfile] = useState<YouthProfile | null>(null)
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    let cancelled = false

    async function pair() {
      setScreen('pairing')
      setProfile(null)
      const existingToken = getPairingToken()
      if (existingToken) {
        try {
          const existingProfile = await api.youthProfile(existingToken)
          if (!cancelled) {
            setProfile(existingProfile)
            setScreen('ready')
          }
          return
        } catch {
          setPairingToken('')
        }
      }

      try {
        const challenge = await api.createPairingChallenge()
        askConfiguredDNS(challenge.dns_name)
        for (let count = 0; count < 45 && !cancelled; count += 1) {
          await wait(1_000)
          const status = await api.pairingStatus(challenge.id)
          if (!status.paired || !status.session_token) continue
          setPairingToken(status.session_token)
          const pairedProfile = await api.youthProfile(status.session_token)
          if (!cancelled) {
            setProfile(pairedProfile)
            setScreen('ready')
          }
          return
        }
        if (!cancelled) setScreen('error')
      } catch {
        if (!cancelled) setScreen('error')
      }
    }

    void pair()
    return () => { cancelled = true }
  }, [attempt])

  function retry() {
    setAttempt((current) => current + 1)
  }

  return (
    <main className="youth">
      <header className="youth-header">
        <Logo />
        <span>seu lado do combinado</span>
        <a href="/como">como funciona</a>
      </header>

      {screen === 'pairing' && <PairingState />}
      {screen === 'error' && <PairingError onRetry={retry} />}
      {screen === 'ready' && profile && <Rules profile={profile} />}
    </main>
  )
}

function PairingState() {
  return <section className="pairing-state" aria-live="polite">
    <p className="kicker">SEM LOGIN. SEM SENHA.</p>
    <h1>PERGUNTANDO<br />PRO DNS<span>...</span></h1>
    <p>Esta página está descobrindo qual perfil teenDNS está ativo neste aparelho.</p>
    <div className="pairing-steps" aria-label="Pareamento em andamento">
      <span className="is-done">1 abriu</span>
      <span className="is-live">2 ouvindo o DNS</span>
      <span>3 mostrando as regras</span>
    </div>
    <small>Isso identifica o perfil da conexão. Não identifica você e não abre seu histórico.</small>
  </section>
}

function PairingError({ onRetry }: { onRetry: () => void }) {
  return <section className="pairing-state pairing-state--error">
    <p className="kicker">NÃO DEU MATCH.</p>
    <h1>O DNS NÃO<br />RESPONDEU.</h1>
    <p>Confira se este aparelho está usando o endereço teenDNS da sua casa.</p>
    <button className="button button--ink" onClick={onRetry}>TENTAR DE NOVO</button>
    <a href="/#configurar">ver como configurar</a>
  </section>
}

function Rules({ profile }: { profile: YouthProfile }) {
  return <>
    <section className="youth-intro">
      <p className="kicker">PERFIL {profile.label.toUpperCase()}</p>
      <h1>O QUE TÁ<br /><em>PROTEGIDO</em> AQUI.</h1>
      <p>Sem lista de sites aqui. O que é observado (não bloqueado) vira um resumo por período pros responsáveis — não uma lista de tudo que você visitou.</p>
    </section>

    <section className="youth-rules" aria-labelledby="youth-rules-title">
      <div className="youth-rules-heading">
        <h2 id="youth-rules-title">AS REGRAS DO JOGO</h2>
        <span>{profile.rules.length}</span>
      </div>
      {profile.rules.length === 0
        ? <p className="youth-empty">Nada bloqueado por enquanto.</p>
        : profile.rules.map((rule, index) => <article className="youth-rule" key={`${rule.name}-${index}`}>
          <b>{String(index + 1).padStart(2, '0')}</b>
          <div><h3>{rule.name}</h3><p>{rule.reason || 'Esta proteção faz parte do combinado da família.'}</p></div>
          <span>PROTEGIDO</span>
        </article>)}
    </section>

    <footer className="youth-footer">
      <strong>REGRA BOA DÁ PRA EXPLICAR.</strong>
      <p>Achou alguma estranha? Leva pra conversa. Classificações podem errar.</p>
    </footer>
  </>
}
