# teenDNS

**Cuidar da internet sem transformar cuidado em vigilância.**

O teenDNS é uma ferramenta para famílias combinarem limites de uso da internet
sem depender dos controles isolados de cada aplicativo e sem ler mensagens ou
páginas. As regras acompanham o perfil da pessoa, não um aparelho específico,
e são aplicadas a domínios no momento em que o dispositivo os consulta.

## Por que existe

Os controles parentais costumam estar espalhados entre plataformas, com
configurações diferentes e pouca portabilidade. Uma regra definida em um
aplicativo não acompanha a família para outro serviço ou aparelho. E ferramentas
que prometem segurança podem acabar coletando muito mais informação do que a
necessária.

O teenDNS propõe outro caminho: regras simples, definidas pela família e
aplicadas fora de cada plataforma. A tecnologia ajuda a cumprir um acordo; não
substitui conversa, confiança nem a autonomia de quem usa a rede.

## Como a proposta funciona

Cada pessoa tem um perfil, e os acordos da família acompanham esse perfil entre
seus aparelhos, em vez de ficarem presos a um aplicativo. Para cada categoria, a
família decide o que permitir, observar ou proteger.

A observação serve para apoiar conversas, não para produzir uma lista completa
da navegação. Uma página voltada a jovens explica quais acordos estão ativos e
por quê, sem expor mensagens ou o conteúdo de páginas visitadas.

Também é possível programar horários por grupo e pausas gerais para momentos
como dormir ou fazer refeições.

O teenDNS não é uma ferramenta de espionagem nem promete controle total da vida
digital. A proposta é apoiar limites proporcionais, compreensíveis e combinados
em família. Os detalhes sobre como a tecnologia funciona e seus limites estão na
[visão técnica](docs/visao-tecnica.md) e no documento de
[ameaças e limites](docs/ameacas-e-limites.md).

## Origem: Hackathon Reflorestando a Rede

O teenDNS foi desenvolvido pela equipe **Ônibus Hacker** durante a Hackathon do
Festival Compartilhe — **Reflorestando a Rede**, realizada de 23 a 25 de setembro
de 2026, em São Paulo, por ARTIGO 19 Brasil e América do Sul e pela Electronic
Frontier Foundation (EFF).

A hackathon reuniu pessoas de diferentes áreas para criar ferramentas práticas
em favor de uma internet mais livre, diversa e plural. O teenDNS dialoga com o
eixo de concorrência e reversibilidade de configurações: demonstra como uma
família pode definir e levar suas próprias regras entre plataformas, sem depender
de cada empresa implementar um controle parental diferente.

## Protótipo

O staging público do MVP está em <https://teendns.lab.markun.com.br>. Ele pode
estar em uma versão diferente do trabalho local em andamento.

## Documentação

- [Visão técnica: arquitetura, execução local e validação](docs/visao-tecnica.md)
- [Estado atual e evidências](docs/status.md)
- [Ameaças, privacidade e limites](docs/ameacas-e-limites.md)
- [Operação do staging](docs/staging.md)
- [Metodologia do catálogo](docs/metodologia-catalogo-v1.md)
- [Sistema visual](docs/design-system.md)

## Licença

O teenDNS está disponível sob a [Licença MIT](LICENSE). Os catálogos e outros
materiais de terceiros mantêm as licenças e atribuições de suas fontes.
