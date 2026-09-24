# Estado da execução

Atualizado em 23 de setembro de 2026. Branch: `codex/admin-web`.

## Marcos

| Marco | Estado | Evidência |
| --- | --- | --- |
| M1 — DNS no laboratório | Concluído | Consulta DNS-over-TLS atravessa gateway e Unbound e recebe resposta determinística da fixture |
| M2 — Mini-DNS por perfil | Concluído | Ana bloqueia `blocked.test`; Bia resolve o mesmo domínio pelo mesmo IP e porta |
| M3 — Cache seguro | Concluído | Dois perfis usam uma consulta upstream; TTL é limitado; política recarrega atomicamente; CNAME não contorna bloqueio |
| M4 — Autoprovisionamento | Concluído no laboratório | Painel cria perfis, edita regras, gira endpoints e ativa mudanças sem reiniciar o DNS; visão jovem pareia pelo DNS |
| M5 — Camada educativa | Em andamento | Página pública e linguagem visual concluídas; pedido e contestação ainda pendentes |
| M6 — Aparelho real | Pendente | Requer domínio, certificado público e staging na porta 853 |
| M7 — Piloto controlado | Pendente | Requer revisão de privacidade, autenticação e operação |

## Verificações executadas

### Testes de código

```text
nix develop --command go test ./cmd/... ./internal/...
nix develop --command go vet ./cmd/... ./internal/...
nix develop --command go test -race ./cmd/... ./internal/...
docker compose config --quiet
cd web && npm run lint && npm run build
```

Resultado: todos passaram. A execução com detector de corridas não encontrou acesso concorrente inseguro na troca de políticas ou no gateway.

### Integração em containers

```text
./scripts/lab-up.sh
./scripts/lab-test.sh
```

Casos confirmados:

- resolução permitida;
- bloqueio por perfil com `NXDOMAIN`;
- políticas diferentes no mesmo IP e porta;
- exceção de domínio mais específico;
- rejeição de endpoint desconhecido;
- bloqueio de destino após CNAME;
- cache compartilhado apenas para resposta pública;
- TTL máximo de 300 segundos e bloqueio negativo de 30 segundos;
- recarga por `SIGHUP` preservando a versão anterior em caso de erro.
- alteração pelo painel aplicada ao DNS sem reiniciar o gateway;
- grupos de regras aplicados pelo resolvedor, incluindo subdomínios;
- restauração dos domínios padrão preservada pelo servidor;
- desafio DNS de uso único convertido em sessão de leitura do perfil;
- resposta jovem limitada a nomes e motivos dos grupos protegidos;
- página pública e painel servidos pelo mesmo laboratório.

### Interface

Validação no navegador conectado:

- página pública em `1440 × 1000` e `390 × 844`;
- painel em `1440 × 900` e `390 × 844`;
- login local, troca de perfil, lista de regras e estados responsivos;
- edição de `Apostas` com os 185 domínios do catálogo visíveis e restauração da
  lista padrão;
- criação com um domínio usando o próprio domínio como nome e criação com dois
  domínios exigindo um nome para o grupo;
- pareamento completo da rota `/meu-dns` com o perfil `Casa`, incluindo os
  estados de espera e sucesso;
- visão jovem em desktop e Chromium emulado em `390 × 844`, sem domínios ou
  histórico na resposta da API;
- mudança de `blocked.test` de `Proteger` para `Permitir` produziu resposta
  `NOERROR`; a restauração para `Proteger` voltou a produzir `NXDOMAIN`.

### Carga local

Burst:

```text
50 perfis
500 consultas
100 trabalhadores concorrentes
4.965 consultas/s
p95 35,194 ms
0 falhas
```

Carga sustentada:

```text
50 perfis
30.000 consultas
50 consultas/s por 10 minutos
p95 1,982 ms
0 falhas
```

Fotografia de recursos durante a carga sustentada:

```text
gateway: aproximadamente 4,5% CPU e 8,9 MiB RAM
Unbound: aproximadamente 0,2% CPU e 7,9 MiB RAM
fixture: aproximadamente 0,01% CPU e 1,9 MiB RAM
```

Esses números descrevem apenas esta máquina e o upstream local em cache. Não são estimativa de capacidade de produção ou latência da internet.

## Problemas encontrados e resolvidos

- A faixa Docker `172.28.0.0/24` sobrepunha uma rede existente; o laboratório passou a usar `10.77.53.0/24`.
- A chave da CA local não era legível pelo usuário sem privilégios do container; o gerador agora aplica permissões exclusivas do laboratório e documenta que a chave não pode ir para staging.
- O Unbound tratava `.test` como zona reservada local; a configuração desativa esse comportamento apenas no laboratório.
- Regenerar a CA sem recriar o gateway deixava cliente e servidor com certificados diferentes; `lab-up.sh` agora força a recriação dos containers.
- A troca atômica do arquivo de configuração criava modo `0600`; o helper agora publica a cópia de laboratório como `0644` para o container sem privilégios.
- Um CNAME poderia apontar para domínio bloqueado depois de uma consulta inicialmente permitida; o gateway agora reavalia os destinos CNAME.
- O catálogo entrava na imagem com permissão exclusiva do arquivo de origem; a
  imagem agora publica os catálogos como leitura para o processo sem privilégios.

## Limites ainda não validados

- Android real, rede móvel e diferentes fabricantes;
- certificado público e wildcard em staging;
- resolvedor recursivo contra a internet real sob carga;
- IPv6;
- cadeias DNAME e outros tipos DNS além de CNAME;
- autenticação multiusuário, recuperação de conta e autorização de produção;
- persistência em PostgreSQL e retenção de eventos;
- navegadores e aplicativos que forçam DoH próprio;
- disponibilidade e recuperação em uma VPS real.
