# Plano de execução — MVP teenDNS

Versão inicial: 23 de setembro de 2026.

Estado atual: M1, M2 e M3 concluídos no laboratório local. Resultados e lacunas estão registrados em [status.md](status.md). O próximo marco é M4 — autoprovisionamento.

## 1. Objetivo da validação

Construir e testar um serviço de DNS privado no qual vários perfis compartilham o mesmo servidor e endereço IP, mas recebem políticas diferentes a partir do hostname usado na conexão DNS-over-TLS.

O MVP precisa provar que:

1. um perfil pode ser criado e receber um hostname individual;
2. dois clientes usando o mesmo gateway recebem decisões diferentes para o mesmo domínio;
3. o cache não mistura respostas ou decisões entre perfis;
4. alterações de regra respeitam um tempo de propagação conhecido;
5. eventos mínimos aparecem no painel correto;
6. um Android real consegue usar o hostname como DNS privado sem aplicativo.

O MVP não precisa interceptar HTTPS, substituir páginas bloqueadas, inspecionar conteúdo ou oferecer infraestrutura DNS de produção.

## 2. Arquitetura do MVP

```text
                         ┌─────────────────────────┐
                         │ Painel e API de controle│
                         │ perfis, regras, eventos │
                         └───────────┬─────────────┘
                                     │
cliente ── DNS-over-TLS :853 ──► gateway teenDNS
hostname do perfil                  │
                                    ├─ identifica o perfil pelo SNI
                                    ├─ aplica política e horários
                                    ├─ registra evento mínimo
                                    └─ encaminha consultas permitidas
                                                │
                                                ▼
                                        Unbound recursivo
                                                │
                                     cache de respostas brutas
                                                │
                                  internet ou DNS falso de teste
```

### Componentes

| Componente | Responsabilidade | Tecnologia proposta |
| --- | --- | --- |
| Gateway DoT | TLS, identificação do perfil, política, TTL e encaminhamento | Go |
| Resolvedor | Recursão, DNSSEC e cache de respostas públicas | Unbound |
| API de controle | Perfis, endpoints, regras, eventos e pedidos | Go no mesmo serviço do gateway no MVP |
| Banco | Configuração e eventos | PostgreSQL |
| Painel | Criar perfil, copiar hostname e editar regras | Interface web mínima |
| DNS autoritativo falso | Respostas determinísticas para testes locais | CoreDNS ou servidor mínimo de fixture |
| Clientes simulados | Consultas DoT com SNI diferentes | Testes de integração em Go |

O gateway e a API podem começar no mesmo binário, com pacotes internos separados. Isso reduz operação sem misturar o núcleo DNS com a interface.

## 3. Laboratório local com containers

O ambiente será iniciado por Docker Compose:

```text
compose
├── gateway
├── unbound
├── postgres
├── authoritative-fixture
├── client-test
└── painel
```

### Dois modos de resolução

**Modo hermético:** o Unbound encaminha todas as consultas ao DNS autoritativo falso. Os testes não dependem da internet e podem controlar IP, TTL, NXDOMAIN e falhas.

**Modo integração:** o Unbound faz resolução recursiva real. Serve para smoke tests, mas não para testes determinísticos.

### Certificados locais

Os testes terão uma autoridade certificadora exclusiva do laboratório e um certificado para `*.dns.teendns.test`. Os clientes simulados confiarão apenas nessa CA.

Cada cliente conecta ao mesmo serviço Docker, mas usa um nome TLS diferente:

```text
p-ana.dns.teendns.test  ─┐
p-bia.dns.teendns.test  ─┼─► gateway:853
p-teste.dns.teendns.test ┘
```

O endereço Docker é o mesmo. O `ServerName` da conexão TLS simula exatamente o hostname configurado como DNS privado no Android.

Os certificados de teste não devem ser reutilizados em staging ou produção.

## 4. Modelo de dados inicial

### `profiles`

- identificador interno;
- apelido informado pelo responsável;
- status;
- fuso horário;
- data de criação.

### `profile_endpoints`

- perfil;
- token aleatório de pelo menos 128 bits;
- hostname derivado;
- status e revogação;
- último uso.

O hostname não contém nome, e-mail ou idade da criança.

### `policies`

- perfil;
- versão monotônica;
- ação padrão;
- regras por domínio e categoria;
- janela de horário;
- TTL máximo permitido;
- data de publicação.

### `domain_catalog`

- domínio ou regra de subdomínio;
- serviço;
- categoria;
- motivo legível;
- fonte e data de revisão.

### `dns_events`

- perfil;
- domínio normalizado;
- categoria;
- decisão;
- instante;
- versão da política.

Não armazenar URL, caminho, título, conteúdo, resposta completa ou endereço IP do destino. Para o piloto, eventos detalhados expiram; agregados podem ter retenção distinta.

## 5. Ordem de implementação

### Fase 0 — Contratos e decisões de segurança

Entregas:

- formato dos hostnames e tokens;
- semântica de `allow`, `block` e `observe`;
- política para domínios desconhecidos;
- estratégia de TTL;
- retenção dos eventos;
- comportamento em falhas;
- ameaça mínima documentada.

Decisão inicial de falha:

- usar a última política válida em memória se o banco cair;
- rejeitar endpoint desconhecido ou revogado;
- retornar `SERVFAIL` se o resolvedor estiver indisponível;
- nunca liberar silenciosamente uma consulta apenas porque um componente falhou.

### Fase 1 — Laboratório DNS reproduzível

Entregas:

- `compose.yaml`;
- Unbound configurado;
- DNS autoritativo de fixture;
- CA e certificados exclusivos do ambiente de teste;
- cliente DoT de integração;
- comandos únicos para subir, testar e derrubar o laboratório.

Critério de aceite: uma consulta DoT atravessa gateway e Unbound e recebe a resposta conhecida da fixture.

### Fase 2 — Perfis e políticas isoladas

Entregas:

- identificação por SNI;
- carregamento e compilação de regras;
- correspondência segura de domínio e subdomínio;
- bloqueio por `NXDOMAIN` ou `REFUSED`, definido nos contratos;
- ação `observe` sem bloqueio;
- versionamento e recarga de política.

Critério de aceite: Ana recebe bloqueio e Bia recebe permissão para o mesmo domínio, usando o mesmo IP e a mesma porta.

### Fase 3 — Cache e propagação

Entregas:

- cache de respostas brutas no Unbound;
- decisões avaliadas por perfil antes da entrega;
- eventual cache de decisão com chave `perfil + domínio + versão`;
- TTL máximo de respostas permitidas;
- TTL curto para bloqueios;
- métricas de acerto de cache.

Critério de aceite: uma resposta pública pode ser reaproveitada sem transportar a decisão de outro perfil, e uma alteração de política entra em vigor dentro do limite declarado.

### Fase 4 — Provisionamento e painel mínimo

Entregas:

- criar e desativar perfil;
- gerar, revogar e regenerar hostname;
- cadastrar regra por domínio ou categoria;
- mostrar instruções de configuração Android;
- listar eventos agregados e bloqueios recentes;
- apagar dados do perfil.

Critério de aceite: um perfil completo é provisionado pela interface sem editar arquivos do servidor.

### Fase 5 — Pedido e contestação

Entregas:

- código ou link de pareamento do portal da criança;
- explicação do motivo de uma intervenção;
- pedido de acesso com justificativa curta;
- contestação de falso positivo;
- aprovação, recusa e exceção com prazo;
- registro separado entre consulta DNS e conversa familiar.

O portal não aparecerá automaticamente sobre um site HTTPS bloqueado. Ele será uma página web própria acessível por favorito, link ou QR code.

### Fase 6 — Staging e Android real

Entregas:

- domínio real;
- certificado público válido;
- gateway acessível na porta TCP 853;
- perfil de teste sem dados pessoais;
- validação em Wi-Fi e rede móvel;
- verificação de Chrome, Firefox e pelo menos um aplicativo comum;
- teste com mudança de rede e reinício do aparelho.

Critério de aceite: dois aparelhos reais, no mesmo Wi-Fi, utilizam políticas diferentes e continuam funcionando em rede móvel.

## 6. Matriz de testes automatizados

### Protocolo e TLS

- certificado válido e inválido;
- SNI ausente, desconhecido e revogado;
- consulta UDP enviada por engano à porta esperada;
- conexão ociosa e reconexão;
- mensagens malformadas e grandes;
- timeouts do resolvedor.

### Políticas

- domínio exato;
- subdomínio permitido e bloqueado;
- domínio enganoso como `google.com.exemplo.test`;
- maiúsculas e ponto final;
- CNAME para categoria bloqueada;
- horários e fuso;
- exceção temporária;
- regra desconhecida;
- atualização concorrente da política.

### Cache

- mesmo domínio permitido para dois perfis;
- mesmo domínio permitido para um e bloqueado para outro;
- cache quente após mudança de política;
- expiração positiva e negativa;
- versão nova invalida decisão antiga;
- cache do resolvedor não contém identificador do perfil.

### Privacidade e isolamento

- perfil não acessa eventos de outro;
- hostname não contém dados pessoais;
- logs não contêm URLs ou conteúdo;
- exclusão remove configuração e eventos previstos;
- endpoint revogado para de resolver;
- métricas não expõem domínios individuais por padrão.

### Carga

Meta inicial de laboratório:

- 50 perfis provisionados;
- 50 consultas por segundo sustentadas durante dez minutos;
- 100 conexões DoT simultâneas;
- nenhuma mistura de política;
- latência quente medida separadamente da resolução externa.

Esses números validam o piloto; não são promessa de capacidade de produção.

## 7. O que os containers simulam — e o que não simulam

### Conseguimos simular

- dezenas ou centenas de perfis;
- mesmo IP com vários hostnames;
- negociação TLS e SNI;
- regras diferentes;
- cache positivo e negativo;
- TTL e atualização de política;
- falhas do banco, gateway, upstream e resolvedor;
- concorrência e carga;
- vazamento entre perfis;
- expiração e exclusão de eventos.

### Precisamos de aparelhos reais

- disponibilidade do campo “DNS privado” em fabricantes diferentes;
- comportamento em Wi-Fi e rede móvel;
- aplicativos que usam DNS próprio ou VPN;
- interferência do DNS seguro do navegador;
- cache real do sistema operacional;
- consumo de bateria e reconexões;
- experiência de configuração para uma família.

## 8. Infraestrutura de staging

Depois do laboratório local, o staging precisa de:

- uma VPS Linux com IP fixo;
- DNS público para `*.dns.<domínio>`;
- certificado wildcard obtido por desafio DNS;
- portas 443 e 853 liberadas;
- containers ou serviços do sistema para gateway, Unbound, banco e painel;
- backup do banco;
- métricas e logs sem conteúdo sensível;
- health checks separados para API, DoT e resolução;
- monitoramento externo antes de qualquer piloto familiar.

O staging começa com perfis sintéticos. Dados de crianças só entram após revisão de privacidade, retenção e controles de acesso.

## 9. Marcos de entrega

| Marco | Resultado demonstrável |
| --- | --- |
| M1 — DNS no laboratório | Consulta DoT determinística completa |
| M2 — Mini-DNS por perfil | Políticas distintas no mesmo IP |
| M3 — Cache seguro | Sem vazamento e propagação previsível |
| M4 — Autoprovisionamento | Perfil e regra criados pelo painel |
| M5 — Camada educativa | Explicação, pedido e contestação no portal |
| M6 — Aparelho real | Android funcionando em Wi-Fi e rede móvel |
| M7 — Piloto controlado | Pequeno grupo, métricas e suporte definidos |

## 10. Primeiro corte de implementação

Começar por M1 a M3, sem frontend:

1. criar o ambiente Nix do projeto;
2. criar o Docker Compose;
3. subir Unbound e a fixture autoritativa;
4. implementar o gateway DoT mínimo;
5. criar dois perfis em fixture;
6. executar consultas concorrentes com SNI distintos;
7. provar isolamento e comportamento do cache;
8. documentar os resultados antes de construir o painel.

Esse corte responde primeiro à incerteza técnica central: se vários “mini-DNS” personalizados podem operar de forma segura atrás do mesmo IP.
