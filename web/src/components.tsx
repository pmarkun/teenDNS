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
