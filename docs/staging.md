# Staging real

O staging público roda na VPS Hetzner `178.105.202.118`, sem compartilhar
containers, rede ou volumes com o Farol Lab/Ralph.

## Endereços

- site e painel: <https://teendns.lab.markun.com.br>;
- DNS-over-TLS do perfil inicial: `p-piloto.dns.lab.markun.com.br`;
- porta DoT: `853/TCP`;
- visão pareada: <https://teendns.lab.markun.com.br/meu-dns>.
- cadastro de casa por convite: <https://teendns.lab.markun.com.br/comecar>.

O registro `*.dns.lab.markun.com.br` aponta para a VPS. Cada perfil criado no
painel recebe um hostname próprio sob esse wildcard, mas todos chegam ao mesmo
gateway. O SNI da conexão TLS seleciona a política correta.

## Isolamento na VPS

| Item | Valor |
| --- | --- |
| Código implantado | `/opt/teendns/app` |
| Estado gravável | `/opt/teendns/runtime` |
| Segredo administrativo | `/opt/teendns/.env` (`0600`) |
| Convites disponíveis | `TEENDNS_INVITATION_CODES` em `/opt/teendns/.env` |
| Projeto Compose | `teendns` |
| Rede Docker | `10.78.53.0/24` |
| Web local para o Caddy | `127.0.0.1:18182` |
| DoT público | `853/TCP` |

O Caddy já existente recebeu apenas um bloco para encaminhar
`teendns.lab.markun.com.br` à porta local `18182`. O arquivo anterior foi salvo
como `/opt/farol-lab/caddy/Caddyfile.pre-teendns-20260923`.

Nunca execute `docker compose down`, `docker system prune` ou alterações globais
na VPS. Toda operação do teenDNS deve declarar o projeto e o arquivo próprios:

```bash
cd /opt/teendns/app
docker compose \
  --project-name teendns \
  --env-file /opt/teendns/.env \
  -f deploy/staging/compose.yaml ps
```

## Certificados

O site usa o certificado individual renovado automaticamente pelo Caddy. O DoT
usa um certificado Let's Encrypt para `*.dns.lab.markun.com.br`, emitido por
desafio DNS manual, sem manter um token amplo da Hetzner na VPS.

O certificado atual vence em **23 de dezembro de 2026**. A renovação ainda é
manual: repetir o desafio, copiar `fullchain.pem` e `privkey.pem` para
`/opt/teendns/runtime/certs/` e recriar somente o gateway.

A API da Hetzner oferece escrita no projeto inteiro, não apenas num registro ou
subdomínio. Por isso o staging deliberadamente não guarda esse token. Automatizar
a renovação exige antes uma credencial DNS realmente limitada ou uma zona
delegada em provedor que aceite subzonas.

## Atualização

1. Validar e commitar localmente.
2. Copiar somente os arquivos rastreados para `/opt/teendns/app`.
3. Recriar o projeto `teendns` com `up -d --build`.
4. Verificar `/healthz`, uma resolução permitida, um bloqueio e o pareamento.

O arquivo `/opt/teendns/runtime/gateway.json` é estado do staging. Não deve ser
substituído durante atualizações, pois contém os perfis e regras editados em
tempo real pelo painel.

## Convites e casas

Os códigos são aleatórios, ficam apenas no `.env` e são separados por vírgula.
Quando um código é usado, somente seu hash é gravado no estado do gateway; por
isso ele não pode cadastrar uma segunda casa. O cadastro mostra a chave da casa
uma única vez. O servidor guarda apenas o hash dessa chave e limita com ela
todas as leituras e alterações aos perfis da casa correspondente.

O `TEENDNS_ADMIN_TOKEN` continua sendo uma credencial operacional global do
staging. Ele não deve ser entregue a famílias.

## Verificações do primeiro deploy

- HTTPS público retornou `200` com certificado válido;
- `example.com` retornou `NOERROR` pelo DoT com TTL limitado a 300 segundos;
- `1pra1.bet.br` retornou `NXDOMAIN` no perfil piloto;
- o desafio criado na visão jovem foi observado pelo mesmo perfil e virou uma
  sessão pareada;
- a rota `/comecar` respondeu `200` e rejeitou um convite inválido sem consumir
  o convite piloto ativo;
- novas casas recebem 15 regras iniciais conforme Acompanhado, Explorando ou
  Autonomia guiada;
- o painel mostra os 15 pacotes prontos para qualquer perfil e permite ligar,
  desligar ou mudar entre Proteger, Observar e Permitir em tempo real;
- o ciclo de ligar e desligar o pacote `Threads` foi executado no perfil piloto
  e o estado anterior foi restaurado;
- o perfil piloto recebeu a regra Instagram com três sufixos específicos; os
  três responderam `NXDOMAIN` pelo DoT público, e o novo teste no app real ainda
  está pendente;
- os três containers `teendns-unbound-1`, `teendns-gateway-1` e
  `teendns-web-1` permaneceram isolados dos containers do Farol Lab.
