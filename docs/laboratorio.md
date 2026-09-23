# Laboratório local

O laboratório comprova o uso de vários perfis DNS-over-TLS no mesmo endereço. Ele é autocontido: um servidor DNS de fixture oferece respostas determinísticas, e o Unbound as resolve e armazena em cache antes do gateway aplicar a política de cada perfil.

## Dependências

- Nix com flakes;
- Docker Engine;
- Docker Compose v2.

O ambiente Nix fornece Go e as ferramentas de desenvolvimento. Os containers não publicam nenhum serviço além do gateway DoT em `127.0.0.1:8853`.

## Executar

```bash
./scripts/lab-up.sh
./scripts/lab-test.sh
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

A faixa `10.77.53.0/24` foi escolhida porque as faixas Docker `172.17.0.0/16` a `172.31.0.0/16` já estavam ocupadas nesta máquina.

## Resultado esperado

O teste verifica:

1. resolução permitida para Bia;
2. bloqueio de `blocked.test` para Ana com `NXDOMAIN`;
3. permissão do mesmo domínio para Bia;
4. precedência da exceção `school.blocked.test` para Ana;
5. TTL de respostas permitidas limitado a 300 segundos.

Os eventos aparecem em JSON nos logs do gateway:

```bash
docker compose logs gateway
```

Cada evento contém perfil, domínio, tipo de consulta, ação e versão da política. Não contém URL, caminho, título ou conteúdo.
