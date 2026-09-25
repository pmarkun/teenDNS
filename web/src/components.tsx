import type { ReactNode } from 'react'

export function Logo({ light = false }: { light?: boolean }) {
  return (
    <a className={`logo ${light ? 'logo--light' : ''}`} href="/" aria-label="teenDNS, página inicial">
      teen<span>DNS</span>
      <small>a rede é nossa.</small>
    </a>
  )
}

export function Arrow() {
  return (
    <svg className="arrow" viewBox="0 0 28 16" aria-hidden="true">
      <path d="M1 8h24M18 1l7 7-7 7" />
    </svg>
  )
}

export function SiteHeader() {
  return (
    <header className="site-header">
      <Logo />
      <nav aria-label="Navegação principal">
        <a href="/como">como funciona</a>
        <a href="/#configurar">configurar</a>
        <a className="youth-nav" href="/meu-dns">minhas regras</a>
        <a className="button button--acid" href="/painel">ABRIR PAINEL</a>
      </nav>
    </header>
  )
}

export function SiteFooter() {
  return (
    <footer className="site-footer">
      <Logo />
    </footer>
  )
}

export function CarePrinciple({ number, name, caption, tone }: {
  number: string
  name: string
  caption: string
  tone: 'acid' | 'blue' | 'red'
}) {
  return (
    <div className={`choice choice--${tone}`}>
      <b>{number}</b>
      <div><h3>{name}</h3><p>{caption}</p></div>
    </div>
  )
}

export function Drawer({ title, onClose, children }: { title: string; onClose: () => void; children: ReactNode }) {
  return (
    <div className="drawer-backdrop" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <section className="drawer" role="dialog" aria-modal="true" aria-labelledby="drawer-title">
        <header>
          <h2 id="drawer-title">{title}</h2>
          <button className="icon-button" onClick={onClose} aria-label="Fechar">×</button>
        </header>
        {children}
      </section>
    </div>
  )
}
