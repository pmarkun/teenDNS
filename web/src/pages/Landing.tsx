import { Arrow, Logo } from '../components'

export function Landing() {
  return (
    <main className="landing">
      <header className="site-header">
        <Logo />
        <nav aria-label="Navegação principal">
          <a href="#como">como funciona</a>
          <a href="#configurar">configurar</a>
          <a className="youth-nav" href="/meu-dns">minhas regras</a>
          <a className="button button--acid" href="/painel">ABRIR PAINEL</a>
        </nav>
      </header>

      <section className="hero" aria-labelledby="hero-title">
        <div className="hero-copy">
          <h1 id="hero-title"><span>CUIDAR DA INTERNET</span> <em>SEM VIGIAR.</em></h1>
          <p>O teenDNS ajuda famílias a observar, conversar e proteger — sem ler mensagens ou páginas.</p>
          <a className="button button--ink" href="#como">ENTENDER EM 1 MINUTO <Arrow /></a>
        </div>
        <figure className="network-art">
          <span>é aqui que<br />a regra entra</span>
          <img src="/assets/network-path.png" alt="O celular consulta o teenDNS, que permite ou interrompe conexões antes da internet." />
        </figure>
        <img className="hero-cat" src="/assets/zine-cat.png" alt="" />
      </section>

      <section className="choices" id="como" aria-labelledby="choices-title">
        <h2 id="choices-title">TRÊS JEITOS DE CUIDAR</h2>
        <div className="choice choice--acid"><b>01</b><div><h3>OBSERVAR</h3><p>sacar os padrões</p></div></div>
        <div className="choice choice--blue"><b>02</b><div><h3>CONVERSAR</h3><p>explicar o motivo</p></div></div>
        <div className="choice choice--red"><b>03</b><div><h3>PROTEGER</h3><p>parar um risco</p></div></div>
      </section>

      <section className="setup" id="configurar" aria-labelledby="setup-title">
        <div>
          <h2 id="setup-title">NO ANDROID. SEM INSTALAR NADA.</h2>
          <ol>
            <li><b>1</b> abre DNS privado</li>
            <li><b>2</b> cola seu endereço</li>
            <li><b>3</b> salva</li>
          </ol>
          <a className="button button--outline-light" href="/painel">VER PASSO A PASSO <Arrow /></a>
        </div>
        <p className="manifesto">menos vigia.<br /><strong>mais vida.</strong></p>
      </section>

      <footer className="site-footer">
        <Logo />
        <p>internet melhor. gente real.</p>
      </footer>
    </main>
  )
}
