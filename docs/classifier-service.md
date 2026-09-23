# Serviço de classificação de URLs

O primeiro classificador do teenDNS recebe uma URL, extrai um perfil textual
limitado da página e avalia os 52 critérios de classificação 14+ usando um
endpoint compatível com Simple Jev.

Ele oferece três superfícies sobre a mesma biblioteca Go:

- pacote importável `github.com/pmarkun/teendns/classifier`;
- CLI `teendns-classify classify`;
- serviço HTTP `teendns-classify serve`.

## Limites deste primeiro corte

- Classifica a **página**, não todo o domínio.
- Usa título, descrição, texto visível e domínios dos links encontrados.
- Não interpreta imagens, áudio, vídeo ou comportamento JavaScript.
- Só procura critérios 14, 16 e 18. Quando nenhum supera o limiar, retorna
  `unknown`, nunca presume `L` ou `12`.
- O resultado é `machine_suggestion` e precisa ser calibrado com exemplos
  revisados antes de orientar bloqueios reais.
- O Simple Jev fornece a pontuação, mas não aponta o trecho exato que motivou a
  decisão. A evidência original deve ser revisada.

## Rodar com Simple Jev local

Inicie um servidor Simple Jev em `127.0.0.1:8000` seguindo a documentação do
projeto e confirme o identificador exato do modelo em `/v1/models`.

Depois classifique uma página:

```sh
nix develop --command go run ./cmd/teendns-classify classify \
  --jev-url http://127.0.0.1:8000/v1/classifier \
  --jev-model Qwen/Qwen3.5-0.8B \
  https://example.com
```

Também é possível configurar por ambiente:

```sh
export SIMPLE_JEV_URL=http://127.0.0.1:8000/v1/classifier
export SIMPLE_JEV_MODEL=Qwen/Qwen3.5-0.8B
nix develop --command go run ./cmd/teendns-classify classify https://example.com
```

## Rodar o serviço HTTP

```sh
nix develop --command go run ./cmd/teendns-classify serve \
  --listen 127.0.0.1:8090
```

Verificação de saúde:

```sh
curl http://127.0.0.1:8090/healthz
```

Classificação:

```sh
curl -X POST http://127.0.0.1:8090/v1/classify \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com"}'
```

Resposta resumida esperada para uma página neutra:

```json
{
  "rating": "unknown",
  "criteria": [],
  "decision": "observe",
  "review_status": "machine_suggestion"
}
```

## Usar como biblioteca

```go
taxonomy, _ := os.Open("criteria/age-rating-v1.json")
guidance, _ := os.Open("criteria/priority-guidance-12-13-v1.json")
catalog, err := classifier.LoadCatalog(taxonomy, guidance)
if err != nil {
    return err
}

service := &classifier.Classifier{
    Catalog:   catalog,
    Extractor: classifier.NewExtractor(false),
    Engine: &classifier.SimpleJEV{
        Endpoint: "http://127.0.0.1:8000/v1/classifier",
        Model:    "Qwen/Qwen3.5-0.8B",
    },
    Threshold: 0.75,
}

result, err := service.ClassifyURL(ctx, "https://example.com")
```

## Segurança de busca da URL

Por padrão, o extrator:

- aceita somente HTTP e HTTPS;
- rejeita credenciais embutidas na URL;
- bloqueia endereços privados, loopback, link-local, multicast e não
  especificados;
- repete a verificação no momento da conexão;
- desabilita proxies de ambiente para evitar contorno da proteção;
- limita redirecionamentos, tempo de resposta, corpo HTML e texto extraído.

Para testes locais existe `--allow-private`. Essa opção não deve ser habilitada
em um serviço exposto ou compartilhado.

## Teste público já realizado

Em 23 de setembro de 2026, o fluxo completo foi validado contra o servidor de
demonstração do Simple Jev com o modelo
`featherless-ai/Qwen3.5-4B-classifier`. Tanto o CLI quanto o endpoint HTTP
classificaram `https://example.com` como `unknown/observe`, sem critérios 14+
acima do limiar de `0.75`.

O servidor público é apropriado somente para testes com conteúdo não sensível.
Não enviar histórico de navegação, mensagens pessoais, dados familiares ou
material ilegal/suspeito.

## Próxima validação

Montar um conjunto pequeno de páginas ou fixtures revisadas contendo casos
positivos e negativos para cada família de critério. Só então calibrar o
limiar, medir falsos positivos e falsos negativos e decidir quais resultados
podem ser automatizados.
