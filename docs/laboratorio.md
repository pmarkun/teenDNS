# Laboratório local

O laboratório comprova o uso de vários perfis DNS-over-TLS no mesmo endereço. Ele é autocontido: um servidor DNS de fixture oferece respostas determinísticas, e o Unbound as resolve e armazena em cache antes do gateway aplicar a política de cada perfil.

## Dependências

- Nix com flakes;
- Docker Engine;
- Docker Compose v2.

O ambiente Nix fornece Go, Node.js e as ferramentas de desenvolvimento. Todos
os serviços são publicados apenas na interface local.

## Executar

```bash
./scripts/lab-up.sh
./scripts/lab-test.sh
./scripts/lab-load-test.sh
```

Encerrar:

```bash
./scripts/lab-down.sh
```

O primeiro comando gera uma CA e um certificado efêmeros em `.local/certs`, ignorados pelo Git. A chave do laboratório é legível pelo usuário sem privilégios do container; ela nunca deve ser usada em staging ou produção.

## Perfis de demonstração

| Perfil | Endpoint | `blocked.test` | `school.blocked.test` |
| --- | --- | --- | --- |
| Ana | `p-ana.dns.teendns.test` | bloqueado | permitido por exceção específica |
| Bia | `p-bia.dns.teendns.test` | permitido | permitido |

Os dois endpoints chegam a `127.0.0.1:8853`. O cliente envia o endpoint como SNI da conexão TLS, e o gateway seleciona a política correspondente.

## Serviços

- `fixture`: DNS UDP determinístico em `10.77.53.10:5353`;
- `unbound`: resolvedor e cache em `10.77.53.20:5353`;
- `gateway`: DNS-over-TLS em `10.77.53.30:853`, publicado localmente como `127.0.0.1:8853`.
- `gateway` API: HTTP em `127.0.0.1:18081`;
- `web`: página pública e painel em `http://127.0.0.1:18082`.

O painel usa a chave `teendns-lab`, que existe somente para o laboratório. A API
aceita `Authorization: Bearer teendns-lab`. Não reutilize essa chave fora do
ambiente local.

A faixa `10.77.53.0/24` foi escolhida porque as faixas Docker `172.17.0.0/16` a `172.31.0.0/16` já estavam ocupadas nesta máquina.

## Resultado esperado

O teste verifica:

1. resolução permitida para Bia;
2. bloqueio de `blocked.test` para Ana com `NXDOMAIN`;
3. permissão do mesmo domínio para Bia;
4. precedência da exceção `school.blocked.test` para Ana;
5. TTL de respostas permitidas limitado a 300 segundos.
6. duas respostas de perfis distintos atendidas por uma única consulta upstream em cache;
7. recarga atômica de política por `SIGHUP`, sem reiniciar o processo;
8. 50 perfis sintéticos, burst com 100 trabalhadores e carga sustentada de 50 consultas/s.

Por padrão, a fase sustentada do ensaio de carga dura dez segundos. Para executar a meta completa do plano:

```bash
LOAD_DURATION=10m ./scripts/lab-load-test.sh
```

O ensaio imprime throughput e latência p95. A carga usa conexões TLS reais, o mesmo IP do gateway e um hostname SNI diferente para cada perfil sintético.

Em 23 de setembro de 2026, o ensaio completo executou 30.000 consultas para 50 perfis durante dez minutos, sustentando 50 consultas/s, sem falhas e com p95 de 1,982 ms no laboratório local. Consulte [status.md](status.md) para o contexto e os limites dessa medição.

Os eventos aparecem em JSON nos logs do gateway:

```bash
docker compose logs gateway
```

Cada evento contém perfil, domínio, tipo de consulta, ação e versão da política. Não contém URL, caminho, título ou conteúdo.

## Alteração de políticas

O caminho normal é o painel. A API valida a configuração completa, grava uma
nova versão do arquivo por troca atômica e publica o novo snapshot em memória.
Consultas novas já usam a regra sem reiniciar o gateway.

O sinal `SIGHUP` continua disponível para testes e operação manual.

O gateway mantém um snapshot imutável das políticas em memória. Ao receber `SIGHUP`, valida a configuração completa e troca o snapshot de forma atômica. Se a nova configuração for inválida, mantém a última versão válida.

No laboratório, `lab-up.sh` copia a configuração base para `.local/gateway.json`. Os testes alteram somente essa cópia ignorada pelo Git.
