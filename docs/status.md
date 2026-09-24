# Estado da execução

Atualizado em 24 de setembro de 2026. Branch: `codex/admin-web`.

## Marcos

| Marco | Estado | Evidência |
| --- | --- | --- |
| M1 — DNS no laboratório | Concluído | Consulta DNS-over-TLS atravessa gateway e Unbound e recebe resposta determinística da fixture |
| M2 — Mini-DNS por perfil | Concluído | Ana bloqueia `blocked.test`; Bia resolve o mesmo domínio pelo mesmo IP e porta |
| M3 — Cache seguro | Concluído | Dois perfis usam uma consulta upstream; TTL é limitado; política recarrega atomicamente; CNAME não contorna bloqueio |
| M4 — Autoprovisionamento | Concluído no staging | Convite cria casa isolada e primeiro perfil; painel lista os 15 pacotes do catálogo, permite ligar, desligar ou mudar sua ação e ativa mudanças sem reiniciar o DNS |
| M5 — Camada educativa | Em andamento | Página pública e linguagem visual concluídas; pedido e contestação ainda pendentes |
| M6 — Aparelho real | Em validação | Android real usou DoT e bloqueou `instagram.com`; o app continuou por domínios auxiliares e recebeu agora o pool ampliado para novo teste |
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

Novos casos cobertos por testes:

- `GET /api/v1/admin/guard` responde `200` para a chave do operador e `401`
  sem token ou com chave de casa, e o bundle do site não embute mais token;
- `PUT /api/v1/houses/{id}/emails` é rejeitado por chave de casa, valida
  endereços, normaliza (trim, minúsculas, sem duplicados), persiste e mantém o
  primeiro e-mail como principal;
- magic link é aceito para qualquer e-mail da lista da casa (incluindo caixa
  alta), sem cair na lista de espera;
- a carga do `gateway.json` migra `email` → `emails` para casas existentes;
- `GET /api/v1/profiles/{id}/setup/info` expõe nome do servidor, IP (quando
  conhecido), porta e domínio de teste — valores neutros, sem dados pessoais;
- `GET /api/v1/profiles/{id}/setup/windows.bat`,
  `windows-remove.bat` e `apple.mobileconfig` geram instalador e removidor
  para Windows 24H2+ (gate `lss 26100`, DoT por perfil com
  `autoupgrade=yes` e `udpfallback=no`) e perfil de DNS TLS com
  `com.apple.dnsSettings.managed`, recusando perfil desativado, kind
  desconhecido e perfil de outra casa com `404`;
- o instalador Windows devolve `422` quando não há IP público e o perfil Apple
  omite `ServerAddresses` nesse caso (o gateway resolve o apex do sufixo do
  DNS quando `TEENDNS_DNS_PUBLIC_IP` não está definido);
- `GET /api/v1/pairing/challenges/{id}/outcome` é escopado por casa: relatando
  `observed:false`, ou o perfil que observou, e `404` para desafio de fora da
  casa;

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
- convite de uso único cria casa, chave administrativa e primeiro perfil;
- chave de uma casa lista apenas seus perfis e recebe `404` para perfil alheio.
- catálogo administrativo lista 15 pacotes prontos com contagem de domínios;
- ligar e desligar um pacote altera a política ativa e persiste a configuração.

### Configuração por aparelho (lab)

Gateway e painel recriados com o código novo (`./scripts/lab-up.sh` com
`TEENDNS_DNS_PUBLIC_IP=203.0.113.10` passado ao gateway pelo compose):

- `GET /api/v1/profiles/ana/setup/info` respondeu
  `{"hostname":"p-ana.dns.teendns.test","ip":"203.0.113.10","port":"853","test_domain":"example.com"}`;
- `windows.bat` retornou `200` com `Content-Type: text/plain; charset=utf-8`,
  `Content-Disposition: attachment; filename="configurar-teendns.bat"` e
  `Cache-Control: no-store`, com `set "TEENDNS_HOST=…"`, o gate
  `if %TEENDNS_BUILD_INT% lss 26100 goto :oldwindows` e
  `netsh dns add encryption … autoupgrade=yes udpfallback=no`;
- `windows-remove.bat` respondeu `200` com `filename="remover-teendns.bat"`;
- `apple.mobileconfig` respondeu com o perfil `com.apple.dnsSettings.managed`
  (`DNSProtocol=TLS`, `ServerAddresses` do IP configurado);
- kind desconhecido respondeu `404` e a chamada sem chave respondeu `401`;
- um desafio de pareamento criado pela API e resolvido pelo DoT do laboratório
  (query `…pair.teendns.test` pelo perfil `ana`) retornou
  `GET /api/v1/pairing/challenges/{id}/outcome` →
  `{"observed":true,"profile_id":"ana"}`, fechando o caminho de
  "Já instalei? Testar conexão" com prova no próprio DNS;
- `./scripts/lab-test.sh` continuou verde após a recriação (resolução,
  bloqueio por perfil, CNAME, cache compartilhado e recarga de política).

### Interface

Validação no navegador conectado:

- página pública em `1440 × 1000` e `390 × 844`;
- painel em `1440 × 900` e `390 × 844`;
- cadastro de casa em `1440 × 900` e `390 × 844`, sem rolagem horizontal;
- formulário de convite inválido exibindo erro legível sem perder os campos;
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
- cadastro oferece Acompanhado, Explorando e Autonomia guiada sem armazenar
  idade ou data de nascimento;
- seletor de preset validado em Chromium emulado em `390 × 844` e `1440 × 900`,
  sem rolagem horizontal ou erros de console.
- gaveta de pacotes validada no staging em `1440 × 900` e Chromium emulado em
  `390 × 844`, com 15 pacotes, sem rolagem horizontal ou mensagens de console;
- pacote `Threads` foi ligado e desligado pela API do staging e o estado inicial
  desligado foi restaurado.

### Catálogo v1

- 185 domínios de apostas e 92 de conteúdo adulto de fontes oficiais ou
  regulatórias;
- 13 pools editáveis por serviço, com 52 sufixos específicos e dependências
  compartilhadas documentadas, mas excluídas do bloqueio;
- Instagram cobre `instagram.com`, `cdninstagram.com` e `ig.me` sem bloquear
  `fbcdn.net`, `fbsbx.com` ou a infraestrutura genérica da Meta;
- 192.095 domínios de phishing e ransomware foram catalogados para uma futura
  camada global compartilhada; não são duplicados dentro de cada perfil;
- metodologia, fontes, licenças e protocolo de teste real estão em
  [metodologia-catalogo-v1.md](metodologia-catalogo-v1.md).

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

- rede móvel, outros fabricantes e reteste do app com o pool ampliado;
- resolvedor recursivo contra a internet real sob carga;
- IPv6;
- cadeias DNAME e outros tipos DNS além de CNAME;
- autenticação multiusuário, recuperação de conta e autorização de produção;
- persistência em PostgreSQL e retenção de eventos;
- navegadores e aplicativos que forçam DoH próprio;
- restauração completa depois de falha da VPS.

## Staging público

O primeiro deploy real está ativo em <https://teendns.lab.markun.com.br>. O
gateway DoT usa `*.dns.lab.markun.com.br:853`, certificado público e Unbound
recursivo. A implantação é um projeto Compose isolado na VPS compartilhada com o
Farol Lab; detalhes operacionais e de renovação estão em [staging.md](staging.md).

Continuam pendentes o reteste do app com o pool ampliado, a renovação automática
do wildcard e exercícios de restauração depois de falha da VPS.
