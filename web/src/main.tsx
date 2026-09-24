import '@fontsource/barlow-condensed/latin-700.css'
import '@fontsource/barlow-condensed/latin-800.css'
import '@fontsource/barlow-condensed/latin-900.css'
import '@fontsource/inter/latin-400.css'
import '@fontsource/inter/latin-600.css'
import '@fontsource/inter/latin-700.css'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { Admin } from './pages/Admin'
import { AuthCallback } from './pages/AuthCallback'
import { Landing } from './pages/Landing'
import { Panel } from './pages/Panel'
import { Register } from './pages/Register'
import { Youth } from './pages/Youth'
import './styles.css'

const isPanel = window.location.pathname.startsWith('/painel')
const isYouth = window.location.pathname.startsWith('/meu-dns')
const isRegister = window.location.pathname.startsWith('/comecar')
const isAdmin = window.location.pathname.startsWith('/admin')
const isAuthCallback = window.location.pathname.startsWith('/entrar')

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    {isPanel ? <Panel />
      : isYouth ? <Youth />
      : isRegister ? <Register />
      : isAdmin ? <Admin />
      : isAuthCallback ? <AuthCallback />
      : <Landing />}
  </StrictMode>,
)
