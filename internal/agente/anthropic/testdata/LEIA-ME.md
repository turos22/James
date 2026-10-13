# Fixtures do provedor Anthropic

- `texto.sse` e `ferramenta.sse`: **respostas reais** da API (`claude-haiku-5-5`, pensamento desligado), gravadas em 2026-10-13 só com o corpo da resposta. A chave nunca entra aqui.
- `recusa.sse`: **sintética**, escrita à mão no mesmo formato. Uma recusa real não dá para provocar sob demanda.

Para regravar, refaça a chamada com `stream: true` e salve o corpo. Confira com `grep -c sk-ant *.sse` que o resultado é 0.
