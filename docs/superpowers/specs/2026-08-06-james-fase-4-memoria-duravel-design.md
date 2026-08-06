# James — Fase 4: Memória durável em markdown, sem banco

**Data:** 2026-08-06
**Autor:** Arthur Orosco (design colaborativo)
**Estado:** aprovado, aguardando revisão do documento escrito
**Projeto:** `C:\Users\Arthur\SI\Projetos\Athena(Go)\meu-app-ts`
**Base:** [`2026-08-03-james-fase-0-1-fundacao-e-nucleo-design.md`](2026-08-03-james-fase-0-1-fundacao-e-nucleo-design.md) (spec original, seções "Consequência de D6" e 4.6) e [`HANDOFF-inicio-fase-4.md`](../HANDOFF-inicio-fase-4.md). Este documento é autocontido: não é necessário ler os anteriores para implementar a Fase 4.

---

## 1. Contexto

As Fases 0, 1 e 3 fecharam e estão mergeadas em `main`. A spec original reservou para a Fase 4 o tema "Obsidian como cérebro de contexto", com `pgvector`, schema de embeddings e desenho de RAG, e deliberadamente **não** fixou granularidade de chunk — registrou que fixá-la antes de desenhar o RAG "seria adivinhação".

Quatro medições feitas nesta sessão mudaram o que essa fase deve ser, e a última delas eliminou o banco de dados do projeto. Estão registradas abaixo porque as decisões centrais da fase dependem delas.

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

### 1.4 O Postgres não serve a nada que sobreviva a esta fase

Levantado depois de o Arthur questionar, durante o brainstorm, para que serve o banco se o Obsidian é quem alimenta o James:

| Tabela | Quem escreve | Quem lê em produção |
|---|---|---|
| `mensagens` | todo turno, `persistirNovasMensagens` | só [`rest.go:102`](../../../internal/api/rest.go), para a SPA exibir |
| `sessoes` | conexão nova | só [`rest.go:72`](../../../internal/api/rest.go), para a SPA listar |
| `aprovacoes` | `Criar` e `Decidir`, em [`ws.go:170`](../../../internal/api/ws.go) e 195/201/210 | **ninguém.** `AprovacaoRepositorio.BuscarPorID` só aparece em teste |

E o ponto decisivo: **a aprovação não é coordenada pelo banco.** Quem bloqueia o turno e o libera é `AprovacoesPendentes`, um mapa de canais Go **em memória** ([`ws.go:175`](../../../internal/api/ws.go), e `resolver` em 65-67). O Postgres apenas anota o pedido e a decisão. Remover o banco não toca no fluxo de aprovação.

Dois fatos adicionais que pesam:

- **O `docker-compose.yml` tem só o serviço do Postgres.** Sem ele, o James deixa de precisar de Docker, o `executar.bat` perde as linhas 2 e 3, e o `.env` perde o `POSTGRES_PASSWORD` — um segredo a menos.
- **A cobertura de banco é condicional.** [`testutil_test.go:15`](../../../internal/armazenamento/postgres/testutil_test.go) faz `t.Skip` quando `DATABASE_URL` não está definida. A suíte fecha verde sem nunca ter tocado o banco.

A "Fase 2 — migrar para Postgres" foi absorvida com a justificativa de que o `pgvector` chegaria na Fase 4. A decisão D4 abaixo adia o `pgvector`, e a justificativa do Postgres foi junto.

### 1.5 Item aberto herdado da Fase 3

O fluxo de aprovação de ponta a ponta pelo celular não foi revalidado ao vivo depois das três correções da Fase 3. Não é bloqueante para esta fase, e a remoção do banco não o afeta — a coordenação de aprovação é em memória. Fica registrado para não ser tratado como verificado.

---

## 2. Escopo

### 2.1 Entra

1. Componente de vault em `internal/memoria`: leitura das notas, renderização em bloco dentro de orçamento próprio, escrita de nota por assunto, e escrita dos dois arquivos de registro.
2. Ferramenta `salvar_na_memoria`, quinta do registro.
3. Composição do bloco de notas no system prompt, por turno, em `processarTurno`.
4. **Remoção completa do Postgres** do código, do Docker, da configuração e do launcher.
5. Endpoints de histórico repontados para os arquivos de `Vault/Logs_Diarios/`.
6. Configuração nova, e remoção da configuração de banco.
7. Instrução no `prompts/sistema.md` orientando quando guardar.

### 2.2 Não entra

- **`pgvector`, embeddings, chunking e retrieval semântico.** Adiados, não cancelados. Justificativa em D4.
- **Destilação automática e ciclo de vida de sessão.** Exigiria inventar o evento "fim de conversa", que hoje não existe: o mais próximo é o fechamento do WebSocket, que é justamente o que já produz sessão nova.
- **Reler transcrição ou auditoria para dentro do prompt.** Os dois arquivos de registro existem para o Arthur, nunca para o modelo. É o que mantém o custo de token inalterado — ver seção 4.
- **`Vault/Logs_Diarios/` antigo e `Vault/Contexto_Core.md` como memória.** Justificativa em D9.
- **Substituir o Postgres por SQLite ou qualquer outro banco.** A conclusão de 1.4 é que não há leitor, não que o leitor precisa de outro banco.
- **Localizar o `obsidian.py` do Gui antigo.** O handoff pedia caçá-lo por conter possível lógica de chunking. Sem chunking nesta fase, a pergunta perdeu o objeto.

---

## 3. Decisões desta sessão

| # | Decisão | Escolha | Por quê |
|---|---|---|---|
| D1 | Quem alimenta a memória | O James, sozinho, sem depender de hábito manual do Arthur | Não existe vault real na máquina (1.3). Apostar a fase na adoção de um hábito de anotação que hoje não tem sinal nenhum seria apostar em mudança de comportamento, não em software |
| D2 | Fonte da verdade | Arquivos `.md` em `Vault/`; qualquer índice é derivado e descartável | Memória de assistente pessoal precisa ser inspecionável e corrigível à mão — quando o James lembrar errado, o conserto é abrir o arquivo |
| D3 | Momento da escrita da nota | Ferramenta explícita que o James decide chamar | Reusa o registro de ferramentas endurecido na Fase 3; a SPA já emite `ferramenta_iniciada`/`ferramenta_concluida`, então a escrita é visível ao vivo. Destilação automática exigiria o ciclo de vida de sessão que não existe |
| D4 | Recuperação | Injeção total dentro de orçamento próprio. Sem embedding, sem `pgvector`, sem chunking | O total dos corpora candidatos é ~4.200 tokens, e **zero** depois de D9 excluir o histórico: a memória começa vazia e cresce nota a nota. Nesse regime, retrieval semântico não é avaliável — não há como calibrar `k`, limiar de similaridade ou modelo de embedding sem corpus para medir, e a única verificação possível seria "não quebrou". É o mesmo argumento que a spec original usou para adiar chunking, aplicado um nível acima |
| D5 | Ponto de injeção | System prompt, **não** `memoria.Janela` | Nota não é conversa e não pode competir com recência: seria errado o James esquecer quem é o Arthur porque um log grande entrou na janela. Efeito colateral bom: [`janela.go`](../../../internal/memoria/janela.go) e seus cinco testes ficam intocados |
| D6 | Granularidade da nota | Um arquivo por assunto | É o que torna o `Vault/` navegável como Obsidian de verdade. Corrigir um tema é abrir um arquivo |
| D7 | Momento da leitura | A cada turno, não no boot | A nota que o James acabou de escrever precisa valer no turno seguinte. Ler 11 KB por turno é custo desprezível |
| D8 | Mecanismo de escrita | `O_APPEND` mais mutex, nunca ler-modificar-gravar | WebSocket e device de voz escrevem concorrentemente (`processarTurno` é compartilhado pelos dois). Append não perde escrita concorrente; ler-modificar-gravar perde |
| D9 | Corpus histórico | `Logs_Diarios/` antigo e `Contexto_Core.md` não entram no prompt, nem brutos nem destilados | Evidência em 1.2: não há o que destilar. O `Contexto_Core.md` já perdeu o papel de system prompt na Fase 1 e descreve coordenadas fixas de tela da Athena antiga |
| D10 | Local de execução | Direto em `main`, commit por task | Escolha do Arthur, mesmo padrão da Fase 3 |
| D11 | **Postgres** | **Removido inteiro. Nenhum banco entra no lugar** | Decisão do Arthur, sustentada por 1.4: nenhuma das três tabelas tem leitor que sobreviva a esta fase, e a aprovação — o único fluxo com valor de segurança — é coordenada em memória, não pelo banco. Trocar por SQLite resolveria um problema de tecnologia que não existe; o problema era ausência de leitor |
| D12 | Registro do que acontece | Transcrição em `Vault/Logs_Diarios/`, auditoria de aprovação em `Vault/Auditoria/`. **Escritos, nunca lidos para o prompt** | Escolha do Arthur. Preserva o histórico visível e a trilha de segurança de uma ferramenta que executa PowerShell, a custo de token exatamente zero |
| D13 | Identidade de sessão | UUID gerado em memória, sem persistência | `sessaoID` não existe só para o banco: [`ws.go:135`](../../../internal/api/ws.go) o injeta via `ferramentas.ComSessao` para o `perguntar_ao_claude_code` manter sessão do Claude Code entre chamadas. O identificador continua; a linha em tabela é que morre |
| D14 | Endpoints de histórico | Repontados para os arquivos de `Logs_Diarios/`; o conceito na interface passa de "sessão" para "dia" | Sem banco não há id de sessão persistido para listar. O dia é a chave natural do formato de arquivo escolhido, e a Fase 6 refaz o front-end de qualquer jeito |
| D15 | Formato da transcrição | O mesmo da Athena antiga, com `**James:**` no lugar de `**Athena:**`, mais uma linha com as ferramentas usadas no turno | Deixa arquivo velho e arquivo novo homogêneos no mesmo diretório, e é formato que o Obsidian já renderiza |
| D16 | `Vault/Memoria/` e `Vault/Auditoria/` no git | Entram no `.gitignore`, onde [`.gitignore:38`](../../../.gitignore) já tem `Vault/Logs_Diarios/` | É conteúdo pessoal sobre o Arthur, e o repositório já trata log de conversa assim. Versionar a memória daria histórico de revisão, mas ao custo de colocar fato pessoal em repositório — troca ruim. **Consequência: a memória não tem backup pelo git.** Ver R7 |
| D17 | Ordem da remoção | O vault é construído e provado ao vivo **antes** de qualquer linha de Postgres sair. O volume `james_pgdata` não é apagado | Nunca remover o que funciona antes de o substituto estar provado. O volume custa disco e nada mais; apagá-lo é irreversível e desnecessário |

---

## 4. Arquitetura

O bloco de notas entra pelo system prompt. `memoria.Janela` continua fazendo exatamente o que faz hoje, sobre exatamente o mesmo orçamento. Transcrição e auditoria são becos sem saída de leitura: escritos e nunca lidos de volta para o modelo.

```
Vault/Memoria/*.md  ──leitura por turno──►  memoria.Vault.Renderizar()
                                                      │
                                                      ▼
                          SistemaPrompt + bloco de notas  ──►  loop.Executar(...)
                                                                    ▲
   historico (por conexão) ──► memoria.Janela.Montar() ─────────────┘
                                                                    │
   loop ──► salvar_na_memoria ──► Vault/Memoria/<assunto>.md        │
                                                                    │
   fim do turno ──────────────────────────► Vault/Logs_Diarios/<data>.md   (nunca relido)
   pedido/decisão de aprovação ───────────► Vault/Auditoria/<data>.md      (nunca relido)
```

Dois orçamentos separados e independentes: `orcamento_tokens` (8000) continua sendo só da conversa da conexão atual; `orcamento_notas` (2000) é só do bloco de memória. Nenhum dos dois consome o do outro.

**Sobre consumo de token:** ele não muda em relação a hoje. O banco nunca alimentou o prompt (1.1), e os dois arquivos de registro de D12 também não alimentam. O único conteúdo novo que passa a entrar no prompt é o bloco de notas, limitado por `orcamento_notas`. O jeito de estragar isso seria injetar transcrição — e esta spec proíbe isso explicitamente em 2.2.

### 4.1 `internal/memoria/vault.go`

Componente novo, sem dependência de banco nem de rede.

- `Renderizar() (string, error)` — lê `Vault/Memoria/*.md`, concatena em bloco delimitado e rotulado, respeitando `orcamento_notas` com a mesma `EstimarTokens` que já existe no pacote.
- `Acrescentar(assunto, texto string) error` — resolve o assunto para caminho validado e acrescenta com `O_APPEND`, sob mutex.
- `Assuntos() ([]string, error)` — lista os assuntos existentes, consumido pela declaração da ferramenta.
- `RegistrarTurno(...) error` — acrescenta a interação em `Logs_Diarios/<data>.md`, formato D15.
- `RegistrarAprovacao(...) error` — acrescenta pedido e decisão em `Auditoria/<data>.md`.

Corte por orçamento: remove assuntos inteiros, do menos recentemente modificado para o mais, e marca no bloco que houve corte. Nunca corta um assunto pela metade.

As três escritas compartilham o mesmo mutex e o mesmo caminho de append. Só a leitura de `Memoria/` alimenta o modelo.

### 4.2 `internal/ferramentas/memoria.go`

`salvar_na_memoria(assunto: string, texto: string)`. Nível seguro — escrever no próprio vault não é ação perigosa e não passa pelo gate de confirmação da Fase 3.

A descrição da ferramenta lista os assuntos que já existem, para o James acrescentar em vez de inventar arquivo novo a cada chamada.

### 4.3 Alterações em `internal/api`

- `processarTurno` ([`ws.go:115`](../../../internal/api/ws.go)) compõe `s.SistemaPrompt` com o bloco de notas antes de chamar `loop.Executar`. Único ponto de alteração no fluxo, e serve tanto ao WebSocket quanto à entrada de voz.
- `s.Sessoes.Criar(...)` vira `uuid.New()` em memória (D13).
- `persistirNovasMensagens` passa a chamar `RegistrarTurno`.
- O caminho de aprovação passa a chamar `RegistrarAprovacao` no lugar de `Aprovacoes.Criar`/`Decidir`. A coordenação por canal em memória não muda em nada.
- `Servidor` perde os campos `Sessoes`, `Mensagens` e `Aprovacoes`, e ganha o vault.
- Os dois endpoints de histórico passam a listar e servir arquivos de `Logs_Diarios/` (D14).

### 4.4 Remoção do Postgres

Sai inteiro: `internal/armazenamento/` (pacote, migrações e `migracoes.go`), `cmd/migrar-historico/` (já rodou uma vez na Fase 1 e vira código morto), o `docker-compose.yml`, as linhas 2 e 3 do `executar.bat`, o `POSTGRES_PASSWORD` do `.env` e a configuração de conexão. A dependência `jackc/pgx/v5` sai do `go.mod`.

Quatro arquivos de teste em `internal/api` constroem o `Servidor` com repositórios de Postgres e precisam ser reescritos junto.

Nada disso se perde de verdade: o git guarda tudo, e o volume `james_pgdata` fica no disco (D17).

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

**Auditoria.** `Vault/Auditoria/` é o que sobra do registro de aprovações depois da remoção do banco, e a razão de D12 não ter sido "só as notas". Numa ferramenta que executa PowerShell com privilégio, ficar sem registro do que foi aprovado é regressão de segurança.

---

## 6. Configuração

```toml
[memoria]
orcamento_tokens  = 8000        # já existe, inalterado
orcamento_notas   = 2000        # novo
diretorio_vault   = "Vault"     # novo — Memoria/, Logs_Diarios/ e Auditoria/ pendem daqui
```

Valores padrão em código, seguindo o `intOuDefault` já usado em [`config.go:116`](../../../internal/config/config.go). A configuração de conexão com banco sai.

`.gitignore` passa a cobrir `Vault/Memoria/` e `Vault/Auditoria/`, juntando-se a `Vault/Logs_Diarios/` que já está lá (D16).

---

## 7. Tratamento de erro

| Situação | Comportamento |
|---|---|
| Diretórios do vault não existem | Criados no boot |
| Leitura das notas falha | Registra log, o turno continua **sem** notas. Memória é melhor-esforço na leitura, igual ao `gerarAudioDaResposta` que já existe |
| Escrita de nota falha | Volta como erro da ferramenta, para o James avisar o Arthur que não conseguiu guardar |
| Escrita de transcrição ou auditoria falha | Registra log e o turno continua. Registro não pode derrubar conversa |
| Bloco estoura o orçamento | Corta assuntos inteiros, do menos recentemente modificado para o mais, marca o corte no bloco e registra log |
| Assunto inválido ou fora do diretório | Erro da ferramenta, sem gravação |

---

## 8. Estratégia de teste

**Unidade, com TDD:**

- Slug e validação de caminho, incluindo travessia (`../`, caminho absoluto, separador do Windows).
- `Acrescentar` concorrente: N escritas simultâneas, todas presentes no fim.
- Corte por orçamento: assunto inteiro removido, marcador presente, nunca corta pela metade.
- Formato do bloco renderizado, e formato da transcrição (D15).
- Vault ausente e vault ilegível não devolvem erro fatal na leitura.

**Integração:**

- Ferramenta registrada e presente em `Declaracoes()`.
- `processarTurno` entrega ao loop um system prompt que contém a nota.
- Fluxo de aprovação continua funcionando **sem banco nenhum** — o teste existente que cobre canal em memória tem que passar depois da remoção.

**Ganho colateral:** com o Postgres fora, o `t.Skip` condicional de [`testutil_test.go:15`](../../../internal/armazenamento/postgres/testutil_test.go) some junto. A suíte passa a rodar inteira sem Docker, sem `DATABASE_URL` e sem estado externo.

**Ao vivo, contra o `jamesd` real:** os critérios CA3 e CA7 da seção 9.

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
| CA7 | **Com o `docker-compose` derrubado e nenhum Postgres no ar, o `jamesd` sobe, conversa, aprova uma ação perigosa e a executa.** Verificação ao vivo — é o que prova que a remoção não quebrou o fluxo de aprovação |
| CA8 | Transcrição do turno aparece em `Vault/Logs_Diarios/<data>.md` e a aprovação em `Vault/Auditoria/<data>.md`, e **nenhum dos dois** aparece no prompt enviado ao provedor |
| CA9 | Suíte completa verde sem `DATABASE_URL` definida e sem Docker rodando |

CA3 é o critério que prova a metade da memória, e ele **falha hoje por construção**. CA7 é o que prova a metade da remoção.

---

## 10. Ordem de implementação

A ordem existe para nunca remover o que funciona antes de o substituto estar provado (D17).

1. Slug e validação de caminho (segurança primeiro, TDD).
2. `memoria.Vault` — leitura das notas, renderização e orçamento (TDD).
3. `memoria.Vault` — escrita com append e concorrência (TDD).
4. Ferramenta `salvar_na_memoria` e registro (TDD).
5. Configuração nova.
6. Composição no `processarTurno`.
7. Instrução no `prompts/sistema.md`.
8. **Verificação ao vivo do CA3.** Nada de remoção antes deste ponto.
9. `RegistrarTurno` e `RegistrarAprovacao` escrevendo em paralelo com o Postgres, os dois ao mesmo tempo.
10. Endpoints de histórico repontados para os arquivos.
11. Remoção do Postgres: código, testes, `docker-compose.yml`, `executar.bat`, `.env`, `go.mod`.
12. **Verificação ao vivo do CA7**, e suíte completa sem Docker (CA9).

---

## 11. Riscos conhecidos

| # | Risco | Mitigação |
|---|---|---|
| R1 | O modelo não chamar a ferramenta, e a memória nunca crescer | Instrução no `prompts/sistema.md`, e CA3 mede isso ao vivo. Se falhar, a resposta é ajustar o prompt e medir de novo — não trocar a arquitetura |
| R2 | O modelo criar assunto novo quando devia acrescentar num existente | A declaração da ferramenta lista os assuntos que já existem |
| R3 | Ruído: o James guardar trivialidade e poluir o bloco | O Arthur abre o arquivo e apaga. É o motivo de D2 |
| R4 | Vault crescer e estourar o orçamento sem ninguém perceber | Marcador de corte no bloco mais log. O estouro é o sinal de que o retrieval de D4 passou a se pagar — e o índice pode ser construído a partir do markdown, em qualquer tecnologia, justamente porque D2 o torna descartável |
| R5 | Nota contendo saída de `pesquisar_internet` promove conteúdo não confiável para o system prompt | Rotulagem do bloco e instrução de guardar em palavras próprias. Contenção parcial e assumida — ver seção 5 |
| R6 | Obsidian com o arquivo aberto atrapalhar a escrita no Windows | `O_APPEND` convive melhor com isso que renomear por cima. Não verificável agora: o Obsidian não está instalado na máquina (1.3) |
| R7 | **A memória fica sem backup.** D16 tira `Vault/Memoria/` do git, e D11 tira o banco — o cérebro do James passa a existir num único diretório, num único disco | Reconhecido e não resolvido nesta fase. É a troca de D16. Se o Arthur quiser backup, o caminho barato é sincronizar o `Vault/` para nuvem por fora do repositório, ou reverter D16 e versionar a memória aceitando o conteúdo pessoal no git |
| R8 | Remoção do banco quebrar algo não mapeado | Ordem de implementação (seção 10) constrói e prova antes de remover; CA7 e CA9 verificam ao vivo depois. O git guarda o código removido, e `james_pgdata` fica no disco |
