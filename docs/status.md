# Estado da execução

Atualizado em 23 de setembro de 2026. Branch: `feat/dot-lab`.

## Marcos

| Marco | Estado | Evidência |
| --- | --- | --- |
| M1 — DNS no laboratório | Concluído | Consulta DNS-over-TLS atravessa gateway e Unbound e recebe resposta determinística da fixture |
| M2 — Mini-DNS por perfil | Concluído | Ana bloqueia `blocked.test`; Bia resolve o mesmo domínio pelo mesmo IP e porta |
| M3 — Cache seguro | Concluído | Dois perfis usam uma consulta upstream; TTL é limitado; política recarrega atomicamente; CNAME não contorna bloqueio |
| M4 — Autoprovisionamento | Próximo | Ainda não iniciado |
| M5 — Camada educativa | Pendente | Requer contrato de conteúdo e direção visual |
| M6 — Aparelho real | Pendente | Requer domínio, certificado público e staging na porta 853 |
| M7 — Piloto controlado | Pendente | Requer revisão de privacidade, autenticação e operação |

## Verificações executadas

### Testes de código

```text
nix develop --command go test ./...
nix develop --command go vet ./...
nix develop --command go test -race ./...
docker compose config --quiet
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

## Limites ainda não validados

- Android real, rede móvel e diferentes fabricantes;
- certificado público e wildcard em staging;
- resolvedor recursivo contra a internet real sob carga;
- IPv6;
- cadeias DNAME e outros tipos DNS além de CNAME;
- autenticação e autorização do painel;
- persistência em PostgreSQL e retenção de eventos;
- navegadores e aplicativos que forçam DoH próprio;
- disponibilidade e recuperação em uma VPS real.
