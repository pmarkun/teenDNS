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
| `social-platforms.txt` | Registro Category 1 da Ofcom | `observe` | Portas de entrada canônicas de grandes serviços sociais, de mensageria, vídeo, fórum e jogo social. |
| `tracking-observe.txt` | The Block List Project | `observe` | Domínios técnicos associados a rastreamento ou analytics. Não devem ser bloqueados por padrão na v1. |

`unknown` não possui arquivo: é o estado calculado quando não há correspondência
no catálogo.

## Decisões da v1

- As listas regulatórias são pequenas e auditáveis. Elas formam o núcleo de
  alta confiança.
- A lista de rastreamento é ampla e comunitária. Ela serve inicialmente para
  observação e para testar quebra de sites antes de qualquer bloqueio.
- A correspondência inicial deve ser exata. Incluir subdomínios é uma decisão
  separada da política, porque bloquear uma zona inteira aumenta muito o risco
  de falso positivo.
- Domínios auxiliares, CDNs e APIs não foram inferidos a partir do nome de um
  serviço. Eles só devem entrar quando houver fonte ou validação específica.
- O catálogo descreve o domínio; a política familiar decide `allow`, `observe`
  ou `block` e pode criar exceções mais específicas.

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
domínios, rejeita entradas inválidas e grava contagens, URLs, datas e hashes em
`manifest.json`. `SHA256SUMS` permite conferir os artefatos gerados.

As relações manuais da Ofcom e da Comissão Europeia ficam no script porque
essas páginas não oferecem uma API ou arquivo de domínios apropriado. Ao
atualizá-las, a revisão deve confrontar diretamente as páginas regulatórias.
