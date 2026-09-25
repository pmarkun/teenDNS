import { Arrow, CarePrinciple, SiteFooter, SiteHeader } from '../components'

export function Landing() {
  return (
    <main className="landing">
      <SiteHeader />

      <section className="hero" aria-labelledby="hero-title">
        <div className="hero-copy">
          <h1 id="hero-title"><span>CUIDAR DA INTERNET</span> <em>SEM VIGIAR.</em></h1>
          <p>O teenDNS ajuda famílias a observar, conversar e proteger — sem ler mensagens ou páginas.</p>
          <a className="button button--ink" href="/como">ENTENDER EM 1 MINUTO <Arrow /></a>
        </div>
        <figure className="network-art">
          <span>é aqui que<br />a regra entra</span>
          <img src="/assets/network-path.png" alt="O celular consulta o teenDNS, que permite ou interrompe conexões antes da internet." />
        </figure>
        <img className="hero-cat" src="/assets/zine-cat.png" alt="" />
      </section>

      <section className="choices" aria-labelledby="choices-title">
        <h2 id="choices-title">TRÊS JEITOS DE CUIDAR</h2>
        <CarePrinciple number="01" name="observar" caption="sacar os padrões" tone="acid" />
        <CarePrinciple number="02" name="conversar" caption="explicar o motivo" tone="blue" />
        <CarePrinciple number="03" name="proteger" caption="parar um risco" tone="red" />
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

      <SiteFooter />
    </main>
  )
}
