# teenDNS — controle parental que inicia conversas

Documento de resgate do conceito discutido em 23 de setembro de 2026. Nome de trabalho usado na conversa original: **Farol da Rede**.

Documentação técnica:

- [Plano de execução](docs/plano-execucao.md)
- [Laboratório local](docs/laboratorio.md)

## Ideia central

O projeto não é apenas “um DNS que bloqueia melhor”. É uma ferramenta de controle parental que transforma intervenções em oportunidades de mediação e educação midiática.

> Controle parental costuma terminar a conversa com uma tela de bloqueio. O teenDNS usa cada intervenção para começar uma conversa sobre como a internet funciona.

Em vez de entregar apenas “bloqueado” ou “proibido”, o produto explica o que motivou a intervenção, oferece alternativas e permite que a criança ou adolescente peça acesso, dê contexto ou conteste uma classificação equivocada.

O objetivo de longo prazo é formar autonomia para decisões futuras, não ampliar a vigilância familiar.

## Princípios

- **Mediação, não só restrição:** explicar antes de encerrar a experiência.
- **Autonomia progressiva:** ajudar crianças e adolescentes a compreender e tomar decisões.
- **Privacidade familiar:** visão agregada e processamento local sempre que possível.
- **Contestação:** toda classificação ou regra pode estar errada.
- **Acordos, não apenas ordens:** regras familiares devem poder ser conversadas e revisadas.
- **Sem espionagem:** não ler mensagens, registrar páginas completas, fazer capturas de tela ou produzir um histórico detalhado para responsáveis.
- **Proteção proporcional:** bloquear somente o que tiver motivo e regra compreensíveis.

## Os quatro modos do produto

### Observar

Mostrar categorias e características do ambiente digital sem expor cada página, busca ou conversa.

Exemplos de observações:

- “Este aplicativo tentou falar com 14 serviços de publicidade.”
- “Quase toda a sua atividade social depende de uma única empresa.”
- “Hoje apareceram cinco domínios que ainda não conhecemos.”
- “Este serviço não oferece uma forma clara de apagar seus dados.”

### Conversar

Transformar eventos e padrões em cartões curtos de educação midiática: rastreamento, publicidade, padrões manipulativos, privacidade, desinformação, apostas, conteúdo adulto e concentração de plataformas.

### Combinar

Permitir que a família crie acordos digitais, horários, categorias protegidas e exceções explicáveis. As regras devem registrar um motivo legível e poder ser revisitadas.

### Proteger

Bloquear ou sinalizar domínios e categorias definidos como perigosos, sem fazer da restrição a única experiência do produto.

## Experiência de uma intervenção

Quando um endereço for sinalizado, a experiência ideal é:

> Este endereço foi interrompido porque tenta acompanhar sua atividade em vários sites.

A pessoa pode então:

- entender como esse rastreamento funciona;
- encontrar uma alternativa;
- pedir acesso;
- conversar sobre a regra;
- informar que o bloqueio parece errado.

Um pedido de acesso pode incluir contexto simples, por exemplo: “Preciso abrir porque minha escola enviou este link.” Isso cria diálogo e ajuda a corrigir falsos positivos.

## Arquitetura proposta para o primeiro protótipo

O DNS permanece como infraestrutura de proteção. Uma extensão de navegador entrega a intervenção pedagógica de forma visual, e um painel local apresenta padrões agregados e acordos familiares.

```text
navegação
   │
   ├── DNS local ── classificação e proteção por domínio
   │
   ├── extensão ─── explicação, alternativa, pedido e contestação
   │
   └── painel ───── visão agregada, cartões e acordos familiares
```

### Por que não usar somente DNS

O DNS enxerga domínios, não a página ou o conteúdo acessado. Com HTTPS, um bloqueio também não pode ser redirecionado de forma universal para uma página educativa: em muitos casos o navegador mostraria apenas um erro de conexão ou certificado.

Por isso, a combinação DNS + extensão é o melhor recorte para demonstrar o diferencial do produto:

- o DNS prova que existe uma camada real de proteção;
- a extensão torna a intervenção visível e compreensível;
- o painel organiza reflexão e acordos;
- o processamento pode permanecer local.

## Escopo recuperado para um MVP solo

1. Uma extensão intercepta cinco domínios ou cenários de demonstração.
2. Cada cenário representa uma categoria: rastreamento, rede social, aposta, conteúdo adulto ou desconhecido.
3. A tela intermediária explica o motivo da intervenção em linguagem adequada.
4. A pessoa pode aprender, voltar, pedir exceção ou contestar.
5. Um painel mostra apenas eventos agregados por categoria.
6. Responsável e criança ou adolescente criam juntos um acordo digital.
7. Um pequeno resolvedor DNS funcional demonstra a camada de proteção.
8. Tudo fica armazenado localmente; não há conta nem backend no primeiro protótipo.

Não é necessário construir infraestrutura DNS de produção para validar a proposta.

## Contraste com controle parental tradicional

| Controle tradicional | teenDNS / Farol da Rede |
| --- | --- |
| Vigilância detalhada | Visão agregada e local |
| Regra imposta | Acordo construído |
| Bloqueio silencioso | Explicação compreensível |
| Criança sem recurso | Pedido e contestação |
| Foco apenas em tempo de tela | Foco em como a rede funciona |
| Histórico para o responsável | Informação mínima para a conversa |
| Dependência de uma empresa | Software aberto e local |

## Direção visual originalmente sugerida

A primeira metáfora foi um **jardim ou mapa da vida digital**, com regiões para conversa, estudo, jogos, entretenimento, redes sociais, publicidade e rastreamento, além de conteúdo desconhecido ou potencialmente arriscado.

O mapa muda conforme surgem padrões, mas não dá nota de “bom” ou “ruim” para a criança. Ele descreve o ambiente e torna relações invisíveis compreensíveis.

Essa direção ainda não foi validada e pode mudar. O princípio importante é tornar visíveis a infraestrutura e os motivos da intervenção, sem transformar o painel em ferramenta de vigilância.

## Questões ainda abertas

- Qual faixa etária deve orientar linguagem, autonomia e participação do responsável?
- O primeiro uso será doméstico, escolar ou ambos?
- Quais categorias justificam bloqueio imediato e quais devem gerar apenas reflexão?
- Como entregar a explicação fora do navegador, em aplicativos móveis?
- Como impedir que responsáveis convertam o produto em monitoramento detalhado?
- Como tratar pedidos urgentes, ausência do responsável e falsos positivos?
- Como definir e revisar fontes de classificação de domínios e materiais educativos?
- O produto deve se chamar teenDNS, Farol da Rede ou outro nome?

## Origem e separação de escopo

O conceito foi recuperado de uma conversa sobre projetos para a Hackathon ARTIGO 19. Depois dele, a conversa seguiu por outro ramo — um diagnóstico de dependência de big techs chamado **Mapa de Autonomia Digital**. Esse segundo projeto usa histórico do navegador e sugere alternativas a serviços digitais, mas não é o teenDNS e não foi incorporado aqui.
