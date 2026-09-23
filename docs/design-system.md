# Sistema visual do teenDNS

Direção aprovada para o painel administrativo e para a página pública do MVP.
Os conceitos de referência estão em [`design/concepts`](../design/concepts/).

## Ideia visual

O teenDNS deve parecer uma mistura de cartilha cívica brasileira e instrumento
de rede doméstico: claro, direto e humano. A interface usa hierarquia editorial,
linhas, tabelas e anotações. Ela não imita o padrão visual de produtos de IA.

Evitar:

- gradientes roxos ou azuis, brilhos e transparências de vidro;
- grades de cartões arredondados e painéis dentro de painéis;
- robôs, estrelas, órbitas ou ilustrações genéricas de IA;
- métricas fictícias, selos decorativos e textos promocionais vagos;
- linguagem que trate a criança como objeto de vigilância.

## Cores

| Token | Valor | Uso |
| --- | --- | --- |
| `paper` | `#f3f0e8` | fundo principal, sem alterar a temperatura |
| `ink` | `#111713` | texto e linhas principais |
| `forest` | `#0c4a35` | estados permitidos e faixas explicativas |
| `signal` | `#f04422` | ação principal, bloqueio e numeração |
| `note` | `#f6dda0` | notas educativas e seleção discreta |
| `muted` | `#61665f` | texto secundário |
| `line` | `#9b9b91` | divisórias finas |

## Tipografia

- Títulos editoriais: `Source Serif 4`, `Georgia`, serif.
- Interface e texto: `Inter`, `Arial`, sans-serif.
- Hostnames, rótulos técnicos e notas: `IBM Plex Mono`, `Courier New`, monospace.
- Controles nunca dependem da tipografia padrão do navegador.

## Geometria e espaçamento

- Raios entre `0` e `4px`; botões de ação podem ser totalmente retos.
- Bordas de `1px`, sombras apenas quando representam papel sobre papel.
- Escala de espaço: `4, 8, 12, 16, 24, 32, 48, 72px`.
- Conteúdo público limitado a `1312px`; painel ocupa a largura disponível.
- Componentes principais são trilhos, listas, tabelas, faixas e gavetas.

## Componentes

- **Marca:** farol geométrico com dois feixes na cor de sinalização.
- **Botão primário:** fundo `signal`, texto branco, altura mínima de `48px`.
- **Botão secundário:** fundo transparente, borda `ink`.
- **Links:** sublinhado fino, seta SVG quando indicam avanço.
- **Tabela de regras:** linhas abertas, cabeçalho monoespaçado, ações editáveis.
- **Estado:** ponto preenchido mais texto; não usar selo em formato de pílula.
- **Gaveta de edição:** painel lateral com borda, motivo obrigatório e ações claras.
- **Foco:** contorno de `3px` em `note`, com contraste sobre todos os fundos.

## Movimento

- Transições de `140–220ms`, sem elasticidade.
- Gavetas entram horizontalmente e mudanças salvas recebem confirmação curta.
- `prefers-reduced-motion` elimina movimentos sem remover feedback de estado.

## Responsividade

- Em telas menores que `900px`, o trilho de perfis vira uma faixa horizontal.
- Tabelas de regra viram uma lista linear, preservando motivo e ação.
- A página pública mantém a ordem editorial; diagramas se reorganizam em coluna.
- Nenhuma ação principal depende de hover.

## Cópia permitida no primeiro viewport

### Página pública

- `teenDNS`
- `Como funciona`, `Princípios`, `Configurar`, `abrir painel`
- `Controle parental que começa uma conversa.`
- `O teenDNS protege no nível da rede, explica os motivos e ajuda famílias a construir acordos — sem transformar cuidado em vigilância.`
- `entender como funciona`, `configurar no Android`

### Painel

- `teenDNS`, `Como funciona`, `Privacidade`, `DNS ativo`
- `Perfis`, `Casa`, `Estudos`, `+ novo perfil`
- `Regras da Casa`
- `Proteção que explica, acordos que podem mudar.`
- `DNS privado`, `copiar`, `regenerar`
- `Acordos e exceções`, `adicionar regra`, `O que aconteceu`

## Referências de implementação

- [`admin-dashboard.png`](../design/concepts/admin-dashboard.png): viewport `1440 × 900`.
- [`explainer-page.png`](../design/concepts/explainer-page.png): viewport `1440 × 1100`.

As imagens são especificações visuais, não assets de interface. Textos, controles,
ícones e diagramas devem continuar acessíveis e nativos em HTML, CSS e SVG.
