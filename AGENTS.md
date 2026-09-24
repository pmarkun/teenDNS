# AGENTS.md

## Projeto

O teenDNS é um MVP de controle parental por DNS-over-TLS. O caminho crítico é
`gateway + política + Unbound`; `classifier/` é experimental e não deve virar
dependência do DNS sem decisão explícita.

Antes de editar, leia `README.md`, `docs/status.md` e a documentação específica
do componente. Preserve mudanças existentes no worktree.

## Ambiente e comandos

- Use `nix develop --command ...` para Go e ferramentas do projeto.
- Use `uv run` para scripts Python; não use `pip`.
- Testes Go: `nix develop --command go test ./cmd/... ./internal/... ./classifier/...`.
- Análise Go: `nix develop --command go vet ./cmd/... ./internal/... ./classifier/...`.
- Frontend: em `web/`, rode `npm run lint` e `npm run build`.
- Catálogo: `uv run scripts/build-catalog-v1.py --check`.
- Critérios: `uv run scripts/validate-age-criteria.py`.
- Integração local: `./scripts/lab-up.sh`, `./scripts/lab-test.sh` e
  `./scripts/lab-down.sh`.

## Regras do repositório

- Não versione `.local/`, `tmp/`, `secrets`, `.env`, certificados ou estado de
  runtime.
- `catalog/v1/` é gerado por `scripts/build-catalog-v1.py`; altere gerador,
  artefatos, manifesto e checksums juntos.
- Pools de serviço devem evitar CDNs, login e nuvens compartilhadas; registre a
  exclusão em `service-pools.json`.
- O DNS decide por domínio. Não prometa inspeção de páginas, redirecionamento
  HTTPS universal ou bloqueio completo de apps com DoH/VPN próprios.
- Mudanças no painel devem manter a identidade descrita em
  `docs/design-system.md` e ser verificadas em desktop e celular.
- Atualize `docs/status.md` somente com evidência executada. Mudanças de operação
  do staging também exigem atualização de `docs/staging.md`.

## Git e staging

- Faça commits convencionais, pequenos e atômicos; não misture código,
  catálogo, limpeza e documentação sem necessidade.
- `main` é integração; `production` é a única fonte de deploy e aceita mudanças
  somente por PR. Não faça push direto nem force-push nessas branches.
- Uma release segue `feature -> main -> production -> deploy do SHA exato`.
  Ao terminar, `/opt/teendns/DEPLOYED_REVISION` deve coincidir com
  `origin/production`.
- Faça merge ou publique staging apenas quando solicitado. Preserve
  `/opt/teendns/runtime/gateway.json`; nunca use `docker compose down`, prune ou
  alterações globais na VPS.
- Após deploy, valide `/healthz`, containers, uma resolução permitida, um
  bloqueio DoT e a interface pública afetada.
