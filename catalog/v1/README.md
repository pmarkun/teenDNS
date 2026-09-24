# Catálogo real v1

Este diretório contém a primeira lista real de domínios do teenDNS. Ela é uma
semente de produto, não uma afirmação de que o Ministério da Justiça atribuiu
classificação indicativa individual a cada domínio.

As listas ficam separadas por origem e grau de confiança para evitar que uma
fonte comunitária seja apresentada como decisão de um órgão público.

| Arquivo | Origem | Ação padrão | Interpretação |
| --- | --- | --- | --- |
| `gambling-br-authorized.txt` | SPA/MF | `block` | Domínios autorizados a oferecer apostas no Brasil. A categoria é aposta; a presença na lista não significa ilegalidade. |
| `adult-content-regulators.txt` | Ofcom e Comissão Europeia | `block` | Serviços identificados por reguladores como serviços adultos ou plataformas pornográficas. |
| `security-threats.txt` | The Block List Project | futuro matcher global | Domínios associados a phishing ou ransomware; ainda não são copiados para políticas por perfil. |
| `services/*.txt` | Documentação oficial dos serviços | `observe` | Pools pequenos para controlar um serviço específico no navegador e no app sem incluir infraestrutura compartilhada. |

Os metadados ficam em:

- `service-pools.json`: tema, arquivo, evidências e dependências compartilhadas
  deliberadamente excluídas de cada serviço;
- `presets.json`: pontos de partida **Acompanhado**, **Explorando** e
  **Autonomia guiada**, sem armazenar idade ou data de nascimento;
- `manifest.json`: origem, licença, data de coleta, hash e contagem de todas as
  listas geradas.

## Pools por serviço

Cada arquivo em `services/` contém apenas sufixos considerados específicos do
provedor. A regra de aplicação é **sufixo DNS**, incluindo o próprio domínio:
`cdninstagram.com` corresponde também a `scontent.cdninstagram.com`.

| Tema | Serviços iniciais |
| --- | --- |
| Redes sociais | Facebook, Instagram, Reddit, Snapchat, Threads e X |
| Vídeo social | TikTok, Twitch e YouTube |
| Mensageria e comunidades | Discord, Telegram e WhatsApp |
| Jogos com interação social | Roblox |

Os pools têm ação `observe` por padrão. Eles existem para uma decisão explícita
da casa — por exemplo, pausar TikTok ou Instagram — e não porque o catálogo
classifique todo o conteúdo do serviço como inadequado.

### Por que há domínios excluídos

Aplicativos grandes usam infraestrutura que também atende outros produtos. O
Instagram, por exemplo, depende de domínios compartilhados da Meta; o YouTube
depende de domínios genéricos do Google; TikTok e Roblox usam redes e nuvens que
podem hospedar terceiros. Esses sufixos aparecem em
`shared_dependencies_excluded`, mas **não** nas listas de bloqueio.

Bloqueá-los aumentaria a cobertura, porém poderia quebrar WhatsApp, busca,
login, notificações, outros jogos ou sites sem relação com a escolha feita. A
v1 prefere uma pequena chance de passagem residual a um sobrebloqueio invisível.
Se um app continuar operando, a próxima evidência deve vir de teste de rede em
aparelho real antes de promover um domínio compartilhado para o pool.

## Presets conscientes

Os presets são pontos de partida editáveis, não diagnósticos nem classificações
da pessoa:

- **Acompanhado:** bloqueia por padrão plataformas sociais e vídeo social;
  mensageria, comunidades e jogos sociais ficam em observação para preservar
  contato, escola e brincadeira combinada.
- **Explorando:** mantém serviços mistos em observação e protege contra adulto
  e apostas.
- **Autonomia guiada:** permite serviços mistos e mantém as proteções essenciais;
  a conversa parte do uso observado e das escolhas da própria casa.

Em todos eles, adulto e apostas começam bloqueados. Phishing e ransomware estão
catalogados para uma futura camada técnica global, ainda não aplicada aos
perfis. Uma família pode alterar qualquer ação ou serviço sem trocar de preset.

`unknown` não possui arquivo: é o estado calculado quando não há correspondência
no catálogo.

## Decisões da v1

- As listas regulatórias são pequenas e auditáveis. Elas formam o núcleo de
  alta confiança.
- A correspondência inicial deve ser exata. Incluir subdomínios é uma decisão
  separada da política, porque bloquear uma zona inteira aumenta muito o risco
  de falso positivo.
- Domínios auxiliares, CDNs e APIs entram apenas com documentação do provedor e
  revisão manual. Infraestrutura compartilhada é registrada, mas não bloqueada.
- O catálogo descreve o domínio; a política familiar decide `allow`, `observe`
  ou `block` e pode criar exceções mais específicas.
- Um pool de serviço é melhor esforço. Cache, conexão já aberta, VPN, DNS próprio
  do app e mudanças de infraestrutura ainda podem permitir tráfego residual.
- O catálogo não trata rede social, jogo ou mensageria como dano por si só. A
  intervenção é proporcional, reversível e explicável.

## Fontes avaliadas, mas não incorporadas

### INTERPOL IWOL

A INTERPOL mantém a *Worst of List* de domínios que distribuem material grave
de abuso sexual infantil. A lista é oferecida a parceiros, especialmente ISPs,
e não é publicada para download aberto. Deve ser tratada como possível parceria
futura, não copiada de fontes secundárias.

### Internet Watch Foundation

A URL List da IWF é licenciada a membros e atualizada duas vezes por dia. A
maior parte das entradas é URL, não domínio. Aplicá-la diretamente em DNS
causaria sobrebloqueio; a própria IWF recomenda bloqueio de domínio somente
quando o site inteiro foi avaliado como criminoso.

### Project Arachnid

O Shield do Canadian Centre for Child Protection oferece hashes de imagens e
vídeos para moderação de plataformas. É importante para serviços que hospedam
conteúdo, mas não é uma lista de domínios adequada ao resolvedor DNS.

### SaferNet Brasil e CERT.br

São referências nacionais importantes para denúncias, resposta a incidentes e
educação, mas não publicam uma lista aberta de domínios categorizados que possa
ser incorporada a este catálogo.

### UT1 Blacklists

A Université Toulouse Capitole publica uma lista adulta sob CC BY-SA. A lista
tem milhões de entradas e exige avaliação de tamanho, atribuição, atualização e
falsos positivos antes de entrar no produto. A v1 prefere o conjunto menor de
serviços explicitamente identificados por reguladores.

### DuckDuckGo Tracker Radar e Disconnect

São fontes tecnicamente maduras, mas usam CC BY-NC-SA. A restrição não comercial
é incompatível com manter aberta a possibilidade de operação comercial do
teenDNS sem obter licença adicional. Por isso não foram copiadas.

## Reprodução

O catálogo é reconstruído apenas com a biblioteca padrão do Python:

```sh
uv run scripts/build-catalog-v1.py
```

O script baixa os registros públicos que possuem formato de dados, normaliza os
domínios, rejeita entradas inválidas, valida a separação dos pools e grava
contagens, URLs, datas e hashes em `manifest.json`. `SHA256SUMS` permite conferir
os artefatos gerados.

As relações manuais da Ofcom e da Comissão Europeia ficam no script porque
essas páginas não oferecem uma API ou arquivo de domínios apropriado. Ao
atualizá-las, a revisão deve confrontar diretamente as páginas regulatórias.
Os pools de serviço seguem a mesma regra: são uma compilação factual pequena a
partir de documentação oficial, não uma cópia de listas comunitárias com licença
incerta. A metodologia e o protocolo de teste estão em
[`docs/metodologia-catalogo-v1.md`](../../docs/metodologia-catalogo-v1.md).
