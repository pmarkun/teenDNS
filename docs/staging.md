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

## Branch de release

`main` recebe o trabalho integrado. A branch protegida `production` contém o
único código elegível para deploy nesta VPS. Ela não aceita push direto,
force-push ou exclusão; uma promoção acontece por Pull Request de `main` para
`production`.

O deploy usa o SHA exato de `origin/production` e o grava em
`/opt/teendns/DEPLOYED_REVISION`. Ao final de cada publicação, os dois valores
devem coincidir. Durante os poucos minutos de uma publicação, `production` pode
estar à frente do serviço vivo; falha de deploy deve ser tratada antes de outra
promoção.

## Isolamento na VPS

| Item | Valor |
| --- | --- |
| Código implantado | `/opt/teendns/app` |
| Estado gravável | `/opt/teendns/runtime` |
| Segredo administrativo | `/opt/teendns/.env` (`0600`) |
| E-mail transacional | `RESEND_API_KEY` e `TEENDNS_MAIL_FROM` em `/opt/teendns/.env` |
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

1. Validar e integrar a mudança em `main`.
2. Abrir e fazer merge de um PR `main -> production`.
3. Resolver localmente o SHA de `origin/production` e copiar somente seus
   arquivos rastreados para `/opt/teendns/app`.
4. Recriar o projeto `teendns` com `up -d --build`.
5. Gravar o SHA em `/opt/teendns/DEPLOYED_REVISION`.
6. Verificar `/healthz`, containers, uma resolução permitida e um bloqueio.

O arquivo `/opt/teendns/runtime/gateway.json` é estado do staging. Não deve ser
substituído durante atualizações, pois contém os perfis e regras editados em
tempo real pelo painel.

## Convites e casas

O operador gera um convite pelo console de admin em `/admin` (autenticado com
o `TEENDNS_ADMIN_TOKEN` global), informando o e-mail do responsável. O gateway
cria um código de uso único válido por 7 dias, grava apenas o hash desse
código no estado (`gateway.json`), e envia por e-mail via Resend um link para
`/comecar?convite=<código>` com o código pré-preenchido. Se o envio falhar, a
resposta da API ainda inclui o link para o operador copiar manualmente — o
convite já foi criado e continua válido.

Ao consumir o convite, o e-mail informado é copiado para o campo `email` da
casa (usado depois pelo digest semanal) e apagado do registro do próprio
convite, que passa a guardar só o hash e a data de uso. O cadastro mostra a
chave da casa uma única vez; o servidor guarda apenas o hash dessa chave e
limita com ela todas as leituras e alterações aos perfis da casa
correspondente.

O `TEENDNS_ADMIN_TOKEN` continua sendo uma credencial operacional global do
staging. Ele não deve ser entregue a famílias.

O console `/admin` só abre o conteúdo para quem apresenta a chave correta: o
próprio servidor valida o token em `GET /api/v1/admin/guard` antes de liberar
qualquer lista, convite ou botão, e o bundle do site não embute mais token de
operador. Abrir a rota sem a chave mostra apenas a tela de login.

## Login por e-mail (magic link)

Em `/painel`, o login principal pede o e-mail da casa e manda um link de
acesso de uso único (válido por 15 minutos), que vira uma sessão de 7 dias ao
ser confirmado em `/entrar`. Nada disso toca a chave administrativa
permanente da casa — ela continua funcionando como alternativa atrás de "ou
cole sua chave administrativa". Um e-mail sem casa correspondente entra na
lista de espera (`GET /api/v1/waitlist`, visível em `/admin`) em vez de
receber um link. Todo esse estado (links pendentes e sessões ativas) vive só
em memória no processo do gateway — um restart derruba sessões de magic link
ativas, mas o login por chave e um novo pedido de link continuam funcionando
normalmente.

Uma casa pode ter vários e-mails de acesso. Além do endereço vindo do convite,
o operador adiciona outros pelo botão **e-mails** de cada casa no console
(`PUT /api/v1/houses/{id}/emails`): eles são normalizados, sem duplicados, e
persistidos na casa dentro do `gateway.json`. Qualquer endereço da lista
dispara o link de uso único para a mesma casa; o primeiro endereço é o que
recebe o digest semanal. O registro antigo `email` continua sendo gravado como
espelho do primeiro endereço.

O console `/admin` também lista as casas existentes (nome, e-mails, número de
perfis) com um botão de apagar que exige digitar o nome da casa para
confirmar — a remoção é imediata e irreversível, e já apaga em cascata os
perfis da casa e qualquer acumulado do digest semanal daquela casa.

## Digest semanal por e-mail

Toda casa com e-mail cadastrado (via convite) recebe, uma vez por semana, um
resumo dos domínios observados (ação `Observar`, nunca `Bloquear`) por
perfil, agregados por período do dia — manhã, tarde ou noite, nunca horário
exato nem sequência de navegação. O acumulado fica em
`/opt/teendns/runtime/observations.json`, separado do `gateway.json`, e é
apagado a cada envio bem-sucedido. Ver a política de retenção completa em
[ameacas-e-limites.md](ameacas-e-limites.md).

O envio depende de `RESEND_API_KEY`/`TEENDNS_MAIL_FROM` reais em
`/opt/teendns/.env`; sem eles o Compose recusa subir (`:?` obrigatório, mesmo
padrão do `TEENDNS_ADMIN_TOKEN`).

## Configuração por aparelho

Hackathon: cada perfil ganhou no painel uma gaveta **CONFIGURAR UM APARELHO**
que gera, sob `GET /api/v1/profiles/{id}/setup/`, arquivos de configuração
calculados na hora:

- `info` — valores neutros usados pela interface (nome do servidor, IP quando
  conhecido, porta `853`, domínio de teste `example.com`);
- `windows.bat` — instalador para **Windows 11 24H2+** (gate `lss 26100`):
  autoelevação de administrador, DoT por perfil via
  `netsh dns add encryption server=… dothost=…:853 autoupgrade=yes
  udpfallback=no`, DNS do Edge desativado e teste de resolução. Versões
  antigas abortam apontando a configuração manual — não há fallback para DNS
  claro;
- `windows-remove.bat` — desfaz a configuração;
- `apple.mobileconfig` — perfil `com.apple.dnsSettings.managed` com
  `DNSProtocol=TLS`, válido para iOS, iPadOS e macOS.

Os arquivos não contêm dados pessoais: só nome do servidor, IP, porta e
domínio de teste. Revogação continua sendo por rotação de hostname — o
endereço antigo para de funcionar e as configurações geradas antes ficam
órfãs.

O IP público vem de `TEENDNS_DNS_PUBLIC_IP`. No staging, porque o apex
`dns.lab.markun.com.br` não é resolvido publicamente (só o wildcard cobre
subdomínios), o compose define
`TEENDNS_DNS_PUBLIC_IP: ${TEENDNS_DNS_PUBLIC_IP:-178.105.202.118}` — o default
é a própria VPS e pode ser sobrescrito no `.env`. Sem IP conhecido, o perfil
Apple sai sem `ServerAddresses` e o instalador Windows recusa com `422`.

O botão **JÁ INSTALOU? TESTAR** cria um desafio de pareamento e pergunta ao
próprio DNS se algum aparelho do perfil respondeu
(`GET /api/v1/pairing/challenges/{id}/outcome`, escopado por casa). Isso ainda
não está publicado no staging — exigira uma promoção `main → production`.

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
