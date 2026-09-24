import '@fontsource/barlow-condensed/latin-700.css'
import '@fontsource/barlow-condensed/latin-800.css'
import '@fontsource/barlow-condensed/latin-900.css'
import '@fontsource/inter/latin-400.css'
import '@fontsource/inter/latin-600.css'
import '@fontsource/inter/latin-700.css'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { Invite } from './pages/Invite'
import { Landing } from './pages/Landing'
import { Panel } from './pages/Panel'
import { Register } from './pages/Register'
import { Youth } from './pages/Youth'
import './styles.css'

const isPanel = window.location.pathname.startsWith('/painel')
const isYouth = window.location.pathname.startsWith('/meu-dns')
const isRegister = window.location.pathname.startsWith('/comecar')
const isInvite = window.location.pathname.startsWith('/convidar')

createRoot(document.getElementById('root')!).render(
  <StrictMode>{isPanel ? <Panel /> : isYouth ? <Youth /> : isRegister ? <Register /> : isInvite ? <Invite /> : <Landing />}</StrictMode>,
)
