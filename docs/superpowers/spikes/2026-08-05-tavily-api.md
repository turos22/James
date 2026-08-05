# Spike — API da Tavily, forma vigente

**Data da consulta:** 2026-08-05
**Motivo:** a spec da Fase 3 (§7) proíbe escrever o payload da Tavily de memória. A Task 10 depende deste documento.
**Fontes:** documentação oficial via context7 (`/websites/tavily`, snapshots de `docs.tavily.com/documentation/api-reference/endpoint/search` e `docs.tavily.com/llms-full.txt`), mais uma chamada real contra `https://api.tavily.com/search` feita nesta data com a `TAVILY_API_KEY` do `.env`.

---

## As cinco perguntas

### 1. Autenticação: header, não corpo

**A forma vigente é o header `Authorization: Bearer <chave>`.**

```bash
curl -X POST https://api.tavily.com/search \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer tvly-SUA_CHAVE" \
  -d '{"query": "..."}'
```

Isto **contraria o que o plano assumiu** e o que o código de hoje faz: [`busca.go`](../../../internal/ferramentas/busca.go) manda `api_key` dentro do corpo JSON. Essa forma é legada — não aparece mais em nenhum exemplo da documentação atual.

Confirmado por chamada real: a requisição com header Bearer e **sem** `api_key` no corpo devolveu 200 com resultados. A Task 10 deve usar o header.

### 2. `search_depth`

Valores aceitos: `advanced`, `basic`, `fast`, `ultra-fast`. Default `basic`.

A Fase 3 usa `advanced` (decisão D7).

Nota: `chunks_per_source` (default 3) só existe quando `search_depth` é `advanced`. Não estamos usando, mas é o parâmetro a mexer se a densidade do trecho de cada fonte ficar ruim.

### 3. `include_answer`

Aceita **booleano**. O exemplo canônico de requisição da documentação lista `"include_answer": false` entre os campos, e a chamada real com `include_answer: true` populou o campo `answer`.

### 4. Campo da resposta sintetizada

`answer`, no **topo** do JSON (não dentro de `results`). Vem `null` quando não se opta por ele.

### 5. Campos de cada item de `results`

Da chamada real: `url`, `title`, `content`, `score`, `raw_content`, `id`.

A documentação do endpoint lista ainda `images`, `rank`, `published_date`, `author` e `domain` — não vieram nesta resposta, então **não devem ser tratados como garantidos**. A Task 10 só depende de `title`, `url` e `content`, que vieram nos dois casos.

---

## Requisição real usada na verificação

```json
{
  "query": "quem ganhou a copa do mundo de 2022",
  "search_depth": "advanced",
  "include_answer": true,
  "max_results": 2
}
```

com `Authorization: Bearer <chave redigida>` e `Content-Type: application/json`.

## Resposta real (estrutura, conteúdo abreviado)

Campos de topo devolvidos:

```
query, follow_up_questions, answer, images, results, response_time, request_id
```

```
answer  = "A Argentina ganhou a Copa do Mundo de 2022, vencendo a França na final
           nos pênaltis. Lionel Messi foi eleito o melhor ..."

results[0].title = "Quem ganhou a Copa 2022? Veja o campeão mundial"
results[0].url   = "https://ndmais.com.br/futebol/quem-ganhou-a-copa-2022"
```

---

## Consequências para a Task 10

1. **Trocar a autenticação** de `api_key` no corpo para header `Authorization: Bearer`. É a única divergência real entre o plano e a API vigente.
2. Os nomes de campo que o plano assumiu (`answer`, `results[].title`, `results[].url`, `results[].content`) estão **corretos** — a struct tipada pode ser escrita como planejado.
3. **Mandar `max_results` na requisição**, além de cortar no lado Go. O corte local continua sendo a garantia (a API pode devolver mais do que se pediu), mas pedir menos economiza banda e tokens de leitura. O default da API é 5, o mesmo default que o `config.toml` da fase adota.
4. `topic` (`general`/`news`/`finance`) e `time_range` existem e seriam úteis para busca de notícia recente. **Fora do escopo** desta fase por decisão D7 — a opção de expor parâmetros ao modelo foi avaliada e descartada.
