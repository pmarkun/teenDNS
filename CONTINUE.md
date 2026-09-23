# Como continuar o trabalho do teenDNS

Última atualização: 23 de setembro de 2026.

## Objetivo atual

Preparar uma base real para identificar sites, páginas e funcionalidades que
não sejam adequados para pessoas de 12 a 13 anos. O foco deixou de ser apostas:
o catálogo deve cobrir principalmente conteúdo sexual, violência, drogas,
automutilação, suicídio, ódio, exploração e riscos de interação.

O teste técnico está sendo desenvolvido em outra task. Não alterar ou assumir
o trabalho dela sem antes verificar o estado atual do repositório.

## Decisão central

Usar os critérios oficiais brasileiros como taxonomia e as listas de domínios
como evidência secundária.

Para o público de 12 a 13 anos:

- `L`, `6`, `10` e `12`: compatíveis em princípio;
- `14`: primeiro limiar de conversa, pedido de contexto ou restrição;
- `16` e `18`: proteção forte, com exceção consciente e explicada.

Isso decorre do significado da classificação brasileira: `12` quer dizer “não
recomendado para menores de 12 anos”.

## Trabalho concluído nesta task

### Serviço de classificação de URLs

- `classifier/`: biblioteca Go com catálogo, extração de página, cliente Simple
  Jev, agregação das faixas e handler HTTP;
- `cmd/teendns-classify/`: CLI com os comandos `classify` e `serve`;
- `docs/classifier-service.md`: instruções de uso, segurança e limites;
- proteção básica contra SSRF habilitada por padrão;
- testes unitários para catálogo, extração, URL, loopback, cliente Simple Jev e
  agregação de classificação;
- fluxo real validado contra a demonstração pública do Simple Jev usando apenas
  `https://example.com`, com resultado esperado `unknown/observe`.

Comandos principais:

```sh
nix develop --command go test ./classifier ./cmd/teendns-classify
nix develop --command go run ./cmd/teendns-classify classify [flags] URL
nix develop --command go run ./cmd/teendns-classify serve [flags]
```

### Taxonomia etária

- `criteria/age-rating-v1.json`
  - JSON válido, verificado com `jq empty`;
  - 106 critérios;
  - quatro eixos: Violência, Sexo e nudez, Drogas e Interatividade;
  - sete faixas: `L`, `6`, `10`, `12`, `14`, `16` e `18`;
  - contém a regra inicial específica para 12-13 anos;
  - a numeração aparentemente incorreta do eixo Sexo e nudez no PDF foi
    normalizada segundo a seção/faixa em que cada critério aparece.

- `docs/criterios-classificacao-etaria.md`
  - explica a interpretação das faixas;
  - resume todos os critérios 14+;
  - propõe o formato de saída do futuro classificador;
  - diferencia conteúdo, funcionalidade, domínio e página;
  - registra as fontes normativas.

- `criteria/profile-12-13-v1.json`
  - transforma as faixas em ações padrão `allow`, `mediate`, `protect` e
    `observe`;
  - registra exceções de risco imediato, contexto e escopo.

- `criteria/priority-guidance-12-13-v1.json`
  - contém definições operacionais e sinais dos 52 critérios 14+;
  - todos os IDs foram confrontados com a taxonomia, sem duplicatas ou
    ausências;
  - o texto é paráfrase de produto, não transcrição normativa.

- `criteria/classification-result.schema.json`
  - formaliza o resultado auditável do futuro classificador;
  - foi validado como JSON Schema 2020-12 com uma amostra completa.

- `scripts/validate-age-criteria.py`
  - valida contagens, eixos, faixas e unicidade dos IDs;
  - exige cobertura exata dos 52 critérios 14+ pela orientação operacional;
  - confere a coerência básica do perfil e do contrato de saída;
  - usa somente a biblioteca padrão do Python.

Comando de verificação:

```sh
jq empty criteria/age-rating-v1.json
jq '[.axes[].bands[][]] | length' criteria/age-rating-v1.json
jq '[.guidance[].criteria[]] | length' criteria/priority-guidance-12-13-v1.json
uv run scripts/validate-age-criteria.py
```

Os dois comandos de contagem devem retornar `106` e `52`, respectivamente.

### Catálogo de domínios anterior

Foi criada uma semente em `catalog/v1/`, reproduzida por
`scripts/build-catalog-v1.py`. Ela contém:

- domínios adultos citados pela Ofcom e Comissão Europeia;
- apostas autorizadas pela SPA/MF;
- grandes plataformas sociais em modo de observação;
- lista comunitária de rastreamento em modo de observação.

Esse catálogo foi criado antes da correção de prioridade feita pelo usuário.
Não deve ser tratado como a lista final para 12-13 anos. Apostas e rastreamento
devem perder protagonismo; as categorias etárias e de dano devem orientar a
próxima versão.

## Fonte oficial já extraída

Guia utilizado:

- **Guia Prático de Classificação Indicativa para Audiovisual, Aplicativos e
  RPG - 5ª edição (2025)**, do Ministério da Justiça e Segurança Pública;
- página oficial:
  <https://www.gov.br/mj/pt-br/assuntos/seus-direitos/classificacao-1/paginas-classificacao-indicativa/guia-de-classificacao>;
- PDF baixado temporariamente em
  `tmp/pdfs/classind-audiovisual-2025.pdf`;
- SHA-256 do PDF:
  `b11060d63c9757d29b2052afa80ac056e34a5ab78e2b542391a60565d237b43e`;
- texto extraído temporariamente em
  `tmp/pdfs/classind-audiovisual-2025.txt`.

Os arquivos em `tmp/` são material de trabalho e não precisam ser
versionados. Não depender deles em produção.

Fontes legais complementares já identificadas:

- Portaria MJSP nº 1.048/2025;
- Lei nº 15.211/2025, o ECA Digital;
- Decreto nº 12.880/2026.

O guia atual acrescenta **Interatividade** aos eixos tradicionais. Isso é
essencial para o produto: comunicação sem proteção, compras, IA generativa,
localização, recomendação algorítmica, filtros de beleza, engajamento contínuo,
loot boxes e funcionalidades de relacionamento possuem critérios próprios.

## Fontes brasileiras encontradas

- A ANPD publicou uma lista inicial de 18 sites adultos monitorados que, segundo
  o órgão, representam 98% do tráfego brasileiro desse segmento. Essa deve ser
  a primeira fonte nacional oficial para `adult_explicit`.
- A SaferNet Brasil recebe denúncias e publica indicadores, mas não oferece uma
  blocklist aberta pronta para reutilização.
- O CERT.br produz orientação e resposta a incidentes, mas não uma taxonomia
  pública de domínios inadequados por idade.
- O projeto comunitário brasileiro `zangadoprojets/pi-hole-blocklist` tem
  categorias de pornografia e outros conteúdos, mas precisa de auditoria de
  proveniência, licença e falsos positivos antes de importação.
- Projetos brasileiros encontrados para apostas (`bloquear-bets` e
  `BlockBets`) são secundários para o objetivo atual.

Fontes internacionais mais alinhadas ao novo objetivo:

- UT1/Université Toulouse: categorias educacionais como adulto, drogas,
  violência/ódio, relacionamento e outras; licença e atualização precisam ser
  registradas por categoria.
- CommunityBlocklists: agrega dezenas de categorias, inclusive pornografia,
  violência, drogas e relacionamento; precisa de auditoria dos upstreams.
- Aegis Blocklist: pequena e focada em segurança infantil, incluindo
  automutilação, gore, riscos de predadores e drogas; possui decisões agressivas
  como bloquear plataformas de desenvolvimento inteiras, portanto não deve ser
  importada sem revisão item a item.
- Ofcom: o marco de segurança infantil prioriza pornografia, suicídio,
  automutilação e transtornos alimentares, mas nem todas as fontes são listas de
  domínios abertas.

## Próximos passos recomendados

### 1. Revisar e completar o enriquecimento dos 106 critérios

Os 52 critérios que acionam intervenção para 12-13 anos já possuem uma primeira
orientação operacional. Revisá-la contra cada seção do PDF e, depois, adicionar
aos 54 critérios compatíveis (`L` a `12`):

- definição operacional resumida;
- exemplos positivos e negativos;
- sinais observáveis em texto, imagem, metadados e funcionalidade;
- agravantes e atenuantes admitidos pelo guia;
- exceções educativas, jornalísticas, científicas e de saúde;
- nível mínimo de evidência para sugestão automática.

Manter separado o texto oficial, a paráfrase operacional do teenDNS e as regras
heurísticas. Não apresentar uma heurística própria como decisão do MJSP.

### 2. Implementar o contrato do classificador

O JSON Schema já está formalizado. Ao implementar, produzir uma saída com, no
mínimo:

- `rating`;
- `axis`;
- `criterion_id` e `criterion`;
- `confidence`;
- `evidence`;
- `context`;
- `scope` (`domain`, `site`, `page` ou `feature`);
- `source_url`;
- `review_status`;
- data e versão do modelo/taxonomia.

A saída deve aceitar múltiplos critérios e calcular a faixa final pelo maior
valor depois da análise contextual.

### 3. Refazer o catálogo real como v2

Organizar fontes por categoria e grau de confiança:

- `adult_explicit`;
- `sexual_services_or_dating`;
- `graphic_violence_or_gore`;
- `self_harm_or_suicide`;
- `eating_disorder_promotion`;
- `hate_or_extremism`;
- `illegal_drugs_promotion`;
- `unsafe_interaction`;
- `adult_commerce`;
- `gambling` como categoria secundária.

Para cada entrada registrar fonte, licença, data, categoria original,
categoria teenDNS, escopo, confiança, motivo e status de revisão.

### 4. Tratar corretamente plataformas mistas

Não classificar Reddit, YouTube, redes sociais, buscas ou IAs inteiras com base
no pior conteúdo hospedado. Para esses serviços, registrar risco de
interatividade e usar controles de página/plataforma:

- SafeSearch ou modo restrito;
- configuração parental;
- extensão do navegador;
- análise local da página;
- conversa ou pedido de acesso.

DNS deve bloquear apenas serviços cuja finalidade dominante sustente a
classificação em nível de domínio.

### 5. Validar antes de integrar

- comparar a taxonomia JSON com o sumário e as seções do PDF;
- revisar especialmente a numeração normalizada de Sexo e nudez;
- criar testes para contagem, unicidade de IDs, faixas válidas e maior faixa;
- revisar licença e proveniência de cada lista externa;
- medir falsos positivos com uma amostra estratificada;
- só depois conectar a taxonomia ao teste que está na outra task.

## Estado do Git e cuidado com trabalho paralelo

No último diagnóstico, `git status --short` mostrava diretórios não rastreados,
incluindo `catalog/`, `criteria/`, `tmp/` e `web/`, além dos documentos e scripts
criados no projeto. Parte desse estado pode pertencer à outra task.

Antes de editar ou versionar:

```sh
git status --short
git diff
find web -maxdepth 3 -type f -print
```

Preservar todo trabalho paralelo. Não adicionar `tmp/` ao commit. Não fazer
commit, push, merge ou deploy sem novo pedido explícito do usuário.
