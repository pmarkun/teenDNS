import { useEffect } from 'react'
import { SiteFooter, SiteHeader } from '../components'

export function Como() {
  useEffect(() => {
    const id = decodeURIComponent(window.location.hash.slice(1))
    if (!id) return
    requestAnimationFrame(() => document.getElementById(id)?.scrollIntoView())
  }, [])

  return (
    <main className="story-page">
      <SiteHeader />

      <section className="story-hero" id="hero" aria-labelledby="story-title">
        <div className="story-shell">
          <h1 className="story-title" id="story-title">Cuidar da internet sem vigiar.</h1>
        </div>
      </section>

      <section className="story-section story-question" id="pergunta" aria-labelledby="question-title">
        <div className="story-shell story-copy">
          <span className="story-index" aria-hidden="true">01</span>
          <div>
            <h2 className="story-title" id="question-title">antes do site, tem uma pergunta.</h2>
            <p>antes de abrir qualquer coisa, o aparelho pergunta qual é o endereço daquele domínio. essa pergunta vai pro DNS, que responde, e só então o site carrega.</p>
            <p>o teenDNS entra bem aqui, antes do site, respondendo a mesma pergunta que qualquer DNS responderia — só que com o combinado da sua casa dentro da resposta, sem instalar nada no aparelho, sem app em segundo plano e sem forçar abertura de conexão criptografada.</p>
          </div>
        </div>
      </section>

      <section className="story-section story-profile" id="perfil" aria-labelledby="profile-title">
        <div className="story-shell story-copy">
          <span className="story-index" aria-hidden="true">02</span>
          <div>
            <h2 className="story-title" id="profile-title">por perfil, não por aparelho.</h2>
            <p>uma casa reúne pessoas, regras e conversas. cada pessoa tem um perfil, e cada perfil tem um endereço de DNS privado.</p>
            <p>cada aparelho recebe só o endereço do DNS, sem login e sem histórico. por isso trocar de aparelho não muda nada no painel, e usar o wi-fi de um amigo também não.</p>
          </div>
        </div>
      </section>

      <section className="story-section story-records" id="registro" aria-labelledby="records-title">
        <div className="story-shell">
          <h2 className="story-title" id="records-title">o que fica registrado. o que não fica.</h2>
          <p className="story-records-lead">o teenDNS guarda contagens, não um histórico de navegação.</p>
          <p className="story-records-copy">aqui não há lista de sites: o que é observado, sem ser bloqueado, vira um resumo por período pros responsáveis — e nunca uma lista de tudo que você visitou.</p>
          <div className="story-comparison">
            <section aria-labelledby="does-title">
              <h3 id="does-title">faz:</h3>
              <ul>
                <li>bloqueia por perfil</li>
                <li>cria horários</li>
                <li>usa categorias</li>
                <li>registra evento mínimo</li>
              </ul>
            </section>
            <section aria-labelledby="does-not-title">
              <h3 id="does-not-title">não faz:</h3>
              <ul>
                <li>não lê mensagens</li>
                <li>não vê páginas</li>
                <li>não intercepta HTTPS</li>
                <li>não grava conteúdo</li>
              </ul>
            </section>
          </div>
        </div>
      </section>

      <section className="story-section story-ethics" id="etica" aria-labelledby="ethics-title">
        <div className="story-shell story-copy">
          <span className="story-index" aria-hidden="true">03</span>
          <div>
            <h2 className="story-title" id="ethics-title">proteção não é vigilância.</h2>
            <p>dá pra fazer as duas coisas com a mesma tecnologia, e a diferença está no que você escolhe guardar. um app que lê mensagens e mostra um histórico completo faz uma escolha; o teenDNS faz outra, com menos dado guardado e mais motivo pra conversar sobre o que apareceu.</p>
          </div>
        </div>
      </section>

      <section className="story-section story-cta" id="cta" aria-labelledby="cta-title">
        <div className="story-shell story-cta-inner">
          <h2 className="story-title" id="cta-title">configurar leva 2 minutos, sem app.</h2>
          <a className="button button--ink" href="/painel">configurar um aparelho <span aria-hidden="true">→</span></a>
        </div>
      </section>

      <section className="story-section story-credit" id="credito" aria-label="Crédito">
        <div className="story-shell">
          <p>desenvolvido no Hackathon Reflorestando a Rede.</p>
        </div>
      </section>

      <SiteFooter />
    </main>
  )
}
