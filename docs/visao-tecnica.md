# teenDNS — visão técnica

Este documento reúne arquitetura, componentes e comandos para executar e validar
o MVP. A apresentação do problema, dos princípios e da origem do projeto está no
[README](../README.md).

## Arquitetura em resumo

```text
aparelho ── DoT + hostname do perfil ──► gateway teenDNS
                                           │
                              ┌────────────┴────────────┐
                              ▼                         ▼
                    resposta protegida              Unbound
                                                     (resolução)

painel web ──► API administrativa ──► estado da casa e políticas
```

O gateway recebe conexões DNS-over-TLS (DoT). O hostname do perfil, enviado
como SNI, seleciona a política daquela pessoa. Também recebe DNS-over-HTTPS
(DoH, RFC 8484) por GET ou POST no caminho HTTPS
`/dns-query/{rótulo-opaco-p-…}`; o Nginx encaminha esse caminho ao mesmo
motor de política. Consultas permitidas seguem para o Unbound; as protegidas
recebem resposta negativa. Alterações feitas pela API administrativa entram em
vigor sem reiniciar o gateway.

Cada casa tem um fuso IANA, inicialmente `America/Sao_Paulo`. Janelas semanais
por grupo substituem sua ação normal enquanto estão ativas; pausas gerais do
perfil bloqueiam todos os domínios e prevalecem sobre as regras. O motor avalia
o horário a cada consulta, inclusive em intervalos que atravessam a meia-noite.
Os horários não dependem de um processo cron.

O DNS opera por domínio: não inspeciona caminhos de URL, conteúdo de páginas ou
mensagens. Aplicativos e navegadores que usem DNS-over-HTTPS (DoH), VPN, IP direto
ou outro resolvedor podem contornar a política. Os limites de segurança e
privacidade estão detalhados em [Ameaças e limites](ameacas-e-limites.md).

## Capacidades do MVP

- vários perfis usam o mesmo endereço e porta DoT; o hostname seleciona a
  política de cada perfil;
- ações `Permitir`, `Observar` e `Proteger`, com atualização sem reiniciar o
  gateway;
- ações agendadas `Permitir` ou `Proteger` por grupo, com janelas semanais no
  fuso da casa;
- pausas gerais por perfil, com bloqueio DNS de todos os domínios durante cada
  janela;
- DoH GET e POST para provedores personalizados de navegador, reutilizando o
  hostname secreto do perfil como seletor no caminho (`TEENDNS_DOH_BASE_URL`)
  e sem registrar esse caminho no access log do Nginx;
- cache recursivo compartilhado sem compartilhar decisões entre perfis;
- painel de regras com 15 pacotes prontos e ajustes por casa;
- cadastro de casas por convite, com perfis e chave administrativa isolados;
- pareamento DNS para uma visão jovem dos acordos ativos, sem histórico de
  navegação;
- catálogo versionado de apostas, conteúdo adulto e pools por serviço;
- classificador experimental separado do caminho crítico do DNS.

## Componentes

| Caminho | Responsabilidade |
| --- | --- |
| `cmd/teendns` | Inicialização do gateway, fixture local e API administrativa |
| `internal/` | Gateway DNS, políticas, configuração, pareamento, digest e setup de aparelhos |
| `web/` | Site público, painel, cadastro e visão jovem |
| `catalog/v1/` | Categorias, presets, pools por serviço e proveniência |
| `classifier/` | Experimento de classificação de páginas, fora do caminho crítico do DNS |
| `criteria/` | Taxonomia etária e contratos do classificador |
| `deploy/staging/` | Compose e configuração declarativa do staging |
| `scripts/` | Laboratório, carga e geração e validação do catálogo |

## Executar o laboratório local

O ambiente de desenvolvimento usa Nix, Go, Node.js, Python com `uv` e Docker
Compose. Na raiz do repositório:

```bash
./scripts/lab-up.sh
```

O laboratório publica:

- site e painel: <http://127.0.0.1:18082>;
- API administrativa: <http://127.0.0.1:18081>;
- DNS-over-TLS: `127.0.0.1:8853`;
- chave administrativa local: `teendns-lab`.

Execute os testes integrados e encerre o ambiente com:

```bash
./scripts/lab-test.sh
./scripts/lab-down.sh
```

Os dados de execução ficam em `.local/` e não devem ser versionados. O Compose
de staging não usa essa chave local: exige configuração própria, descrita em
[Operação do staging](staging.md).

## Validar mudanças

Go, catálogo e critérios são verificados a partir da raiz:

```bash
nix develop --command go test ./cmd/... ./internal/... ./classifier/...
nix develop --command go vet ./cmd/... ./internal/... ./classifier/...
nix develop --command go test -race ./cmd/... ./internal/... ./classifier/...
uv run scripts/build-catalog-v1.py --check
uv run scripts/validate-age-criteria.py
```

Frontend, a partir de `web/`:

```bash
npm run lint
npm run build
```

Mudanças de interface também devem ser conferidas em desktop e celular. Para
alterações no catálogo, atualize juntos o gerador, os artefatos versionados, o
manifesto e os checksums.

## Documentação técnica relacionada

- [Estado atual e evidências](status.md)
- [Laboratório local](laboratorio.md)
- [Operação do staging](staging.md)
- [Ameaças, privacidade e limites](ameacas-e-limites.md)
- [Metodologia do catálogo](metodologia-catalogo-v1.md)
- [Critérios de classificação etária](criterios-classificacao-etaria.md)
- [Classificador experimental](classifier-service.md)
- [Sistema visual](design-system.md)
- [Plano técnico original](plano-execucao.md)

As regras de contribuição e operação do repositório estão em
[`AGENTS.md`](../AGENTS.md).
