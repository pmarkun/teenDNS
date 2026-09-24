# Metodologia do catálogo v1

Este documento explica como a semente do teenDNS vira uma política utilizável
sem transformar proteção em vigilância ou toda atividade juvenil em risco.

## Princípios de produto

1. **Proteger acesso, não punir curiosidade.** Conteúdo adulto dedicado, apostas
   e ameaças técnicas recebem proteção forte. Plataformas de uso misto começam
   em observação ou são uma escolha explícita da casa.
2. **Preservar participação.** Mensageria, vídeo, comunidade e jogo também são
   espaços de amizade, criação, estudo e expressão. O Comentário Geral nº 25 da
   ONU destaca que acesso digital significativo viabiliza direitos e
   participação; proteção não deve significar exclusão automática.
3. **Intervenção compreensível e reversível.** A pessoa vê o nome macro da regra
   e o motivo. A casa consegue permitir, observar, bloquear e restaurar o ponto
   de partida.
4. **Sem perfil etário oculto.** A casa escolhe Acompanhado, Explorando ou
   Autonomia guiada. O sistema não precisa guardar nascimento nem inferir idade.
5. **Menor dano colateral.** Infraestrutura de nuvem, login, push e CDN usada por
   vários serviços não entra silenciosamente em um bloqueio específico.

Esses princípios acompanham o ECA Digital, que distribui responsabilidade entre
família, sociedade, Estado e plataformas, e a abordagem Safety by Design, que
coloca segurança e direitos no desenho inicial em vez de responsabilizar apenas
o usuário.

## Tipos de lista e confiança

| Tipo | Evidência | Uso na v1 |
| --- | --- | --- |
| Registro oficial | CSV da Secretaria de Prêmios e Apostas | bloqueio de apostas |
| Identificação regulatória | Ofcom e Comissão Europeia | bloqueio de serviços adultos dedicados |
| Lista comunitária licenciada | The Block List Project, MIT | catálogo técnico de phishing/ransomware para uma futura camada global |
| Pool de serviço | documentação do próprio provedor + revisão manual | controle seletivo; observação por padrão |

Os nomes de domínio nos pools são fatos técnicos compilados manualmente. Não foi
copiada uma lista de firewall de terceiros. A URL de evidência de cada pool fica
em `catalog/v1/service-pools.json`.

## Pool exclusivo versus dependência compartilhada

Um domínio entra no arquivo `services/<serviço>.txt` quando:

- é um sufixo controlado ou identificado pelo próprio provedor;
- representa entrada, API, mídia ou CDN relevante ao serviço;
- bloquear o sufixo inteiro não tem risco conhecido de atingir muitos terceiros;
- existe evidência oficial auditável, ainda que a lista completa seja uma
  compilação manual.

Uma dependência fica apenas em `shared_dependencies_excluded` quando atende mais
de um produto ou cliente. Exemplos importantes:

| Serviço | Pool aplicado | Compartilhados excluídos e risco |
| --- | --- | --- |
| Instagram | `instagram.com`, `cdninstagram.com`, `ig.me` | `fbcdn.net`, `fbsbx.com`, `facebook.com`: pode afetar Facebook, Messenger e WhatsApp |
| TikTok | raízes TikTok e Musical.ly | ByteDance genérico e Akamai: pode atingir outros produtos e clientes |
| YouTube | YouTube, `ytimg.com`, `googlevideo.com` e endpoint YouTube API | Google genérico: busca, login, Android e inúmeros sites |
| Roblox | `roblox.com`, `robloxapi.com`, `rbxcdn.com` | AWS, CloudFront e Google: infraestrutura multi-inquilino |
| Discord | raízes Discord | Google genérico usado por push e componentes de muitos apps |

Essa distinção é deliberadamente conservadora. Um pool pode deixar uma função
residual operando; adicionar uma nuvem inteira para obter “100% de bloqueio” não
é aceitável sem mostrar claramente o impacto.

## Presets

Os três presets combinam **temas**, não uma lista de sites “bons” e “ruins”:

| Tema | Acompanhado | Explorando | Autonomia guiada |
| --- | --- | --- | --- |
| Conteúdo adulto dedicado | bloquear | bloquear | bloquear |
| Apostas | bloquear | bloquear | bloquear |
| Redes sociais | bloquear | observar | permitir |
| Vídeo social | bloquear | observar | permitir |
| Mensageria e comunidades | observar | observar | permitir |
| Jogos com interação social | observar | observar | permitir |

“Bloquear redes sociais” aplica os pools específicos e permite exceções por
serviço. Não inclui WhatsApp, Discord, Telegram ou Roblox, que pertencem a temas
separados justamente para que contato e jogo não desapareçam como efeito
colateral.

Phishing e ransomware não fazem parte dos pacotes por perfil. A lista técnica
fica versionada como insumo para um matcher global futuro, evitando duplicar
centenas de milhares de domínios em cada casa.

## Protocolo para promover domínios

Antes de ampliar um pool:

1. confirmar o domínio em documentação oficial ou resposta de rede do app;
2. testar instalação já autenticada e sessão nova, em Wi-Fi e rede móvel;
3. fechar o app, limpar conexão/cache relevante e produzir conteúdo novo;
4. verificar navegador e app separadamente;
5. testar serviços vizinhos do mesmo fornecedor;
6. registrar aparelho, sistema, versão do app, data e região;
7. classificar como exclusivo ou compartilhado e explicar o possível dano;
8. repetir depois de pelo menos uma atualização do app antes de tratá-lo como
   dependência estável.

Uma captura local de DNS revela nomes consultados, mas não conteúdo nem intenção.
Ela não deve ser armazenada como histórico pessoal por padrão. Para o catálogo,
basta registrar a evidência técnica agregada e descartar identificadores do
dispositivo.

## Limites operacionais

- DNS não encerra conexões existentes e não apaga IPs já em cache.
- Um app pode usar DNS-over-HTTPS próprio, VPN, IP literal ou infraestrutura nova.
- Bloqueio por domínio não distingue feed, mensagem, notícia, aula ou saúde.
- CDNs podem mudar sem aviso; por isso os pools são versionados e testáveis.
- `observe` deve gerar informação proporcional, não um dossiê de navegação.

## Fontes primárias

### Direitos, hábitos e segurança

- [ECA Digital — Ministério da Justiça e Segurança Pública](https://www.gov.br/mj/pt-br/assuntos/sua-protecao/sedigi/eca-digital)
- [Lei nº 15.211/2025 — Presidência da República](https://planalto.gov.br/ccivil_03/_ato2023-2026/2025/lei/l15211.htm)
- [Comentário Geral nº 25 — UNICEF](https://www.unicef.org/turkiye/en/reports/un-committee-rights-child-childrens-rights-digital-environments)
- [Children and Parents: Media Use and Attitudes 2025–6 — Ofcom](https://www.ofcom.org.uk/siteassets/resources/documents/research-and-data/media-literacy-research/children/2026-children-and-parents-report/children-and-parents-media-use-and-attitudes-report-2025-6.pdf)
- [Safety by Design — eSafety Commissioner](https://www.esafety.gov.au/industry/safety-by-design)

### Evidência técnica dos pools

- [Instagram Platform — Meta for Developers](https://developers.facebook.com/docs/instagram-platform/)
- [WhatsApp Business Platform — Meta for Developers](https://developers.facebook.com/docs/whatsapp/)
- [Threads API — Meta for Developers](https://developers.facebook.com/docs/threads/)
- [Content Posting API — TikTok for Developers](https://developers.tiktok.com/doc/content-posting-api-get-started/)
- [YouTube embeds — Google Help](https://support.google.com/youtube/answer/171780)
- [Google hostname allowlist](https://support.google.com/a/answer/6334001)
- [Snap for Developers](https://developers.snap.com/)
- [X for Websites](https://developer.x.com/en/docs/x-for-websites/overview)
- [Reddit Developer Platform](https://developers.reddit.com/docs/capabilities/server/splash-screen)
- [Discord domain migration and CDN](https://support.discord.com/hc/en-us/articles/360042987951-Discordapp-com-is-now-Discord-com)
- [Roblox Open Cloud](https://create.roblox.com/docs/cloud/open-cloud)
- [Telegram deep links](https://core.telegram.org/api/links)
- [Twitch embeds](https://dev.twitch.tv/docs/embed/)

As páginas oficiais normalmente documentam apenas a superfície pública ou de
desenvolvedor, não todos os endpoints privados do app. Por isso elas sustentam
uma semente auditável, mas não a alegação de cobertura completa.
