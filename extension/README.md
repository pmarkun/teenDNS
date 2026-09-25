# teenDNS — extensão Chrome (MVP)

Extensão MV3 sem passo de build (JS puro, service worker ES module) que:

- **pareia em um clique** reusando o fluxo de desafio da visão jovem: cria um
  desafio na API do painel, resolve o host do desafio (o resolvedor da casa
  responde) e troca a sessão por um snapshot de política;
- **espelha a política do perfil localmente** (`chrome.storage`) — regras,
  grupos, pausas e o fuse horário da casa — e a re-sincroniza sozinha
  (o token de sessão dura 1h, então renovamos o pareamento automaticamente);
- **bloqueia em nível de navegador** com `declarativeNetRequest` em *session
  rules*: a ação padrão, regras e grupos (incluindo `allow` e `observe`) são
  compilados preservando a maior especificidade de domínio e a ordem de empate
  do resolvedor; a pausa geral vence tudo;
- **mostra a cor por aba**: vermelho = bloqueado, azul = observado,
  limão = liberado (badge no ícone);
- **não envia telemetria**: nenhum evento de block incrementa os contadores do
  painel; a extensão só lê e aplica a política.

## Arquitetura

```
manifest.json   MV3, permissões storage/alarms/declarativeNetRequest, <all_urls>
background.js   SW: pareamento, sync periódico, rebuild das regras, badges
decision.js     porta fiel de internal/policy.DecideAt (sem chrome.*, testável)
rules.js        constrói as session rules DNR a partir do snapshot (puro)
popup.*         emparelhar/desvincular/endpoint, estado e erros
blocked.*       página de bloqueio (web_accessible), busca motivo no SW
decision.test.mjs  testes node (espelham internal/policy/policy_test.go)
icons/          ícones PNG em quatro tamanhos para a barra e a loja
```

### Decisões de implementação

- **Redirect `extensionPath`** (documentado) → `blocked.html` sem query string.
  A página busca categoria/motivo/grupo no service worker por `tabId`
  (`recentBlocked` grava a navegação que precedeu o redirect); assim o
  redirect fica 100% dentro do contrato documentado da API.
- **Regras por grupo**: domínios de um grupo com o mesmo tamanho são agrupados
  em uma regra `requestDomains` (match por sufixo, subdomínios inclusos). A
  prioridade DNR codifica tamanho do domínio e ordem do resolvedor; regras
  exatas usam uma regex de host ancorada, sem capturar subdomínios.
- **Agendas**: janelas de horário não cabem em condições DNR estáticas, então
  um alarm de 1 minuto reavalia a *superfície de bloqueio efetiva* e reaplica as
  session rules (guardado por assinatura para evitar churn).
- **Relação com o DNS**: a extensão replica a decisão principalmente para
  *main_frame*. O resolvedor da casa decide o resto (subrecursos, apps). O
  bloqueio total de apps com VPN/DoH próprios não é prometido (ver AGENTS/README
  raiz).

## Desenvolvimento

Ambiente e testes (veja AGENTS.md raiz):

```sh
nix develop --command node --test extension/decision.test.mjs
```

Carregar a extensão não empacotada em Chrome: `chrome://extensions` →
"Modo de desenvolvedor" → "Carregar sem compactação" → pasta `extension/`.

## Limites do MVP

- Sobreposição de grupos: primeiro grupo arbitrário vence no DNR (o DNS faz
  longest-match); aceito para o MVP.
- `blocked.html` consulta a explicação da navegação pendente na memória de
  sessão do service worker (`chrome.storage.session`); se o Chrome não emitir o
  evento de navegação original, a página cai no texto genérico.
- Permissão `host_permissions: <all_urls>` é necessária para redirect + leitura
  de URL das abas — decisão explícita do usuário.
