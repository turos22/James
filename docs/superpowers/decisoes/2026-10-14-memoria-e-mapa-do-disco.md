# Para a spec 2 (memória + dois modelos): o James saber onde as coisas ficam

Anotado em 2026-10-14, a pedido do Arthur ("deixa isso tudo para a próxima spec"), depois do aceite da autonomia de arquivos.

## O que o aceite mostrou

"me mostra a chave do Claude" falhou duas vezes com "precisei de passos demais" (`max_iteracoes = 5`). Reproduzido com o modelo real e o disco real (aprovação do `revelar` negada, só leitura): o James não sabia onde fica o `.env`, gastou os 5 passos procurando (busca no `~`, listas de pasta, nomes chutados) e só chegou ao `revelar` no último. **Com uma única linha de memória dizendo onde o projeto fica, foi direto ao `revelar` no passo 1, nas duas rodadas.**

## Estado da memória hoje

- Vault de notas em texto, injetado no prompt a cada turno, orçamento de 2000 tokens (`orcamento_notas`).
- Duas notas: `carro.md` e `preferencias.md` (7 linhas).
- Serve para fatos curtos e duráveis; **não** serve para um mapa completo do disco (não cabe e envelhece).

## O que entra na spec 2

1. **Pontos de referência na memória:** onde ficam o James e os projetos de trabalho (o Arthur dita, ou o James aprende). Regra nova no prompt: depois de uma busca que custou vários passos, guardar o caminho achado.
2. **Busca de arquivos apoiada em índice** (`plocate` ou o Baloo do KDE) em vez de varrer o disco a cada pedido (hoje: profundidade 8, 100 resultados, 5 s). Achar um nome vira instantâneo.
3. **Teto de passos:** `max_iteracoes` de 5 para 8, ou por tipo de pedido.
4. Nota imediata: `~/SI/Projetos/Athena(Go)/meu-app-ts` é onde o James fica, e o `.env` está lá.

## Pendente do aceite de arquivos (não é desta spec)

- A recusa da memória foi vista ao vivo só como julgamento do modelo; a trava de código está coberta por teste e mutação, mas falta vê-la bloquear um valor realmente revelado (depende de o caminho ser conhecido).
