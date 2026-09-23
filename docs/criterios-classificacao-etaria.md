# Critérios de classificação etária para o teenDNS

Esta taxonomia traduz para o projeto o **Guia Prático de Classificação
Indicativa para Audiovisual, Aplicativos e RPG - 5ª edição (2025)**, do
Ministério da Justiça e Segurança Pública. Os artefatos estruturados são:

- [`age-rating-v1.json`](../criteria/age-rating-v1.json): os 106 critérios
  oficiais organizados por eixo e faixa;
- [`profile-12-13-v1.json`](../criteria/profile-12-13-v1.json): política de
  intervenção para pessoas de 12 a 13 anos;
- [`priority-guidance-12-13-v1.json`](../criteria/priority-guidance-12-13-v1.json):
  definições e sinais operacionais dos 52 critérios de 14, 16 e 18 anos;
- [`classification-result.schema.json`](../criteria/classification-result.schema.json):
  contrato de saída para o futuro classificador.

Ela não afirma que todo site brasileiro recebe classificação individual do
MJSP. O objetivo é usar o vocabulário e os limiares oficiais como referência
para um classificador próprio, com indicação explícita da fonte e possibilidade
de contestação.

## Regra para o público de 12 a 13 anos

Na classificação brasileira, `12` significa "não recomendado para menores de
12 anos". Portanto, uma política dirigida a pessoas de **12 e 13 anos** pode
considerar compatíveis as faixas `L`, `6`, `10` e `12`. O primeiro limiar de
intervenção é `14`.

```text
L, 6, 10, 12  -> compatível com a faixa de 12-13 anos
14             -> conversar, pedir contexto ou restringir conforme confiança
16, 18         -> proteção forte; exceção consciente e explicada
```

Essa regra é uma política inicial do produto, não uma determinação de bloqueio
automático. Conteúdo educativo, jornalístico, artístico, científico ou de saúde
pode exigir tratamento contextual diferente.

## Quatro eixos oficiais

| Eixo | O que observa |
| --- | --- |
| Violência | agressões, sofrimento, medo, discriminação, exploração, suicídio e violência gráfica |
| Sexo e nudez | sexualização, nudez, atos sexuais e situações sexuais de maior impacto |
| Drogas | menção, consumo, indução, apologia, produção e tráfico; o guia também enquadra jogos de azar neste eixo |
| Interatividade | comunicação, comércio, IA generativa, dados, localização, recomendação algorítmica, engajamento e funcionalidades adultas |

A classificação resultante deve ser a **maior faixa identificada entre os
eixos**, depois de considerar contexto, intensidade, frequência, composição de
cena, motivação, contraponto, simulação e tentativa. A ausência de evidência
não equivale a conteúdo livre.

## Limiar prático: critérios 14+

Estes são os primeiros critérios incompatíveis com uma política para 12-13
anos. As faixas 16 e 18 são cumulativamente mais restritivas.

### Não recomendado para menores de 14 anos

- **Violência:** aborto, autópsia, glamourização do consumo, bullying ou
  cyberbullying, estigma ou preconceito, eutanásia ou suicídio assistido,
  exploração sexual, medo ou tensão intensos, morte intencional, pena de morte
  e tráfico de pessoas.
- **Sexo e nudez:** erotização, nudez, prostituição, relação sexual e
  vulgaridade.
- **Drogas:** apologia a droga lícita, consumo insinuado de droga ilícita,
  descrição de consumo ou tráfico de droga ilícita e prática de jogos de azar.
- **Interatividade:** compras on-line ou troca de produtos, comunicação direta
  sem proteção padrão e IA generativa de conteúdo amplo.

### Não recomendado para menores de 16 anos

- **Violência:** glamourização de padrões estéticos perigosos, crime de ódio,
  estupro ou coação sexual, mutilação, suicídio, tortura, violência gratuita e
  violência sexual contra crianças ou adolescentes.
- **Sexo e nudez:** relação sexual intensa.
- **Drogas:** consumo ou indução ao consumo de droga ilícita e produção ou
  tráfico de droga ilícita.
- **Interatividade:** compartilhamento de dados ou localização, curadoria
  algorítmica com engajamento direcionado, filtros de beleza automatizados e
  mecanismos de engajamento contínuo.

### Não recomendado para menores de 18 anos

- **Violência:** apologia ou glamourização da violência e violência de forte
  impacto.
- **Sexo e nudez:** sexo explícito e situação sexual complexa ou de forte
  impacto.
- **Drogas:** apologia ao uso de droga ilícita e estímulo a apostas ou jogos de
  azar.
- **Interatividade:** manipulação digital com conteúdo sensível, aposta ou
  simulação de jogo de azar, interações com potencial de dano, relacionamentos,
  loot boxes e publicidade ou venda de conteúdo adulto.

## Como transformar isso em classificador

O classificador não deve produzir apenas uma faixa. Cada decisão precisa
registrar:

```json
{
  "rating": 16,
  "axis": "violence",
  "criterion_id": "A.6.5",
  "criterion": "Suicídio",
  "confidence": 0.91,
  "evidence": ["trecho, imagem, metadado ou funcionalidade observada"],
  "context": ["educational", "news"],
  "scope": "page",
  "source_url": "https://exemplo.invalid/pagina",
  "review_status": "machine_suggestion"
}
```

O JSON Schema do projeto formaliza esse contrato. A orientação prioritária é
deliberadamente uma paráfrase operacional: serve para desenho e avaliação do
classificador, mas não deve ser apresentada como transcrição normativa do
MJSP.

Regras de modelagem:

1. **Classificar a evidência, não a reputação do domínio.** Um domínio de
   conteúdo misto pode conter páginas `L` e `18`.
2. **Separar conteúdo de funcionalidade.** Uma página pode ser editorialmente
   segura e ainda ter chat aberto, compras, localização ou recomendação
   compulsiva.
3. **Manter múltiplos rótulos.** A saída final usa a maior faixa, mas preserva
   todos os critérios detectados.
4. **Usar `unknown` quando faltar evidência.** Não inferir `L` a partir da
   ausência de correspondência.
5. **Separar sugestão automática de decisão revisada.** Fonte, data, escopo,
   evidência e confiança devem ser auditáveis.
6. **Aplicar contexto antes da intervenção.** Educação sexual, prevenção ao uso
   de drogas, jornalismo e saúde não devem ser confundidos com promoção ou
   exploração.
7. **Não usar DNS para alegações em nível de página.** O DNS só sustenta
   classificação de serviços dedicados; plataformas mistas exigem extensão,
   integração com a plataforma ou análise local da página.

## Consequência para a lista de domínios

A lista real deve priorizar serviços cuja finalidade dominante já corresponda a
um critério 14, 16 ou 18, por exemplo pornografia explícita, encontros sexuais,
apostas, promoção de drogas, gore ou comunidades dedicadas a automutilação.

Redes sociais, mecanismos de busca, plataformas de vídeo, fóruns e serviços de
IA não devem ser classificados integralmente pelo pior conteúdo que hospedam.
Para esses casos, a lista registra **risco de interatividade** e aciona medidas
como conversa, modo seguro, análise da página ou configuração parental.

## Fonte normativa e limites

- [Guia de Classificação do MJSP](https://www.gov.br/mj/pt-br/assuntos/seus-direitos/classificacao-1/paginas-classificacao-indicativa/guia-de-classificacao)
- [Portaria MJSP nº 1.048/2025](https://anttlegis.antt.gov.br/action/ActionDatalegis.php?acao=abrirTextoAto&cod_menu=7145&cod_modulo=420&desItem=&desItemFim=&nomeTitulo=codigos&numeroAto=00001048&orgao=MJSP&seqAto=000&tipo=POR&valorAno=2025)
- [Lei nº 15.211/2025 - ECA Digital](https://www2.camara.leg.br/legin/fed/lei/2025/lei-15211-17-setembro-2025-797997-normaatualizada-pl.html)

O ECA Digital exige proteção contra conteúdo impróprio ou inadequado, mas não
fornece uma lista pública completa de domínios por faixa. A taxonomia acima é
uma base de decisão; o catálogo técnico continuará precisando de fontes,
evidências e revisão próprias.
