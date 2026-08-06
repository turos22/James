# James — Fase 4: Memória durável

**Data:** 2026-08-06
**Autor:** Arthur Orosco (design colaborativo)
**Estado:** aprovado, aguardando revisão do documento escrito
**Projeto:** `C:\Users\Arthur\SI\Projetos\Athena(Go)\meu-app-ts`
**Base:** [`2026-08-03-james-fase-0-1-fundacao-e-nucleo-design.md`](2026-08-03-james-fase-0-1-fundacao-e-nucleo-design.md) (spec original, seções "Consequência de D6" e 4.6) e [`HANDOFF-inicio-fase-4.md`](../HANDOFF-inicio-fase-4.md). Este documento é autocontido: não é necessário ler os anteriores para implementar a Fase 4.

---

## 1. Contexto

As Fases 0, 1 e 3 fecharam e estão mergeadas em `main`. A spec original reservou para a Fase 4 o tema "Obsidian como cérebro de contexto", com `pgvector`, schema de embeddings e desenho de RAG, e deliberadamente **não** fixou granularidade de chunk — registrou que fixá-la antes de desenhar o RAG "seria adivinhação".

Três medições feitas nesta sessão mudam o que essa fase deve ser. Estão registradas abaixo porque a decisão central da fase (seção 3, D4) depende delas.

### 1.1 O James não lê nada de volta — nem o passado, nem a sessão anterior

Verificado por leitura de código, não deduzido:

- [`ws.go:37`](../../../internal/api/ws.go) e [`dispositivo.go:53`](../../../internal/api/dispositivo.go) declaram `var historico []agente.Mensagem` **por conexão**, começando vazio.
- [`ws.go:116`](../../../internal/api/ws.go): `if *sessaoID == uuid.Nil` cria sessão nova. Conexão nova, sessão nova.
- `MensagemRepositorio.ListarPorSessao` — o único caminho de leitura de mensagem no banco — é chamado em [`rest.go:102`](../../../internal/api/rest.go), para a SPA **exibir** histórico, e em dois testes. Nunca em `processarTurno`.

A tabela `mensagens` é escrita e nunca relida para dentro do prompt. Fechar a aba apaga a conversa inteira do ponto de vista do agente. O banco confirma o efeito: **543 sessões para 557 mensagens**, com 205 sessões de uma ou duas mensagens.

Consequência para o desenho: a fase tem duas metades obrigatórias, e a segunda não existe hoje em forma nenhuma — escrever a nota, e fazer a nota voltar ao prompt. A segunda é a espinha, e ela é necessária mesmo que o mecanismo de seleção nunca seja semântico.

### 1.2 Não existe corpus para recuperar

| Fonte | Volume medido em 2026-08-06 |
|---|---|
| `Vault/Logs_Diarios/` | 5 arquivos, **11.168 bytes**, 28 interações, mais recente 2026-08-03 |
| Postgres `mensagens` | 557 linhas, **5.677 bytes de texto**, 04 a 06/08 |

Detalhe do Postgres: 365 mensagens de usuário com média de 7 caracteres e máximo de 142; 171 do modelo com média de 18; 21 de ferramenta com `conteudo` vazio e 77 kB em `chamadas`. As mensagens de usuário mais longas são `"pergunta ao claude code: quantos arquivos .go existem no diretorio atual"` e `"Clique na posição 500,500 da tela agora"` — é tráfego de teste das Fases 1 e 3, não conversa.

Os logs do Vault são pergunta volátil sobre modelos da API do Gemini, `"abra o spotify tocando a música highway to hell"`, e várias linhas truncadas por reconhecimento de voz. Nada durável sobre o Arthur.

Soma dos dois corpora: ~17 KB, cerca de 4.200 tokens pela régua do próprio projeto (`EstimarTokens`, 4 caracteres por token, em [`janela.go:14`](../../../internal/memoria/janela.go)). O `orcamento_tokens` atual é 8000 ([`config.toml:15`](../../../config.toml)).

### 1.3 Não existe vault do Obsidian na máquina

Busca por diretório `.obsidian` em `C:\Users\Arthur` (profundidade 3) e em `Documents`, `Desktop`, `OneDrive` e `SI` (profundidade 4): nenhum resultado. Nenhuma pasta `obsidian` em `AppData\Roaming`.

**Limite honesto desta verificação:** os drives `G:` e `H:` não foram varridos, e a busca teve limite de profundidade. Se um vault real aparecer depois, ele muda o insumo da fase, não a arquitetura decidida aqui.

O `Vault/` do repositório é uma pasta que a Athena antiga escrevia, nomeada pelo conceito do Obsidian — nunca foi um vault do Obsidian de verdade.

### 1.4 Item aberto herdado da Fase 3

O fluxo de aprovação de ponta a ponta pelo celular não foi revalidado ao vivo depois das três correções da Fase 3. Não é bloqueante para esta fase e não é tocado por ela. Fica registrado para não ser tratado como verificado.

---

## 2. Escopo

### 2.1 Entra

1. Componente de vault em `internal/memoria`: leitura dos arquivos de memória, renderização em bloco dentro de um orçamento próprio, e escrita por assunto.
2. Ferramenta `salvar_na_memoria`, quinta do registro.
3. Composição do bloco de notas no system prompt, por turno, em `processarTurno`.
4. Configuração nova: orçamento de notas e diretório de memória.
5. Instrução no `prompts/sistema.md` orientando quando guardar.

### 2.2 Não entra

- **`pgvector`, embeddings, chunking e retrieval semântico.** Adiados, não cancelados. Justificativa em D4.
- **Destilação automática e ciclo de vida de sessão.** Exigiria inventar o evento "fim de conversa", que hoje não existe: o mais próximo é o fechamento do WebSocket, que é justamente o que já produz sessão nova.
- **Carregar `mensagens` do Postgres para dentro do `historico`.** Resolveria "continue a conversa de ontem", problema diferente de "lembre de mim", mexe na SPA e conversa com a Fase 6. Lacuna conhecida e registrada, fora desta fase.
- **`Vault/Logs_Diarios/` e `Vault/Contexto_Core.md`.** Justificativa em D9.
- **Localizar o `obsidian.py` do Gui antigo.** O handoff pedia caçá-lo por conter possível lógica de chunking. Sem chunking nesta fase, a pergunta perdeu o objeto.

---

## 3. Decisões desta sessão

| # | Decisão | Escolha | Por quê |
|---|---|---|---|
| D1 | Quem alimenta a memória | O James, sozinho, sem depender de hábito manual do Arthur | Não existe vault real na máquina (1.3). Apostar a fase na adoção de um hábito de anotação que hoje não tem sinal nenhum seria apostar em mudança de comportamento, não em software |
| D2 | Fonte da verdade | Arquivos `.md` em `Vault/Memoria/`; qualquer índice é derivado e descartável | Memória de assistente pessoal precisa ser inspecionável e corrigível à mão — quando o James lembrar errado, o conserto é abrir o arquivo. E sobrevive a um `docker volume rm` |
| D3 | Momento da escrita | Ferramenta explícita que o James decide chamar | Reusa o registro de ferramentas endurecido na Fase 3; a SPA já emite `ferramenta_iniciada`/`ferramenta_concluida`, então a escrita é visível ao vivo. Destilação automática exigiria o ciclo de vida de sessão que não existe |
| D4 | Recuperação | Injeção total dentro de orçamento próprio. Sem embedding, sem `pgvector`, sem chunking | O total dos corpora candidatos é ~4.200 tokens, e **zero** depois de D9 excluir o histórico: a memória começa vazia e cresce nota a nota. Nesse regime, retrieval semântico não é avaliável — não há como calibrar `k`, limiar de similaridade ou modelo de embedding sem corpus para medir, e a única verificação possível seria "não quebrou". É o mesmo argumento que a spec original usou para adiar chunking, aplicado um nível acima. Quando o vault estourar o orçamento, existirá corpus real para calibrar — e aí o `pgvector` se paga |
| D5 | Ponto de injeção | System prompt, **não** `memoria.Janela` | Nota não é conversa e não pode competir com recência: seria errado o James esquecer quem é o Arthur porque um log grande entrou na janela. Efeito colateral bom: [`janela.go`](../../../internal/memoria/janela.go) e seus cinco testes ficam intocados, zero risco de regressão no que a Fase 1 consertou |
| D6 | Granularidade de arquivo | Um arquivo por assunto | É o que torna o `Vault/` navegável como Obsidian de verdade. Corrigir um tema é abrir um arquivo. Um arquivo por nota viraria centenas de arquivos minúsculos; um arquivo só cresceria sem estrutura |
| D7 | Momento da leitura | A cada turno, não no boot | A nota que o James acabou de escrever precisa valer no turno seguinte. Ler 11 KB por turno é custo desprezível |
| D8 | Mecanismo de escrita | `O_APPEND` mais mutex, nunca ler-modificar-gravar | WebSocket e device de voz escrevem concorrentemente (`processarTurno` é compartilhado pelos dois). Append não perde escrita concorrente; ler-modificar-gravar perde |
| D9 | Corpus histórico | `Logs_Diarios/` e `Contexto_Core.md` não entram, nem brutos nem destilados | Evidência em 1.2: não há o que destilar. O `Contexto_Core.md` já perdeu o papel de system prompt na Fase 1 e descreve coordenadas fixas de tela da Athena antiga |
| D10 | Local de execução | Direto em `main`, commit por task | Escolha do Arthur, mesmo padrão da Fase 3 |

---

## 4. Arquitetura

O bloco de notas entra pelo system prompt. `memoria.Janela` continua fazendo exatamente o que faz hoje, sobre exatamente o mesmo orçamento.

```
Vault/Memoria/*.md  ──leitura por turno──►  memoria.Vault.Renderizar()
                                                      │
                                                      ▼
                          SistemaPrompt + bloco de notas  ──►  loop.Executar(...)
                                                                    ▲
   historico (por conexão) ──► memoria.Janela.Montar() ─────────────┘

   loop ──► ferramenta salvar_na_memoria ──► memoria.Vault.Acrescentar() ──► Vault/Memoria/<assunto>.md
```

Dois orçamentos separados e independentes: `orcamento_tokens` (8000) continua sendo só da conversa; `orcamento_notas` (2000) é só do bloco de memória. Nenhum dos dois consome o do outro.

### 4.1 `internal/memoria/vault.go`

Componente novo, sem dependência de banco nem de rede.

- `Renderizar() (string, error)` — lê `Vault/Memoria/*.md`, concatena em um bloco delimitado e rotulado, respeitando `orcamento_notas` com a mesma `EstimarTokens` que já existe no pacote.
- `Acrescentar(assunto, texto string) error` — resolve o assunto para um caminho validado e acrescenta com `O_APPEND`, sob mutex.
- `Assuntos() ([]string, error)` — lista os assuntos existentes, consumido pela declaração da ferramenta.

Corte por orçamento: remove assuntos inteiros, do menos recentemente modificado para o mais, e marca no bloco que houve corte. Nunca corta um assunto pela metade.

### 4.2 `internal/ferramentas/memoria.go`

`salvar_na_memoria(assunto: string, texto: string)`. Nível seguro — escrever no próprio vault não é ação perigosa e não passa pelo gate de confirmação da Fase 3.

A descrição da ferramenta lista os assuntos que já existem, para o James acrescentar em vez de inventar arquivo novo a cada chamada.

### 4.3 Alteração em `processarTurno`

[`ws.go:115`](../../../internal/api/ws.go) passa a compor `s.SistemaPrompt` com o bloco de notas antes de chamar `loop.Executar`. É o único ponto de alteração no fluxo, e serve tanto ao WebSocket quanto à entrada de voz do device, que já compartilham essa função.

---

## 5. Segurança

O `assunto` chega do modelo. `salvar_na_memoria(assunto: "../../prompts/sistema.md")` não pode escapar do diretório de memória.

Duas barreiras, ambas com teste:

1. O assunto é reduzido a slug `[a-z0-9-]+` antes de virar nome de arquivo.
2. O caminho final resolvido é validado como filho do diretório de memória; qualquer coisa fora é erro, não gravação.

**Promoção de conteúdo não confiável.** A Fase 3 marcou a saída de `pesquisar_internet` como conteúdo externo não confiável, justamente porque uma página web pode conter texto endereçado ao modelo. Se o James guardar essa saída em nota, o texto passa a entrar no **system prompt** de todo turno seguinte — a posição de maior confiança do prompt. Contenções:

- O bloco de notas é delimitado e rotulado como memória escrita pelo James, nunca como instrução do sistema.
- A instrução no `prompts/sistema.md` orienta guardar conclusão em palavras próprias, não copiar saída bruta de ferramenta.

Nenhuma das duas é barreira dura. É a lacuna consciente desta fase, e está registrada como R5.

---

## 6. Configuração nova

```toml
[memoria]
orcamento_tokens  = 8000        # já existe, inalterado
orcamento_notas   = 2000        # novo
diretorio_memoria = "Vault/Memoria"  # novo
```

Ambos com valor padrão em código, seguindo o padrão de `intOuDefault` já usado em [`config.go:116`](../../../internal/config/config.go).

---

## 7. Tratamento de erro

| Situação | Comportamento |
|---|---|
| Diretório de memória não existe | Criado no boot |
| Leitura do vault falha | Registra log, o turno continua **sem** notas. Memória é melhor-esforço na leitura, igual ao `gerarAudioDaResposta` que já existe |
| Escrita falha | Volta como erro da ferramenta, para o James avisar o Arthur que não conseguiu guardar |
| Bloco estoura o orçamento | Corta assuntos inteiros, do menos recentemente modificado para o mais, marca o corte no bloco e registra log |
| Assunto inválido ou fora do diretório | Erro da ferramenta, sem gravação |

---

## 8. Estratégia de teste

**Unidade, com TDD:**

- Slug e validação de caminho, incluindo os casos de travessia (`../`, caminho absoluto, separador do Windows).
- `Acrescentar` concorrente: N escritas simultâneas, todas presentes no fim.
- Corte por orçamento: assunto inteiro removido, marcador presente, nunca corta pela metade.
- Formato do bloco renderizado.
- Vault ausente e vault ilegível não devolvem erro fatal na leitura.

**Integração:**

- Ferramenta registrada e presente em `Declaracoes()`.
- `processarTurno` entrega ao loop um system prompt que contém a nota.

**Ao vivo, contra o `jamesd` real:** o critério de aceitação da seção 9, item CA3.

---

## 9. Critérios de aceitação

| # | Critério |
|---|---|
| CA1 | Chamada de `salvar_na_memoria` cria ou acrescenta o arquivo certo em `Vault/Memoria/`, com o assunto virando slug |
| CA2 | Travessia de caminho é rejeitada, com teste que falha antes da correção |
| CA3 | **Contar um fato ao James numa conexão, fechar a conexão, abrir outra — o que hoje garante sessão nova e amnésia total — e o James saber o fato.** Verificação ao vivo, com o `jamesd` real |
| CA4 | Vault ausente ou ilegível não derruba o turno |
| CA5 | Vault maior que `orcamento_notas` é cortado por assunto inteiro, com marcador |
| CA6 | [`janela.go`](../../../internal/memoria/janela.go) inalterado e seus cinco testes passando |

CA3 é o critério que prova a fase, e ele **falha hoje por construção** — é a espinha da seção 1.1 que não existe.

---

## 10. Ordem de implementação

1. Slug e validação de caminho (segurança primeiro, TDD).
2. `memoria.Vault` — leitura, renderização e orçamento (TDD).
3. `memoria.Vault` — escrita com append e concorrência (TDD).
4. Ferramenta `salvar_na_memoria` e registro (TDD).
5. Configuração nova.
6. Composição no `processarTurno`.
7. Instrução no `prompts/sistema.md`.
8. Verificação ao vivo do CA3.

---

## 11. Riscos conhecidos

| # | Risco | Mitigação |
|---|---|---|
| R1 | O modelo não chamar a ferramenta, e a memória nunca crescer | Instrução no `prompts/sistema.md`, e CA3 mede isso ao vivo. Se falhar, a resposta é ajustar o prompt e medir de novo — não trocar a arquitetura |
| R2 | O modelo criar assunto novo quando devia acrescentar num existente | A declaração da ferramenta lista os assuntos que já existem |
| R3 | Ruído: o James guardar trivialidade e poluir o bloco | O Arthur abre o arquivo e apaga. É o motivo de D2 |
| R4 | Vault crescer e estourar o orçamento sem ninguém perceber | Marcador de corte no bloco mais log. O estouro é o sinal de que o `pgvector` de D4 passou a se pagar |
| R5 | Nota contendo saída de `pesquisar_internet` promove conteúdo não confiável para o system prompt | Rotulagem do bloco e instrução de guardar em palavras próprias. Contenção parcial e assumida — ver seção 5 |
| R6 | Obsidian com o arquivo aberto atrapalhar a escrita no Windows | `O_APPEND` convive melhor com isso que renomear por cima. Não verificável agora: o Obsidian não está instalado na máquina (1.3) |
