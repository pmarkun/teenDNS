# Ameaças e limites do MVP

Este documento registra limites que não devem ser escondidos no produto ou no piloto.

## O hostname identifica o perfil, mas não autentica fortemente o aparelho

No Android sem aplicativo, o único identificador disponível ao teenDNS é o hostname configurado como provedor de DNS privado. O gateway o recebe como SNI da conexão TLS.

O identificador deve ser longo, aleatório, revogável e não conter nome ou outro dado pessoal. Ainda assim, o SNI do DNS-over-TLS pode ser observado pela rede antes de o canal TLS ser estabelecido. Quem copiar o hostname pode consultar usando a política daquele perfil e poluir suas estatísticas.

Portanto, no MVP:

- o hostname é uma credencial de baixa garantia;
- não autoriza acesso ao painel, pedidos ou dados pessoais;
- pode ser regenerado sem recriar o perfil;
- eventos devem ser agregados e ter retenção curta;
- o painel usa autenticação separada;
- não se afirma que toda consulta atribuída ao perfil veio do aparelho esperado.

Autenticação forte por certificado de cliente ou chave do dispositivo exigiria aplicativo, perfil gerenciado ou VPN e está fora do caminho “sem instalar nada”.

## O filtro pode ser contornado

O teenDNS não é um mecanismo antifraude ou de vigilância resistente ao usuário. Ele pode ser contornado por:

- VPN;
- navegador ou aplicativo com resolvedor próprio;
- acesso direto por endereço IP;
- mudança da configuração de DNS;
- outro aparelho ou rede;
- proxy e alguns mecanismos de tunelamento.

O objetivo do produto é proteção proporcional e mediação familiar, não controle coercitivo impossível de desativar.

## DNS não descreve uma navegação completa

Uma consulta revela um domínio solicitado, não:

- URL, caminho ou busca;
- conteúdo visto;
- identidade dentro do serviço;
- duração ou atenção;
- intenção da pessoa;
- qual aplicativo originou a consulta, de forma confiável;
- cada acesso, por causa dos caches.

As interfaces devem falar em “domínios consultados” ou “atividade observada”, nunca em histórico completo ou tempo de uso.

## Cache atrasa alterações

Celular, navegador e aplicativos podem reutilizar respostas até o TTL expirar. Reduzir o TTL entregue pelo gateway limita consultas futuras a 300 segundos, mas não encerra conexões existentes nem apaga caches que ignorem o TTL.

Bloqueios locais usam resposta negativa de 30 segundos. O painel deve comunicar que mudanças podem levar alguns minutos.

## Classificações podem estar erradas

Um domínio pode atender vários serviços, mudar de proprietário, hospedar conteúdo de terceiros ou ser necessário para uma função inesperada. Listas externas são insumos, não verdade automática.

O produto precisa oferecer:

- fonte e data de revisão;
- exceção mais específica;
- contestação;
- expiração de exceção;
- reversão rápida;
- tratamento explícito de CNAME, já coberto no laboratório;
- revisão futura de outros encadeamentos e tipos DNS.

## Logs DNS são sensíveis

Mesmo sem URLs, a sequência de domínios pode revelar saúde, religião, política, sexualidade, rotina e relações pessoais. O MVP não deve registrar respostas completas nem IPs de destino.

Antes de um piloto com famílias, ainda será necessário definir:

- retenção máxima dos eventos individuais;
- agregação e exclusão automática;
- criptografia e backups;
- controle de acesso e auditoria;
- política para incidentes;
- transparência compreensível para crianças e responsáveis;
- base jurídica e responsabilidades relativas a dados de crianças e adolescentes.

## Disponibilidade

Quando configurado no aparelho inteiro, o teenDNS vira infraestrutura crítica: se falhar, a internet parece estar fora do ar.

O comportamento inicial é:

- manter a última política válida se a fonte de configuração falhar;
- rejeitar endpoint desconhecido ou revogado;
- retornar `SERVFAIL` quando não houver resolução confiável;
- nunca liberar silenciosamente apenas porque um componente falhou.

Staging e produção precisarão de monitoramento externo específico para TLS na porta 853 e consultas DNS válidas, não somente um `/healthz` HTTP.
