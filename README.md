# teenDNS

Controle parental por DNS que protege sem virar vigilância. Cada casa recebe
perfis independentes, escolhe pacotes de proteção e pode explicar os acordos
para crianças e adolescentes em uma página pareada.

O MVP está ativo em <https://teendns.lab.markun.com.br>.

## Usar o staging

- painel administrativo: <https://teendns.lab.markun.com.br/painel>;
- cadastro por convite: <https://teendns.lab.markun.com.br/comecar>;
- visão jovem pareada: <https://teendns.lab.markun.com.br/meu-dns>.

O painel mostra o hostname de DNS privado de cada perfil. No Android, ele pode
ser informado em **Configurações > Rede e internet > DNS privado**, sem instalar
aplicativo. Cada hostname seleciona uma política diferente no mesmo IP e porta
DNS-over-TLS.

## Rodar localmente

O projeto usa Nix, Go, React e Docker Compose:

```bash
./scripts/lab-up.sh
```

O laboratório publica:

- site e painel: <http://127.0.0.1:18082>;
- API administrativa: <http://127.0.0.1:18081>;
- DNS-over-TLS: `127.0.0.1:8853`;
- chave administrativa local: `teendns-lab`.

Valide o fluxo completo e encerre o laboratório com:

```bash
./scripts/lab-test.sh
./scripts/lab-down.sh
```

O estado local fica em `.local/`, fora do Git.

## O que já funciona

- vários perfis DNS no mesmo endereço, identificados pelo SNI do DoT;
- políticas `Permitir`, `Observar` e `Proteger`, aplicadas sem reiniciar;
- cache recursivo compartilhado sem compartilhar decisões familiares;
- painel responsivo com edição de regras e 15 pacotes prontos;
- cadastro de casas por convite e isolamento por chave administrativa;
- pareamento pelo DNS para uma visão jovem sem histórico de navegação;
- catálogo versionado de apostas, conteúdo adulto e pools por serviço;
- classificador experimental de páginas separado do caminho crítico do DNS.

```text
celular ou navegador
        │ DNS-over-TLS + hostname do perfil
        ▼
gateway teenDNS ── política em memória ──► NXDOMAIN quando protegido
        │
        ▼
     Unbound ──► internet

painel web ──► API administrativa ──► estado persistido + troca atômica
```

## Estrutura

| Caminho | Responsabilidade |
| --- | --- |
| `cmd/teendns` | gateway DoT, fixture local e API administrativa |
| `internal/` | política, DNS, configuração, pareamento e painel |
| `web/` | site público, painel, cadastro e visão jovem |
| `catalog/v1/` | listas, presets, pools por serviço e proveniência |
| `classifier/` | experimento de classificação de páginas |
| `criteria/` | taxonomia etária e contratos do classificador |
| `deploy/staging/` | Compose e configuração declarativa do staging |
| `scripts/` | laboratório, carga e geração/validação de catálogo |
| `docs/` | decisões, operação, limites e evidências de validação |

## Validar mudanças

```bash
nix develop --command go test ./cmd/... ./internal/... ./classifier/...
nix develop --command go vet ./cmd/... ./internal/... ./classifier/...
nix develop --command go test -race ./cmd/... ./internal/... ./classifier/...
uv run scripts/build-catalog-v1.py --check
uv run scripts/validate-age-criteria.py
cd web && npm run lint && npm run build
```

Mudanças de interface também devem ser verificadas em desktop e celular no
navegador conectado. O Compose de staging exige um `.env` real e não deve ser
validado inventando credenciais.

## Limites importantes

- DNS conhece domínios, não páginas, mensagens ou intenção.
- HTTPS impede redirecionar universalmente um bloqueio para uma página própria.
- Apps podem manter conexões/IPs em cache ou usar VPN e DNS próprio.
- Pools de serviços evitam infraestrutura compartilhada; por isso são melhor
  esforço e precisam de testes periódicos em aparelhos reais.
- O staging ainda não possui recuperação de conta, PostgreSQL ou alta
  disponibilidade.

## Documentação

- [estado atual e evidências](docs/status.md);
- [laboratório local](docs/laboratorio.md);
- [operação do staging](docs/staging.md);
- [ameaças e limites](docs/ameacas-e-limites.md);
- [metodologia do catálogo](docs/metodologia-catalogo-v1.md);
- [sistema visual](docs/design-system.md);
- [plano técnico original](docs/plano-execucao.md);
- [critérios de classificação etária](docs/criterios-classificacao-etaria.md);
- [classificador experimental](docs/classifier-service.md).

As regras de trabalho do repositório estão em [AGENTS.md](AGENTS.md).
