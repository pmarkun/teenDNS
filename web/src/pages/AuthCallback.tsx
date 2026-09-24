import { useState } from 'react'
import { api, setToken } from '../api'
import { Logo } from '../components'

// This page deliberately does NOT exchange the token on mount. Email
// providers' security scanners often GET links automatically before a
// person opens the message; if that alone consumed the one-time login
// token, the real click would fail. Requiring an explicit click here means
// a scanner fetching the page never touches the API.
export function AuthCallback() {
  const token = new URLSearchParams(window.location.search).get('token') || ''
  const [status, setStatus] = useState<'idle' | 'busy' | 'error'>('idle')
  const [error, setError] = useState('')

  async function enter() {
    setStatus('busy')
    setError('')
    try {
      const session = await api.exchangeMagicLink(token)
      setToken(session.session_token)
      window.location.href = '/painel'
    } catch (cause) {
      setStatus('error')
      setError(cause instanceof Error ? cause.message : 'Não foi possível entrar')
    }
  }

  return (
    <main className="login">
      <Logo />
      <div className="login-card">
        {!token ? (
          <>
            <h1>LINK INCOMPLETO</h1>
            <p>Este link não tem um código de acesso. Peça um novo no painel.</p>
          </>
        ) : status === 'error' ? (
          <>
            <h1>LINK INVÁLIDO</h1>
            <p>{error || 'Esse link já foi usado ou expirou.'}</p>
            <a className="login-invite" href="/painel">pedir um novo link →</a>
          </>
        ) : (
          <>
            <h1>ENTRAR NO PAINEL</h1>
            <p>Confirma que foi você quem pediu o acesso.</p>
            <button className="button button--ink" disabled={status === 'busy'} onClick={() => void enter()}>
              {status === 'busy' ? 'ENTRANDO…' : 'ENTRAR NO PAINEL'}
            </button>
          </>
        )}
      </div>
    </main>
  )
}
