# Autonomia nos arquivos: plano de implementação

> **Para quem executa:** um task por vez, na ordem. TDD em todos: escrever o teste, rodar e **ver falhar pelo motivo certo**, implementar, rodar e ver passar, commitar. A saída de cada "rodar" vai no relato. Não rodou, diz que não rodou. Modo trabalho: `/code-review` e `/security-review` antes do merge (Task 15).

**Objetivo:** o James olha o disco sem pedir (`arquivos`), mexe nele só com aprovação (`alterar_arquivos`), o shell deixa de ter o buraco de criar e editar, e o que ele lê de arquivo sensível não vai para a memória.

**Spec:** [`../specs/2026-10-14-james-arquivos-design.md`](../specs/2026-10-14-james-arquivos-design.md) (aprovada em 2026-10-14).
**Branch:** `feat/arquivos` (a partir de `main` em `67237dd`).

---

## Antes de começar: leia isto

Fatos conferidos no código em 2026-10-14. Cada um muda o desenho.

1. **O classificador comum não enxerga redirecionamento nem flag.** `denylist.precisaConfirmacao` (`classificador.go`) quebra o comando em tokens por `separadores`, e `>`, `<`, `=` e as aspas são separadores; depois faz `TrimLeft(token, "&.\\/-")`, então `-i` vira `i`. Resultado: `echo x > f` e `sed -i` passam hoje sem nenhum token suspeito, e é por isso que a ampliação precisa de **duas varreduras próprias** (Tasks 11 e 12), não só de mais verbos na tabela.
2. **O `denylist` é compartilhado com o PowerShell** (`denylistPowerShell`). Mexer na struct (campo novo opcional) não pode mudar o comportamento do Windows: `TestAsDuasDenylistsSaoIndependentes` guarda isso.
3. **`>` dentro de aspas não é redirecionamento** (`grep ">" f`, `awk '$1 > 3'`, `echo "a > b"`). A varredura de redirecionamento tem de respeitar aspas simples, duplas e barra invertida.
4. **Perfil:** `Restrito` (celular) só usa `NivelSeguro` sem pedir. Por isso `arquivos` é `NivelPerigoso`: no desktop roda livre e no celular pede aprovação. A escrita é `NivelPerigoso` **e** `ConfirmacaoNecessaria` sempre `true`.
5. **`ExigeConfirmacao` e `DescreveConfirmacao`** (`tipos.go`) já existem. Para `alterar_arquivos`, o cartão sai do `DescricaoDaConfirmacao`, que o `ws.go` já lê.
6. **`Parametro` não tem `enum` nem `required` por ação**: os valores aceitos ficam na descrição e a validação é no `Executar`. Número vem do JSON como `float64`.
7. **`higiene` só importa a stdlib** (sem ciclo com `ferramentas`), mas os padrões de credencial são **não exportados**: a Task 1 expõe `ParecaCredencial`. A varredura do repositório (`TestRepositorioLimpo`) acusa segredo literal em **qualquer** arquivo versionado, inclusive testes: monte as fixtures em tempo de execução (`"AIza" + "Sy" + …`, como o `chaveFalsa()` de `higiene_test.go`), e **PEM nunca num literal só** (`"-----BEGIN " + "RSA PRIVATE KEY-----"`).
8. **`SalvarNaMemoria` é o único ponto de escrita da memória** (`memoria.go`), `NivelSeguro`, e o pacote `ferramentas` não pode importar `memoria` (ciclo). O verificador de segredo entra por **interface local**, como o `vaultMemoria`.
9. **`gio trash`** existe nesta máquina (`/usr/bin/gio`). Chamar sempre com lista de argumentos e `--` antes do caminho. Código de lixeira é só unix (`//go:build unix`, como `plataforma_unix.go`); o Windows responde "indisponível".
10. **Arquivos com CRLF:** `prompts/sistema.md` e `internal/api/criar_solicitar_aprovacao_test.go`. O `Edit` falha neles: edite por script preservando o fim de linha.
11. **`gofmt -l` lista muitos arquivos antigos.** Formate só os que você cria ou muda.
12. **Comandos de suíte:**
    - Go (com o Postgres de pé):
      ```bash
      go vet ./... && GOOS=windows go vet ./... && GOOS=windows go build ./... && go test -count=1 ./...
      ```
    - Device: `dispositivo/.venv/bin/python -m pytest -q dispositivo`
    - Front: `npm test --prefix web -- --run`

    Base em `main` (`67237dd`): Go verde, pytest **506 passed, 6 skipped**, vitest **119 passed**.
13. **Nada de Python nem de front neste plano.** É só Go, mais o prompt.

---

## Estrutura de arquivos

```
internal/higiene/higiene.go, _test.go                 MODIFICAR  ParecaCredencial (exporta o que já existe)
internal/ferramentas/
  segredos.go, segredos_test.go                       CRIAR      CaminhoSensivel, SegredosLidos (valores, TTL, teto)
  memoria.go, memoria_test.go                         MODIFICAR  trava de segredo no salvar_na_memoria
  caminhos.go, caminhos_test.go                       CRIAR      resolver ~/absoluto, piso de pastas protegidas
  arquivos_leitura.go, arquivos_leitura_test.go       CRIAR      listar, ler, buscar (funções puras sobre o disco)
  arquivos.go, arquivos_test.go                       CRIAR      ferramenta `arquivos`
  escrita.go, escrita_test.go                         CRIAR      criar, editar (atômico), mover, copiar
  lixeira_unix.go, lixeira_windows.go, lixeira_test.go CRIAR     apagar para a lixeira (gio trash)
  alterar_arquivos.go, alterar_arquivos_test.go       CRIAR      ferramenta `alterar_arquivos`
  classificador.go                                    MODIFICAR  campo opcional `extras` na struct denylist
  classificador_bash.go, classificador_bash_test.go   MODIFICAR  verbos de escrita, flags, redirecionamento
cmd/jamesd/main.go (e ferramentas.go)                 MODIFICAR  registrar as duas, compartilhar SegredosLidos
prompts/sistema.md                                    MODIFICAR  (CRLF) regras 1, nova de arquivos, memória
internal/agente/anthropic/integracao_test.go          MODIFICAR  portão de tool_use com as ferramentas novas
```

---

## Fatia 1: segredo (a trava)

### Task 0: base

Sem código. Confirmar o ponto de partida e abrir a branch (já aberta).

- [ ] `git branch --show-current` → `feat/arquivos`. Rodar as três suítes (fato 12) e anotar os números no relato.

### Task 1: `higiene.ParecaCredencial`

- **Arquivos:** `internal/higiene/higiene.go`, `higiene_test.go`.
- **Testes novos:**
  - `ParecaCredencial` é verdadeira para cada padrão existente (monte as fixtures em tempo de execução, fato 7): chave do Google, `tvly-`, `nvapi-`, `sk-or-v1-`, URL com senha, `AKIA…`, token do GitHub, JWT, PEM.
  - É falsa para texto comum: "o projeto usa uma chave de API", "localhost:8080", "password", uma URL sem senha (`https://exemplo.com/a`).
  - A varredura do repositório continua achando as mesmas coisas (os testes atuais passam sem mudança: uma lista só, sem cópia).
- **Implementação:** extrair o laço de `padroes`/`padroesRegex` para uma função que serve aos dois usos; `ParecaCredencial(texto string) bool` exportada.
- **Commit:** `feat(higiene): ParecaCredencial exporta o detector de credencial`

### Task 2: `SegredosLidos` e `CaminhoSensivel` (puro)

- **Arquivos:** `segredos.go`, `segredos_test.go`.
- **Testes:**
  - `CaminhoSensivel` (tabela, com `~` já expandido): casa `~/.ssh/id_rsa`, `~/.ssh/config`, `/home/a/.env`, `/x/proj/.env.local`, `/x/.env.production`, `~/.aws/credentials`, `~/.gnupg/pubring.kbx`, `~/.netrc`, `~/.git-credentials`, `~/.npmrc`, `~/.pypirc`, `~/.kube/config`, `~/.docker/config.json`, `/x/cert.pem`, `/x/chave.key`, `/x/c.p12`, `/home/a/id_ed25519`, `/etc/shadow`, `/etc/sudoers`, `/etc/sudoers.d/x`. **Não** casa: `/x/.env.example`, `/x/ambiente.go`, `/x/README.md`, `/x/keyboard.go`, `/x/monkey.txt`.
  - `Extrair(conteudo)` devolve os valores secretos (D10 da spec): lado direito de `CHAVE=valor` e `chave: valor` com 6 ou mais caracteres (sem aspas em volta, sem comentário depois de ` #`); trechos de 16 ou mais caracteres sem espaço que tenham letra **e** dígito; cada linha do corpo de um bloco PEM. **Não** extrai: `PORT=80`, `DEBUG=true`, `host: localhost`, palavras longas só de letra ("internacionalizacao"), linhas de comentário.
  - `Registrar(caminho, conteudo)` só guarda se `CaminhoSensivel(caminho)`; arquivo comum não marca nada.
  - `Contem(texto)` é verdadeiro quando o texto tem algum valor guardado; falso para texto sem nenhum. Comparação **sem diferenciar maiúscula** em valores só de letras e dígitos? **Não:** exata (segredo é sensível a caixa).
  - TTL de 1 hora com relógio injetado: passado o prazo, `Contem` volta a falso. Teto de 500 valores: o mais antigo sai primeiro.
  - Seguro para uso concorrente (`go test -race` no teste de duas goroutines).
- **Implementação:** `type SegredosLidos struct{ mu; valores []entrada; agora func() time.Time }`, `NovoSegredosLidos()`.
- **Commit:** `feat(ferramentas): SegredosLidos e CaminhoSensivel`

### Task 3: trava no `salvar_na_memoria`

- **Arquivos:** `memoria.go`, `memoria_test.go`.
- **Testes:**
  - Com `Segredos` configurado e um valor registrado (`.env` com `CHAVE=abcdef123456`): `Executar` com nota que contém `abcdef123456` devolve erro com a mensagem da spec ("essa nota contém um valor que veio de um arquivo sensível; guarde só a conclusão, sem o valor") e **não chama** `Vault.Acrescentar`.
  - A nota "o projeto usa uma chave de API" passa.
  - Sem leitura prévia, nota que casa com `higiene.ParecaCredencial` também é recusada (mensagem própria: "essa nota parece conter uma credencial").
  - Sem `Segredos` (nil), o comportamento é o de hoje: todos os testes atuais passam sem mudança.
  - A mensagem de erro **não repete o valor** (asserção: o erro não contém `abcdef123456`).
- **Implementação:** campo `Segredos interface{ Contem(texto string) bool }` em `SalvarNaMemoria` (interface local, fato 8); a checagem da credencial usa `higiene.ParecaCredencial` direto.
- **Commit:** `feat(ferramentas): salvar_na_memoria recusa valor lido de arquivo sensível`

---

## Fatia 2: leitura (`arquivos`)

### Task 4: caminhos e piso de proteção (puro)

- **Arquivos:** `caminhos.go`, `caminhos_test.go`.
- **Testes:**
  - `ResolverCaminho("~/x")` expande a home (variável `HOME` do teste); `"/abs/x"` passa; `"x/y"`, `"./x"`, `"../x"` e `""` dão erro "diga o caminho completo (absoluto ou começando por ~)"; o caminho é limpo (`/a/../b` → `/b`).
  - `~` sozinho e `~/` viram a home. `~outro/x` (home de outro usuário) é erro.
  - `CaminhoProtegido(caminho)` é verdadeiro para: `/`, a home exata, `/etc`, `/usr`, `/var`, `/boot`, `/bin`, `/sbin`, `/lib`, `/lib64`, `/dev`, `/proc`, `/sys`, e a mesma coisa com barra no fim. **Falso** para `/etc/hosts`, `/home/a/projeto`, `/var/log/x`, `/usr/local/x`.
  - Link simbólico: `CaminhoProtegido` olha o destino real (`filepath.EvalSymlinks`) **e** o caminho pedido; um link `~/atalho → /etc` não escapa do piso.
- **Commit:** `feat(ferramentas): resolução de caminho e piso de pastas protegidas`

### Task 5: `listar`

- **Arquivos:** `arquivos_leitura.go`, `arquivos_leitura_test.go` (pasta temporária).
- **Testes:**
  - Pastas primeiro, depois arquivos, cada grupo por nome; cada linha tem tipo (`pasta`, `arquivo`, `link`), tamanho, data (`2006-01-02 15:04`) e nome; pasta mostra `/` no fim.
  - Ocultos só com `incluir_ocultos`.
  - Mais de 200 entradas: mostra 200 e uma linha "… e mais N entradas".
  - Link simbólico aparece como `link -> destino` e **não é seguido**; link quebrado não derruba a listagem.
  - Caminho que é arquivo, que não existe ou sem permissão devolve erro claro (sem `panic`, sem expor o erro cru do SO se ele trouxer outro caminho).
  - Pasta vazia: "(pasta vazia)".
- **Commit:** `feat(ferramentas): arquivos listar`

### Task 6: `ler`

- **Arquivos:** `arquivos_leitura.go`, `arquivos_leitura_test.go`.
- **Testes:**
  - Devolve as linhas numeradas (`   3  texto`); padrão 200 linhas e 64 KB por chamada; ao cortar, diz "continua na linha N" (use `a_partir_da_linha: N`).
  - `a_partir_da_linha` e `linhas` respeitam os tetos (`linhas` acima de 2000 vira 2000). Linha enorme (acima de 8 KB) é cortada com aviso, para um arquivo minificado não estourar a resposta.
  - Binário (NUL nos primeiros 8 KB) → "arquivo binário, não leio (N bytes)". UTF-8 inválido sem NUL é lido com substituição, sem erro.
  - Arquivo vazio: "(arquivo vazio)". Pasta: erro "é uma pasta, use listar". Não existe, sem permissão: erro claro.
  - **Um arquivo sensível lido registra os valores no `SegredosLidos`** (`.env` com `CHAVE=abcdef123456` → `Contem("abcdef123456")` fica verdadeiro); arquivo comum não registra nada; o registro acontece com o arquivo **inteiro**, não só com as linhas devolvidas (o modelo pode pedir em pedaços).
  - Arquivo grande demais para o registro (acima de 2 MB) registra só os primeiros 2 MB, sem travar.
- **Commit:** `feat(ferramentas): arquivos ler, com marca de segredo`

### Task 7: `buscar`

- **Arquivos:** `arquivos_leitura.go`, `arquivos_leitura_test.go`.
- **Testes:**
  - Por `nome`: trecho sem caixa e glob (`*.go`); acha em subpastas até a profundidade 8; devolve caminho e tipo.
  - Por `conteudo`: texto simples sem caixa em arquivo de texto até 2 MB; devolve `caminho:linha: trecho` (a linha cortada em 160 caracteres); pula binário e arquivo acima de 2 MB.
  - Os dois juntos: `nome` filtra e `conteudo` procura dentro.
  - Pula `.git`, `node_modules`, `.venv`, `__pycache__`, `.cache`, `vendor` quando o caminho pedido **não** está dentro de um deles; se está, não pula.
  - Teto de 100 resultados com aviso "cortado em 100"; teto de 5 s com aviso (relógio injetado); pasta sem permissão no meio é pulada, não derruba.
  - Sem `nome` nem `conteudo`: erro. Link simbólico de pasta não é seguido (evita ciclo).
  - **Busca por conteúdo em arquivo sensível não devolve a linha** (`.env`, chave): devolve só `caminho (arquivo sensível, use ler)`. Sem isso, `buscar` viraria um jeito de ler segredo sem passar pelo registro da Task 6.
- **Commit:** `feat(ferramentas): arquivos buscar`

### Task 8: a ferramenta `arquivos`

- **Arquivos:** `arquivos.go`, `arquivos_test.go`.
- **Testes:**
  - `Declaracao`: nome `arquivos`, descrição em português dizendo quando usar (olhar pastas e arquivos; para mexer, `alterar_arquivos`; não use `cat`/`ls` por shell), `acao` obrigatória, valores aceitos na descrição (fato 6).
  - `Nivel() == NivelPerigoso` (D2) e **não** implementa `ExigeConfirmacao` (D5: no desktop não pede).
  - O `Executar` despacha `listar`/`ler`/`buscar`; `acao` desconhecida ou ausente → erro com a lista das aceitas; `caminho` relativo → o erro da Task 4.
  - Números chegam como `float64` e inteiros inválidos (`linhas: 3.5`, `-1`, `"12"`) dão erro claro.
  - Compartilha o `SegredosLidos` recebido no construtor (`NovoArquivos(segredos)`; `nil` vale e só desliga o registro).
  - O perfil `Restrito` **pede aprovação** e o `Privilegiado` não (teste em `internal/permissao` ou pelo `Perfil.Pode`).
- **Commit:** `feat(ferramentas): ferramenta arquivos`

---

## Fatia 3: escrita (`alterar_arquivos`)

### Task 9: `criar` e `editar` (atômicos)

- **Arquivos:** `escrita.go`, `escrita_test.go`.
- **Testes:**
  - `criar`: cria o arquivo com o conteúdo e modo `0644`; **falha se existir** ("já existe; use sobrescrever: true"); `sobrescrever: true` troca o conteúdo **preservando o modo** do original; pasta pai inexistente é erro, e `criar_pastas: true` cria (modo `0755`); conteúdo acima de 1 MB é erro; caminho que é pasta é erro.
  - `editar`: troca `trecho_antigo` por `trecho_novo` quando aparece **exatamente uma vez**; zero ocorrências → erro com a contagem; duas ou mais → erro com a contagem e a dica de `todas: true`; com `todas: true` troca todas e diz quantas; `trecho_antigo` vazio é erro; arquivo binário ou acima de 1 MB é recusado; o modo do arquivo é preservado.
  - **Atomicidade:** a escrita vai para um temporário na mesma pasta e `os.Rename`; se o rename falhar (pasta sem permissão), o original está intacto e o temporário não sobra (`ReadDir` confere).
  - Fins de linha: `editar` num arquivo CRLF com `trecho_antigo` sem `\r` **acha** (a comparação tenta o texto como veio e, se não achar, com `\r\n`); o arquivo continua CRLF depois.
  - Link simbólico: edita **o destino** (não troca o link por um arquivo).
- **Commit:** `feat(ferramentas): criar e editar arquivos de forma atômica`

### Task 10: `mover`, `copiar` e `apagar` (lixeira)

- **Arquivos:** `escrita.go`, `lixeira_unix.go`, `lixeira_windows.go`, `escrita_test.go`, `lixeira_test.go`.
- **Testes:**
  - `mover`: não sobrescreve (destino existe → erro); destino que é uma pasta existente move **para dentro** dela? **Não:** erro "o destino já existe" (nada de adivinhar); origem inexistente é erro; entre sistemas de arquivos (`EXDEV`) cai para copiar e depois apagar a origem, e só apaga depois de conferir o tamanho copiado.
  - `copiar`: arquivo e pasta (recursivo, até 5000 arquivos e 500 MB, senão erro antes de começar); não sobrescreve; preserva o modo; não segue link simbólico para fora (copia o link como link).
  - `apagar`: chama o executor de lixeira com **lista de argumentos** `["gio","trash","--",caminho]` (executor falso nos testes); falha do executor vira erro com a mensagem dele; caminho inexistente é erro **antes** de chamar a lixeira.
  - **Piso (D11):** `apagar` e `mover` recusam `CaminhoProtegido` da Task 4, na origem, **mesmo se o chamador já aprovou** (o piso está na função, não só no cartão); `copiar` e `criar` ignoram o piso (copiar `/etc/hosts` para fora é leitura).
  - Windows: `apagar` devolve "indisponível neste sistema (Windows)"; `GOOS=windows go vet ./...` e `go build` passam.
- **Implementação:** `lixeira_unix.go` (`//go:build unix`) com `executarLixeira(ctx, caminho)` por `exec.CommandContext` (timeout 10 s); `lixeira_windows.go` com o erro de indisponível.
- **Commit:** `feat(ferramentas): mover, copiar e apagar para a lixeira, com piso de pastas`

### Task 11: a ferramenta `alterar_arquivos`

- **Arquivos:** `alterar_arquivos.go`, `alterar_arquivos_test.go`.
- **Testes:**
  - `Declaracao` e `Nivel() == NivelPerigoso`.
  - `ConfirmacaoNecessaria` é **sempre** `true` para as cinco ações, e também para ação desconhecida (falha para o lado seguro).
  - **`editar` com zero ou várias ocorrências não pede aprovação** (CA6: nada a aprovar): o `Executar` devolve o erro direto. Para isso, a checagem de ocorrência roda numa função de **validação** chamada tanto pelo `ConfirmacaoNecessaria` (que devolve `false` se o pedido é inválido de saída) quanto pelo `Executar`.
  - `DescricaoDaConfirmacao`: `criar /x/y.txt (120 bytes)`, `sobrescrever /x/y.txt (120 bytes)`, `editar /x/y.go: trocar 1 trecho (de "abc…" por "def…")` com trechos cortados em 60 caracteres e `\n` escapado, `mover /a → /b`, `copiar /a → /b`, `apagar /x/y.txt (vai para a lixeira)`.
  - Ação desconhecida ou parâmetro faltando: erro claro sem tocar no disco.
  - **Log (D13):** o `slog` registra ação e caminho e **nunca** o conteúdo nem os trechos (teste captura o handler, como o de `fala_test.go`).
  - Caminho relativo é erro.
- **Commit:** `feat(ferramentas): ferramenta alterar_arquivos`

---

## Fatia 4: o shell

### Task 12: verbos de escrita e flags

- **Arquivos:** `classificador.go` (campo opcional), `classificador_bash.go`, `classificador_bash_test.go`.
- **Testes novos (tabela):**
  - **Pedem aprovação:** `touch a`, `mkdir -p a/b`, `cp a b`, `cp -r a b`, `mv a b`, `ln -s a b`, `install -m 644 a b`, `echo x | tee f`, `tee -a f`, `rsync -a a/ b/`, `patch -p1 < x.diff`, `sed -i s/a/b/ f`, `sed -i.bak s/a/b/ f`, `sed --in-place s/a/b/ f`, `sed -ni 3p f`, `perl -pi -e 's/a/b/' f`, `awk -i inplace '{print}' f`, `/usr/bin/touch a`, `FOO=1 touch a`.
  - **Não pedem:** `ls`, `cat f`, `sed s/a/b/ f`, `sed -n 3p f`, `perl -e 'print 1'`, `awk '{print $1}' f`, `grep -i x f`, `touchscreen-calibrate` (palavra inteira), `echo touch`.
  - O PowerShell não muda (`TestAsDuasDenylistsSaoIndependentes` continua passando).
- **Implementação:**
  - Verbos novos na tabela `verbosDestrutivosBash`: `touch`, `mkdir`, `cp`, `mv`, `ln`, `install`, `tee`, `rsync`, `patch`.
  - Campo opcional `extras func(comando string) bool` na struct `denylist`; `precisaConfirmacao` o chama depois das checagens atuais. Só o `denylistBash` o preenche. Uma função `escreveEmArquivoBash` trata flags: tokeniza **sem** o `TrimLeft` do hífen e vê `sed`/`perl`/`awk` com flag que contenha `i` num conjunto de letras (`-i`, `-i.bak`, `-ni`, `-pi`, `--in-place`, `-i inplace`).
- **Commit:** `feat(ferramentas): shell pede aprovação para criar, copiar, mover e editar`

### Task 13: redirecionamento, ciente de aspas

- **Arquivos:** `classificador_bash.go`, `classificador_bash_test.go`.
- **Testes novos (tabela):**
  - **Pedem aprovação:** `echo x > f`, `echo x >> f`, `echo x>f`, `cat a > b`, `cmd >| f`, `cmd 2> erro.log`, `cmd &> f`, `cmd > "arquivo com espaço"`, `cat > f <<EOF`, `echo x > ~/.bashrc`, `ls && echo a > f`, `cmd >(outro)`.
  - **Não pedem:** `cmd 2>&1`, `cmd > /dev/null`, `cmd >/dev/null 2>&1`, `cmd 2>/dev/null`, `cmd &>/dev/null`, `cmd > /dev/stderr`, `cmd > /dev/stdout`, `cmd 1>&2`, `grep ">" f`, `awk '$1 > 3' f`, `echo "a > b"`, `echo 'a >> b'`, `echo a \> b`, `cmd < entrada`, `cmd <<EOF` (só `<`/`<<` sem `>`).
  - Aspas mal fechadas (`echo "a > b`) **pedem aprovação** (falha para o lado seguro).
- **Implementação:** varredor de uma passada com estado (fora de aspas / aspas simples / aspas duplas / escape) em `escreveEmArquivoBash`; ao achar `>` fora de aspas, lê o destino (pula espaços, tira aspas) e libera só `/dev/null`, `/dev/stdout`, `/dev/stderr` e `&N`. Sem destino legível, pede.
- **Commit:** `feat(ferramentas): shell pede aprovação para redirecionar para arquivo`

---

## Fatia 5: ligação

### Task 14: registro, prompt e portão com o modelo real

- **Arquivos:** `cmd/jamesd/main.go` (e `ferramentas.go`), `prompts/sistema.md` (CRLF), `internal/agente/anthropic/integracao_test.go`.
- **Testes e passos:**
  - Um `SegredosLidos` único criado no `main`, passado a `NovoArquivos` e ao `NovoSalvarNaMemoria` (campo `Segredos`). Teste de montagem (`registrarFerramentas`, se houver; senão, teste do registro): as duas ferramentas novas existem, e `salvar_na_memoria` recusa o valor lido por `arquivos` **no mesmo Registro** (teste de ponta a ponta sobre o registro real: lê um `.env` temporário, tenta guardar o valor, recusa).
  - **Prompt** (CRLF, por script; `git add` avisa da normalização, é esperado):
    - regra 1: tira "criar, escrever" de "faça, não pergunte"; acrescenta que criar, editar, mover, copiar e apagar **arquivos** pedem aprovação e vão por `alterar_arquivos`;
    - nova regra: olhar pastas, arquivos e conteúdo pelo `arquivos`; não usar `cat`, `ls`, `find`, `grep` por shell para ler;
    - regra de memória: `arquivos` entra na lista de ferramentas cuja saída não vai para a memória, citando `.env` e `.ssh`;
    - renumerar as regras seguintes.
  - **Portão (CA10, parte do modelo):** acrescentar ao `integracao_test.go` pedidos como "Lista o que tem na minha pasta de projetos.", "Lê o arquivo config.toml.", "Procura onde aparece a palavra jamesd nos meus arquivos.", "Cria um arquivo teste.txt na minha pasta de documentos com a palavra oi.", "Apaga o arquivo teste.txt." e conferir `tool_use` na ferramenta certa em todos, **e que nenhum caiu em `executar_comando_sistema` para ler ou escrever**. Rodar com `JAMES_TESTE_ANTHROPIC=1` (gasta frações de centavo). Se algum pedido de leitura ainda for por shell, ajustar a **descrição** das ferramentas e o prompt, e rodar de novo.
- **Commit:** `feat: arquivos e alterar_arquivos registradas, prompt e portão com o modelo real`

---

## Fatia 6: verificação

### Task 15: aceite ao vivo com o Arthur

Só o Arthur: voz e cartão de aprovação. Preparar antes uma pasta `~/teste-james/` (o James **não** mexe em nada fora dela).

- [ ] CA10 por voz: "me mostra o que tem na minha pasta de projetos"; "lê o config.toml do James"; "procura onde aparece jamesd nos meus arquivos"; "cria um arquivo teste.txt na pasta teste-james" (cartão com `criar … (N bytes)`; **negar** não cria; aprovar cria); "edita o teste.txt trocando oi por olá" (cartão com o trecho); "apaga o teste.txt" (cartão com "vai para a lixeira"; conferir na lixeira do KDE).
- [ ] CA4 ao vivo: "lê o meu `.env`" (sem aprovação, no desktop); depois "guarda na memória a chave que você acabou de ler": o James tem de recusar.
- [ ] CA3 no celular (se o Tailscale estiver de pé): a leitura pede aprovação.
- [ ] CA8 ao vivo: "roda `touch ~/teste-james/x`" pelo terminal → cartão de aprovação.

### Task 16: revisão e fechamento

- [ ] `/code-review` (nível high) contra `main` e `/security-review` (manual: `git diff main...HEAD`, já que a skill falha sem `origin`... o remoto agora existe, tentar a skill primeiro). Pontos de segurança para olhar com atenção: o piso de pastas e links simbólicos (`~/atalho → /etc`), `gio trash` com `--` e lista de argumentos, **corrida entre `ConfirmacaoNecessaria` e `Executar`** (o arquivo mudar entre a aprovação e a escrita: o `editar` confere o trecho de novo ao executar), `buscar` por conteúdo vazando segredo, o log sem conteúdo, o `SegredosLidos` sem vazar valor em mensagem de erro, e a varredura de redirecionamento (aspas e escape).
- [ ] Achados corrigidos com TDD, registrados aqui, e `ReportFindings` de novo com os desfechos.
- [ ] Suítes completas (fato 12) verdes, saída colada.
- [ ] Memória do projeto atualizada.
- [ ] Merge em `main` **só com o OK explícito do Arthur**; push só com a autorização dele (o remoto agora existe: `turos22/James`).

---

## Fora deste plano

- Lixeira e `PowerShell` no Windows (o classificador do PowerShell e o `apagar` ficam como estão).
- `sudo`, arquivo de outro usuário, edição de binário.
- Operações conscientes de git (`git reset`, `git clean`) e `chmod`/`chown` (seguem pelo shell, e já pedem aprovação).
- Observar mudança de arquivo, indexar o disco, busca semântica.
