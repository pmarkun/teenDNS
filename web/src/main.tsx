import '@fontsource/barlow-condensed/latin-700.css'
import '@fontsource/barlow-condensed/latin-800.css'
import '@fontsource/barlow-condensed/latin-900.css'
import '@fontsource/inter/latin-400.css'
import '@fontsource/inter/latin-600.css'
import '@fontsource/inter/latin-700.css'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { Landing } from './pages/Landing'
import { Panel } from './pages/Panel'
import './styles.css'

const isPanel = window.location.pathname.startsWith('/painel')

createRoot(document.getElementById('root')!).render(
  <StrictMode>{isPanel ? <Panel /> : <Landing />}</StrictMode>,
)
