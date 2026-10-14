# Autonomia nos arquivos: plano de implementação (versão 2)

> **Para quem executa:** um task por vez, na ordem. TDD em todos: escrever o teste, rodar e **ver falhar pelo motivo certo**, implementar, rodar e ver passar, commitar. A saída de cada "rodar" vai no relato. Não rodou, diz que não rodou. Modo trabalho: `/code-review` e `/security-review` antes do merge (Task 18).

**Objetivo:** o James olha o disco sem pedir (`arquivos`), sem que valor secreto saia do jamesd (máscara); mostra valor real só com aprovação (`revelar`); mexe no disco só com aprovação (`alterar_arquivos`); o shell deixa de ter o buraco de criar e editar; e o turno de voz avisa quando espera aprovação.

**Spec:** [`../specs/2026-10-14-james-arquivos-design.md`](../specs/2026-10-14-james-arquivos-design.md) (versão 2).
**Branch:** `feat/arquivos` (a partir de `main` em `67237dd`).

---

## Antes de começar: leia isto

Fatos conferidos no código em 2026-10-14. Cada um muda o desenho.

1. **Todo resultado de ferramenta sai do jamesd por quatro caminhos:** para a Anthropic (o modelo lê), para o Postgres (`persistirNovasMensagens` grava o JSON dos `Resultados`), para a janela de contexto e o resumo rolante (`internal/memoria/janela.go` conta e carrega `r.Saida`), e para o app (`ferramenta_concluida` leva a `Saida`). **A máscara (Task 3) tem de ser aplicada dentro da ferramenta, antes do `return`.** Mascarar depois seria tarde.
2. **O classificador comum não enxerga redirecionamento nem flag.** `denylist.precisaConfirmacao` (`classificador.go`) quebra o comando em tokens por `separadores`, e `>`, `<`, `=` e as aspas são separadores; depois faz `TrimLeft(token, "&.\\/-")`, então `-i` vira `i`. `echo x > f` e `sed -i` passam hoje sem token suspeito. Por isso o shell precisa de **varreduras próprias** (Tasks 13 e 14), não só de verbos novos.
3. **A struct `denylist` é compartilhada com o PowerShell.** Campo novo tem de ser opcional e o PowerShell não pode mudar: `TestAsDuasDenylistsSaoIndependentes` guarda isso.
4. **`>` dentro de aspas não é redirecionamento** (`grep ">" f`, `awk '$1 > 3'`, `echo "a > b"`). A varredura respeita aspas simples, duplas e barra invertida.
5. **A lista `caminhosSensiveisBash` é por substring do comando inteiro em minúsculas.** `.env` não casa com `.venv` (o ponto tem de vir colado no `e`), mas casa com `.envrc` (o que é desejável: o direnv guarda segredo).
6. **Perfil:** `Restrito` (celular, qualquer IP fora de loopback) só usa `NivelSeguro` sem pedir; `Privilegiado` (loopback) usa tudo. Aprovação aparece só no app do desktop; `timeout_aprovacao_segundos = 300`.
7. **Ordem no loop:** `despachar` chama `AoIniciarFerramenta` (que, no turno de voz, descarrega a fala pendente) **antes** de `despacharSemHook` (que pede aprovação). As chamadas de uma resposta rodam em sequência. O aviso de voz (Task 15) entra num envoltório do `SolicitarAprovacao` dentro de `processarTurno`, que conhece `modo` e `aoDelta`.
8. **O quebrador de frases fecha a frase na quebra de linha** (`proximaFrase` trata `\n`): mandar `"Preciso da sua aprovação na tela.\n"` pelo `aoDelta` faz o device falar na hora.
9. **`higiene` só importa a stdlib** (sem ciclo), mas os padrões de credencial são **não exportados** e **não incluem `sk-ant-`** (Anthropic). A varredura do repositório (`TestRepositorioLimpo`) acusa segredo literal em qualquer arquivo versionado, inclusive teste: monte fixture em tempo de execução (`"sk-" + "ant-" + …`, como o `chaveFalsa()` de `higiene_test.go`) e **nunca** escreva um cabeçalho PEM num literal só (`"-----BEGIN " + "RSA PRIVATE KEY-----"`).
10. **`SalvarNaMemoria` é o único ponto de escrita da memória** (`memoria.go`), `NivelSeguro`, e o pacote `ferramentas` não pode importar `memoria` (ciclo). O verificador entra por **interface local**, como o `vaultMemoria`.
11. **Windows:** o padrão já existe. `plataforma_unix.go` (`//go:build unix`) e `plataforma_windows.go` definem `TerminalSuportado`, e `cmd/jamesd/ferramentas.go` tem `registrarTerminal(registro, cfg, suportado)` com o teste `TestRegistrarTerminalNaoRegistraQuandoNaoSuportado`. As ferramentas novas seguem o mesmo molde (`ArquivosSuportado`).
12. **`gio trash`** existe nesta máquina (`/usr/bin/gio`). Sempre lista de argumentos e `--` antes do caminho. Em sistema de arquivos sem lixeira ele falha; a falha **nunca** vira `os.Remove`.
13. **`Parametro` não tem `enum`**: valores aceitos ficam na descrição e a validação no `Executar`. Número vem do JSON como `float64`.
14. **Arquivos com CRLF:** `prompts/sistema.md` e `internal/api/criar_solicitar_aprovacao_test.go`. O `Edit` falha neles: edite por script preservando o fim de linha. `git add` avisa da normalização para LF; é esperado.
15. **`gofmt -l` lista muitos arquivos antigos.** Formate só os seus.
16. **Comandos de suíte:**
    - Go (com o Postgres de pé):
      ```bash
      go vet ./... && GOOS=windows go vet ./... && GOOS=windows go build ./... && go test -count=1 ./...
      ```
    - Device: `dispositivo/.venv/bin/python -m pytest -q dispositivo`
    - Front: `npm test --prefix web -- --run`

    Base em `main` (`67237dd`): Go verde, pytest **506 passed, 6 skipped**, vitest **119 passed**.
17. **Nada de Python nem de front neste plano.** Go e prompt.

---

## Estrutura de arquivos

```
internal/higiene/higiene.go, _test.go                    MODIFICAR  TrechosDeCredencial, ParecaCredencial, sk-ant-
internal/ferramentas/
  sensivel.go, sensivel_test.go                          CRIAR      CaminhoSensivel (com destino de link)
  mascara.go, mascara_test.go                            CRIAR      Mascarar(conteudo, sensivel) -> (texto, ocultos)
  segredos.go, segredos_test.go                          CRIAR      SegredosLidos (valores revelados, TTL, teto)
  memoria.go, memoria_test.go                            MODIFICAR  trava no salvar_na_memoria
  caminhos.go, caminhos_test.go                          CRIAR      ResolverCaminho, CaminhoProtegido
  arquivos_leitura.go, arquivos_leitura_test.go          CRIAR      listar, ler, buscar
  arquivos.go, arquivos_test.go                          CRIAR      ferramenta `arquivos` (+ revelar)
  escrita.go, escrita_test.go                            CRIAR      criar, editar, mover, copiar
  lixeira_unix.go, lixeira_windows.go, lixeira_test.go   CRIAR      apagar para a lixeira
  alterar_arquivos.go, alterar_arquivos_test.go          CRIAR      ferramenta `alterar_arquivos`
  plataforma_unix.go, plataforma_windows.go              MODIFICAR  ArquivosSuportado
  classificador.go                                       MODIFICAR  campo opcional `extras` na struct denylist
  classificador_bash.go, classificador_bash_test.go      MODIFICAR  verbos, flags, caminhos, redirecionamento
internal/api/ws.go, fala_test.go                         MODIFICAR  aviso de voz na aprovação
cmd/jamesd/main.go, ferramentas.go, ferramentas_test.go  MODIFICAR  registrarArquivos, SegredosLidos compartilhado
prompts/sistema.md                                       MODIFICAR  (CRLF) regra 1, regra de arquivos, memória
internal/agente/anthropic/integracao_test.go             MODIFICAR  portão de tool_use
```

---

## Fatia 1: credencial, máscara e trava

### Task 0: base

- [ ] `git branch --show-current` → `feat/arquivos`. Rodar as três suítes (fato 16) e anotar os números no relato.

### Task 1: `higiene` exporta o detector e conhece a Anthropic

- **Arquivos:** `internal/higiene/higiene.go`, `higiene_test.go`.
- **Testes novos:**
  - `TrechosDeCredencial(texto) [][2]int` devolve o início e o fim de cada credencial: chave do Google, `tvly-`, `nvapi-`, `sk-or-v1-`, **`sk-ant-`** (novo), `AKIA…`, token do GitHub, JWT, cabeçalho PEM. Para padrão de prefixo fixo, o trecho vai do prefixo até o fim do token (primeiro espaço, aspa ou fim de linha). Para URL com senha, o trecho é **só a senha** (`postgres://u:SENHA@h` → só `SENHA`).
  - `ParecaCredencial(texto) bool` = `len(TrechosDeCredencial) > 0`.
  - Falso para texto comum: "o projeto usa uma chave de API", "localhost:8080", "password", `https://exemplo.com/a`, `sk-` sozinho.
  - **A varredura do repositório acusa uma chave `sk-ant-` montada no teste** (fixture em tempo de execução, fato 9) e o repositório real continua limpo (`TestRepositorioLimpo` verde).
- **Implementação:** uma lista só (a dos padrões atuais mais `"sk-" + "ant-"`, montado assim no próprio `higiene.go` para o arquivo não se acusar), usada pela varredura e pelas funções novas.
- **Commit:** `feat(higiene): exporta o detector de credencial e reconhece chave da Anthropic`

### Task 2: `CaminhoSensivel`

- **Arquivos:** `sensivel.go`, `sensivel_test.go`.
- **Testes:**
  - Tabela (caminho já absoluto): casa `~/.ssh/id_rsa`, `~/.ssh/config`, `/x/.env`, `/x/.env.local`, `/x/.env.production`, `~/.aws/credentials`, `~/.gnupg/x`, `~/.netrc`, `~/.git-credentials`, `~/.npmrc`, `~/.pypirc`, `~/.kube/config`, `~/.docker/config.json`, `/x/c.pem`, `/x/c.key`, `/x/c.p12`, `/x/c.pfx`, `/h/id_ed25519`, `/h/id_ecdsa.pub` (casa por `id_ecdsa*`; mascarar a chave pública não custa nada), `/etc/shadow`, `/etc/gshadow`, `/etc/sudoers`, `/etc/sudoers.d/x`.
  - **Não** casa: `/x/.env.example`, `/x/.env.sample`, `/x/.env.template`, `/x/.venv/bin/python`, `/x/ambiente.go`, `/x/keyboard.go`, `/x/README.md`.
  - **Link simbólico** (pasta temporária): `~/atalho → ~/.ssh/id_rsa` é sensível; `~/atalho2 → /x/README.md` não; link quebrado devolve "não sensível" sem erro.
- **Commit:** `feat(ferramentas): CaminhoSensivel, inclusive por link simbólico`

### Task 3: `Mascarar` (puro)

- **Arquivos:** `mascara.go`, `mascara_test.go`.
- **Assinatura:** `Mascarar(conteudo string, sensivel bool) (texto string, ocultos []string)`. `ocultos` são os valores escondidos, para o `revelar` registrar (Task 9).
- **Testes (fixtures montadas em tempo de execução, fato 9):**
  - **Qualquer arquivo:** credencial reconhecível vira `••••` (o trecho da Task 1); bloco PEM de chave privada (do `BEGIN` ao `END`, inclusive as duas linhas) vira `[chave privada omitida: N linhas]`; o resto do texto fica idêntico, byte a byte.
  - **Arquivo sensível:** `CHAVE=valor` → `CHAVE=•••• (oculto, 5 caracteres)`; `export CHAVE="valor"` → aspas fora da contagem; `chave: valor` (YAML); `"auth": "valor"` (JSON); `machine x login y password z` (`.netrc`) → só `z`; `postgres://u:senha@h:5432/db` → `postgres://u:••••@h:5432/db`; linha vazia, comentário (`# …`) e `CHAVE=` sem valor ficam como estão.
  - **Arquivo comum** (`sensivel=false`): `PORT=8080` e `DEBUG=true` ficam visíveis; só os formatos reconhecíveis e o PEM são mascarados.
  - `ocultos` traz exatamente os valores escondidos, sem repetir.
  - A máscara é estável: aplicar duas vezes dá o mesmo texto.
- **Commit:** `feat(ferramentas): máscara de valores secretos`

### Task 4: `SegredosLidos` e a trava na memória

- **Arquivos:** `segredos.go`, `segredos_test.go`, `memoria.go`, `memoria_test.go`.
- **Testes (`SegredosLidos`):**
  - `Registrar(valores)` guarda só valores com 6 ou mais caracteres; `Contem(texto)` é verdadeiro se o texto tem algum valor guardado (comparação exata, segredo é sensível a caixa).
  - TTL de 1 hora com relógio injetado; teto de 500, o mais antigo sai primeiro.
  - Concorrência: duas goroutines registrando e consultando, sem corrida (`go test -race ./internal/ferramentas/`).
- **Testes (`salvar_na_memoria`):**
  - Com `Segredos` configurado e `abcdef123456` registrado: nota com `abcdef123456` → erro "essa nota contém um valor secreto que você leu de um arquivo; guarde só a conclusão, sem o valor", **sem** chamar `Vault.Acrescentar`, e o erro **não contém** o valor.
  - "o projeto usa uma chave de API" passa.
  - Nota com credencial reconhecível (`higiene.ParecaCredencial`) é recusada mesmo sem registro ("essa nota parece conter uma credencial").
  - `Segredos` nil: comportamento de hoje, testes atuais passam sem mudança.
- **Implementação:** campo `Segredos interface{ Contem(string) bool }` em `SalvarNaMemoria` (fato 10).
- **Commit:** `feat(ferramentas): SegredosLidos e trava no salvar_na_memoria`

---

## Fatia 2: leitura (`arquivos`)

### Task 5: caminhos e piso (puro sobre o disco)

- **Arquivos:** `caminhos.go`, `caminhos_test.go`.
- **Testes:**
  - `ResolverCaminho`: `~/x` e `~` expandem a home (`HOME` do teste); `/abs/x` passa limpo (`/a/../b` → `/b`); `x/y`, `./x`, `../x`, `""`, `~outro/x` dão "diga o caminho completo (absoluto ou começando por ~)".
  - `CaminhoProtegido`: verdadeiro para `/`, a home exata, `/etc`, `/usr`, `/var`, `/boot`, `/bin`, `/sbin`, `/lib`, `/lib64`, `/dev`, `/proc`, `/sys`, `/opt`, `/srv`, `/root` (com e sem barra no fim); falso para `/etc/hosts`, `/home/a/projeto`, `/var/log/x`, `/usr/local/x`.
  - **Link:** `~/atalho → /etc` e `~/casa → ~` são protegidos (olha o destino real **e** o pedido).
- **Commit:** `feat(ferramentas): resolução de caminho e piso de pastas protegidas`

### Task 6: `listar`

- **Arquivos:** `arquivos_leitura.go`, `arquivos_leitura_test.go`.
- **Testes:** pastas primeiro, depois arquivos, por nome; linha com tipo (`pasta`/`arquivo`/`link`), tamanho, data (`2006-01-02 15:04`) e nome (pasta com `/`); ocultos só com `incluir_ocultos`; 200 entradas e "… e mais N"; link aparece como `link -> destino` sem ser seguido, link quebrado não derruba; pasta vazia → "(pasta vazia)"; arquivo, inexistente e sem permissão → erro claro.
- **Commit:** `feat(ferramentas): arquivos listar`

### Task 7: `ler`

- **Arquivos:** `arquivos_leitura.go`, `arquivos_leitura_test.go`.
- **Testes:**
  - Linhas numeradas (`   3  texto`); padrão 200 linhas e 32 KB; `linhas` acima de 2000 vira 2000; linha acima de 8 KB cortada com aviso; ao cortar, "continua na linha N".
  - Binário (NUL nos primeiros 8 KB) → "arquivo binário, não leio (N bytes)"; UTF-8 inválido sem NUL é lido com substituição; vazio → "(arquivo vazio)"; pasta → "é uma pasta, use listar"; inexistente e sem permissão → erro claro.
  - **Máscara aplicada** (Task 3): `.env` com `CLAUDE_API_KEY=` e um valor montado → a saída tem `CLAUDE_API_KEY=•••• (oculto, …)` e **não contém** o valor; arquivo comum com chave reconhecível e PEM → os dois mascarados; link para `.ssh/id_rsa` → mascarado.
  - A máscara é aplicada **antes** de cortar em linhas: um bloco PEM que começa na linha 1 e o pedido de `a_partir_da_linha: 3` não vaza o miolo do bloco.
  - `lerSemMascara` (uso interno do `revelar`) devolve o texto cru e a lista `ocultos`.
- **Commit:** `feat(ferramentas): arquivos ler, com máscara`

### Task 8: `buscar`

- **Arquivos:** `arquivos_leitura.go`, `arquivos_leitura_test.go`.
- **Testes:**
  - `nome` (trecho sem caixa e glob `*.go`) em subpastas até a profundidade 8.
  - `conteudo` (sem caixa) em texto até 2 MB → `caminho:linha: trecho` (160 caracteres); pula binário e arquivo grande; `nome` e `conteudo` juntos.
  - **D9:** em arquivo sensível, a linha que casa sai mascarada (`~/.env:3: CLAUDE_API_KEY=••••`); buscar pelo **próprio valor** de um segredo devolve o caminho e a linha mascarada, nunca o valor.
  - A máscara de formato reconhecível vale em arquivo comum também.
  - Pula `.git`, `node_modules`, `.venv`, `__pycache__`, `.cache`, `vendor` fora deles e não pula se o caminho pedido está dentro; 100 resultados com aviso; 5 s com aviso (relógio injetado); pasta sem permissão no meio é pulada; link de pasta não é seguido; sem `nome` nem `conteudo` → erro.
- **Commit:** `feat(ferramentas): arquivos buscar`

### Task 9: a ferramenta `arquivos` (com `revelar`)

- **Arquivos:** `arquivos.go`, `arquivos_test.go`.
- **Testes:**
  - `Declaracao`: nome `arquivos`; descrição diz quando usar (olhar pastas e arquivos; para mexer, `alterar_arquivos`; não use `cat`/`ls`/`find` por shell); explica que valores secretos aparecem como `••••` de propósito e que `revelar` mostra o valor com aprovação.
  - `Nivel() == NivelPerigoso`.
  - `ConfirmacaoNecessaria`: **verdadeiro só para `revelar`**; falso para `listar`, `ler`, `buscar`.
  - `DescricaoDaConfirmacao` de `revelar`: `mostrar os valores secretos de /x/.env ao modelo (vão para a Anthropic e ficam no histórico desta conversa)`; vazio para as outras.
  - `revelar` devolve o texto sem máscara **e** registra os `ocultos` no `SegredosLidos`; `ler` não registra nada (nada foi revelado).
  - Despacho, ação desconhecida, caminho relativo e número inválido (`3.5`, `-1`, `"12"`) → erro claro.
  - `Restrito` pede aprovação para `ler` (pelo `Perfil.Pode`), `Privilegiado` não.
  - Ponta a ponta no pacote: `revelar` um `.env` temporário e tentar `salvar_na_memoria` com o valor → recusado (os dois usam o **mesmo** `SegredosLidos`).
- **Commit:** `feat(ferramentas): ferramenta arquivos, com revelar sob aprovação`

---

## Fatia 3: escrita (`alterar_arquivos`)

### Task 10: `criar` e `editar` (atômicos)

- **Arquivos:** `escrita.go`, `escrita_test.go`.
- **Testes:**
  - `criar`: arquivo novo com modo `0644`; existente → "já existe; use sobrescrever: true"; `sobrescrever: true` troca e **preserva o modo**; pasta pai inexistente → erro, e `criar_pastas: true` cria (`0755`); acima de 1 MB → erro; caminho que é pasta → erro.
  - `editar`: uma ocorrência → troca; zero → erro com a contagem; duas ou mais → erro com a contagem e a dica de `todas`; `todas: true` troca e diz quantas; `trecho_antigo` vazio → erro; binário ou acima de 1 MB → recusa; modo preservado.
  - **CRLF:** `trecho_antigo` com `\n` acha em arquivo CRLF, e o arquivo continua CRLF.
  - **Atômico:** grava num temporário na mesma pasta e faz `rename`; se o `rename` falhar, o original fica intacto e o temporário não sobra (`ReadDir`).
  - **Link:** edita o destino, não troca o link por arquivo.
  - **Validação separada da execução:** `validarCriar`/`validarEditar` são funções puras chamadas pela ferramenta antes do cartão e de novo no `Executar` (Task 12).
- **Commit:** `feat(ferramentas): criar e editar arquivos de forma atômica`

### Task 11: `mover`, `copiar` e `apagar` (lixeira)

- **Arquivos:** `escrita.go`, `lixeira_unix.go`, `lixeira_windows.go`, `escrita_test.go`, `lixeira_test.go`.
- **Testes:**
  - `mover`: não sobrescreve ("o destino já existe"); origem inexistente → erro; entre sistemas de arquivos (`EXDEV`, só em unix) copia, confere tamanho e só então remove a origem (teste com uma função de `rename` injetada que devolve `EXDEV`).
  - `copiar`: arquivo e pasta (até 5000 arquivos e 500 MB, senão erro **antes** de começar); não sobrescreve; preserva modo; copia link como link.
  - `apagar`: chama o executor com `["gio","trash","--",caminho]` (executor falso); erro do executor volta com a mensagem dele e **o arquivo continua lá** (nenhum `os.Remove` em nenhum caminho de erro); inexistente → erro antes de chamar a lixeira.
  - **Piso:** `apagar` e `mover` recusam `CaminhoProtegido` (Task 5), inclusive por link, **mesmo chamados direto** (o piso está na função, não só no cartão).
  - Windows: `lixeira_windows.go` devolve "indisponível neste sistema (Windows)"; `GOOS=windows go vet ./...` e `go build ./...` passam.
- **Commit:** `feat(ferramentas): mover, copiar e apagar para a lixeira, com piso`

### Task 12: a ferramenta `alterar_arquivos`

- **Arquivos:** `alterar_arquivos.go`, `alterar_arquivos_test.go`.
- **Testes:**
  - `Declaracao` e `Nivel() == NivelPerigoso`.
  - `ConfirmacaoNecessaria`: verdadeiro para as cinco ações e para ação desconhecida; **falso para pedido inválido de saída** (editar com 0 ou 2+ ocorrências, destino que já existe, caminho relativo, piso, origem inexistente). Nesse caso o `Executar` devolve o erro direto, sem cartão.
  - `DescricaoDaConfirmacao`: `criar /x/y.txt (120 bytes)`, `sobrescrever …`, `editar /x/y.go: trocar 1 trecho (de "abc…" por "def…")` (60 caracteres, `\n` escapado), `mover /a → /b`, `copiar /a → /b`, `apagar /x/y.txt (vai para a lixeira)`. **Arquivo sensível**: `editar /x/.env: trocar 1 trecho`, sem trecho.
  - **Reconferência:** aprovar (`ConfirmacaoNecessaria` verdadeiro), mudar o arquivo por fora (o trecho some, ou o destino passa a existir) e chamar `Executar` → erro e nada escrito.
  - **Log sem conteúdo:** o `slog` registra ação e caminho e **nunca** o conteúdo nem os trechos (handler capturado, como em `fala_test.go`).
- **Commit:** `feat(ferramentas): ferramenta alterar_arquivos`

---

## Fatia 4: o shell

### Task 13: verbos, flags e caminhos sensíveis

- **Arquivos:** `classificador.go` (campo opcional), `classificador_bash.go`, `classificador_bash_test.go`.
- **Testes novos (tabela):**
  - **Pedem:** `touch a`, `mkdir -p a/b`, `cp a b`, `mv a b`, `ln -s a b`, `install -m 644 a b`, `echo x | tee f`, `rsync -a a/ b/`, `patch -p1 < x.diff`, `zip a.zip f`, `unzip a.zip`, `gzip f`, `gunzip f.gz`, `xz f`, `zstd f`, `7z x a.7z`, `split -b 1M f`, `tar -czf a.tgz d`, `tar xf a.tar`, `tar --extract -f a.tar`, `sed -i s/a/b/ f`, `sed -i.bak …`, `sed --in-place …`, `sed -ni 3p f`, `perl -pi -e … f`, `awk -i inplace '{print}' f`, `/usr/bin/touch a`, `FOO=1 touch a`, **`cat .env`**, `cat ~/.aws/credentials`, `less ~/.netrc`, `cat ~/.git-credentials`, `cat id_rsa`.
  - **Não pedem:** `ls`, `cat f`, `sed s/a/b/ f`, `sed -n 3p f`, `perl -e 'print 1'`, `awk '{print $1}' f`, `grep -i x f`, `tar -tf a.tar`, `tar --list -f a.tar`, `unzip -l a.zip`, `zcat f.gz`, `touchscreen-x`, `echo touch`, `ls .venv`. (`source .venv/bin/activate` já pede hoje, porque `source` é verbo da lista: não muda.)
  - O PowerShell não muda.
- **Implementação:** verbos novos em `verbosDestrutivosBash`; caminhos novos em `caminhosSensiveisBash` (`.env`, `.aws/`, `.gnupg/`, `.netrc`, `.git-credentials`, `.npmrc`, `.pypirc`, `id_rsa`, `id_ed25519`, `id_ecdsa`); campo `extras func(string) bool` na struct `denylist`, preenchido só no bash, com uma função que tokeniza **sem** aparar o hífen e olha `sed`/`perl`/`awk` com edição no lugar e `tar` com `c`/`x`/`r`/`u`/`A` no primeiro grupo de letras ou `--create`/`--extract`/`--append`/`--update`.
- **Commit:** `feat(ferramentas): shell pede aprovação para criar, copiar, mover, editar e ler segredo`

### Task 14: redirecionamento, ciente de aspas

- **Arquivos:** `classificador_bash.go`, `classificador_bash_test.go`.
- **Testes novos (tabela):**
  - **Pedem:** `echo x > f`, `echo x >> f`, `echo x>f`, `cmd >| f`, `cmd 2> erro.log`, `cmd &> f`, `cmd > "arq com espaço"`, `cat > f <<EOF`, `ls && echo a > f`, `cmd >(outro)`, aspas mal fechadas `echo "a > b`.
  - **Não pedem:** `cmd 2>&1`, `cmd > /dev/null`, `cmd >/dev/null 2>&1`, `cmd 2>/dev/null`, `cmd &>/dev/null`, `cmd > /dev/stderr`, `cmd > /dev/stdout`, `cmd 1>&2`, `grep ">" f`, `awk '$1 > 3' f`, `echo "a > b"`, `echo 'a >> b'`, `echo a \> b`, `cmd < entrada`, `cmd <<EOF`.
- **Implementação:** varredor de uma passada com estado (fora de aspas, aspas simples, aspas duplas, escape); ao achar `>` fora de aspas, lê o destino (pula espaço, tira aspas) e libera só `/dev/null`, `/dev/stdout`, `/dev/stderr` e `&N`. Destino ilegível pede.
- **Commit:** `feat(ferramentas): shell pede aprovação para redirecionar para arquivo`

---

## Fatia 5: ligação

### Task 15: aviso de voz na aprovação

- **Arquivos:** `internal/api/ws.go`, `internal/api/fala_test.go`.
- **Testes:**
  - Turno de **voz** em que o provedor pede uma ferramenta que exige confirmação (ferramenta de teste registrada no `Loop.Ferramentas` com `ConfirmacaoNecessaria` verdadeiro; `TimeoutAprovacao` curto): o device recebe `fala_frase` "Preciso da sua aprovação na tela." **antes** do fim do turno.
  - Turno de **texto** com a mesma ferramenta: o device não recebe nada (o texto não fala por ele).
  - Turno de voz sem aprovação: nenhum aviso extra (os testes atuais de ordem de frases continuam passando).
- **Implementação:** em `processarTurno`, quando `modo == agente.ModoVoz` e `aoDelta != nil`, envolver o `SolicitarAprovacao`: chamar `aoDelta("Preciso da sua aprovação na tela.\n")` e só então esperar (fatos 7 e 8).
- **Commit:** `feat(api): o James avisa por voz quando espera aprovação`

### Task 16: registro, prompt e portão com o modelo real

- **Arquivos:** `plataforma_unix.go`, `plataforma_windows.go`, `cmd/jamesd/main.go`, `ferramentas.go`, `ferramentas_test.go`, `prompts/sistema.md` (CRLF), `internal/agente/anthropic/integracao_test.go`.
- **Testes e passos:**
  - `ArquivosSuportado` (unix `true`, windows `false`); `registrarArquivos(registro, segredos, suportado)` registra as duas ferramentas só quando suportado (teste espelhando `TestRegistrarTerminalNaoRegistraQuandoNaoSuportado`).
  - Um `SegredosLidos` único criado no `main`, entregue a `NovoArquivos` e ao `NovoSalvarNaMemoria` (campo `Segredos`).
  - **Prompt** (por script, CRLF):
    - regra 1: sai "criar, escrever"; entra que criar, editar, mover, copiar e apagar **arquivos** pedem aprovação e vão por `alterar_arquivos`;
    - regra nova depois da 5: olhar pastas, arquivos e conteúdo pelo `arquivos`; não usar `cat`, `ls`, `find`, `grep` por shell para ler; `••••` é proposital; `revelar` só quando o Arthur pedir o valor;
    - regra de memória (hoje a 8): `arquivos` na lista de ferramentas cuja saída não vai para a memória;
    - renumerar.
  - **Portão (CA14):** acrescentar ao `integracao_test.go` "Lista o que tem na minha pasta de projetos.", "Lê o arquivo config.toml do James.", "Procura onde aparece a palavra jamesd nos meus arquivos.", "Cria um arquivo teste.txt na minha pasta de documentos com a palavra oi.", "Apaga o arquivo teste.txt da minha pasta de documentos." e exigir `tool_use` em `arquivos`/`alterar_arquivos`, **nenhum** em `executar_comando_sistema`. Rodar com `JAMES_TESTE_ANTHROPIC=1`. Se algum cair no shell, ajustar descrição e prompt e rodar de novo; registrar quantas rodadas.
- **Commit:** `feat: arquivos e alterar_arquivos registradas, prompt e portão com o modelo real`

---

## Fatia 6: verificação

### Task 17: aceite ao vivo com o Arthur

Só o Arthur: voz e cartão. Antes, uma pasta `~/teste-james/` (nada fora dela é alterado).

- [ ] CA15 por voz: "me mostra o que tem na minha pasta de projetos"; "lê o config.toml do James"; "o meu .env tem a chave do Claude preenchida?" (a resposta diz que sim **sem dizer o valor**); "cria um arquivo teste.txt na pasta teste-james com a palavra oi" (aviso de voz, cartão `criar … (N bytes)`; negar não cria; aprovar cria); "troca oi por olá no teste.txt" (cartão com o trecho); "apaga o teste.txt" (cartão "vai para a lixeira"; conferir na lixeira do KDE).
- [ ] CA6 ao vivo: "me mostra a chave do Claude" → cartão de `revelar` com o aviso da Anthropic; aprovar; depois "guarda essa chave na memória" → recusa.
- [ ] CA5 no celular (se o Tailscale estiver de pé): `ler` pede aprovação.
- [ ] CA11 ao vivo: "roda `touch ~/teste-james/x`" → cartão.

### Task 18: revisão e fechamento

- [ ] `/code-review` (high) contra `main` e `/security-review` (o remoto existe; se a skill falhar, manual com `git diff main...HEAD`). Olhar com atenção: máscara (formatos, PEM partido entre páginas, JSON, link), `revelar` registrando e a trava, piso e link, `gio trash` com `--` e sem queda para `os.Remove`, reconferência no `Executar`, `buscar` como atalho de leitura, log sem conteúdo, varredura de redirecionamento (aspas e escape), o aviso de voz.
- [ ] Achados corrigidos com TDD, registrados aqui, `ReportFindings` de novo com os desfechos.
- [ ] Suítes completas verdes, saída colada.
- [ ] Memória do projeto atualizada.
- [ ] Merge em `main` só com o OK explícito do Arthur; push só com a autorização dele (`turos22/James`).

---

## Fora deste plano

- Windows (ferramentas novas não registradas lá; PowerShell como está).
- `sudo`, arquivo de outro usuário, edição de binário.
- Programas que escrevem arquivo como efeito colateral (`git`, compiladores, gerenciadores de pacote).
- Máscara de segredo em formato desconhecido fora de arquivo sensível (R1 da spec).

---

## Registro da execução (2026-10-14)

Tasks 0 a 16 feitas, cada uma com teste vermelho antes, mutação do ponto de segurança e suíte Go verde (`go vet`, `GOOS=windows go vet/build`, `go test`). Commits: `c1defca`, `5244c6a`, `cf4659c`, `349f594`, `ec99165`, `989c38b`, `635b011`, `530acd4`, `d5dcc76`, `9a33d15`, `635b344`, `4ac0328`, `541d743`, `9a1995d`, `1ef8d81`, `7638c4a`.

### Desvios da spec e achados

- **A busca por conteúdo roda no texto já mascarado** (Task 8). A spec dizia que buscar pelo valor "devolve o caminho e a linha mascarada"; isso viraria um **oráculo** (o modelo perguntaria "o `.env` contém tal valor?" e a resposta confirmaria o palpite). Agora buscar pelo valor de um segredo não acha nada e não vaza nada.
- **Uma mutação minha não foi aplicada** na Task 8 (o `sed` procurava um literal diferente do que estava no arquivo) e passou em silêncio. Foi refeita com um script que **imprime quantas trocas fez**, e as duas falhas esperadas apareceram. Desde então todas as mutações confirmam a contagem de trocas.
- **Quebrador de frases** (Task 15): o fato 8 do plano estava errado. Ponto no fim do buffer esperava o próximo caractere mesmo quando era uma quebra de linha, então o aviso de aprovação por voz só saía **depois** da aprovação expirar (medido: +3,01 s com a aprovação pendente por 3 s). Corrigido no `internal/voz/fala.go`: ponto + quebra de linha fecha a frase na hora.
- **`install` só no começo do comando** (Task 13): `pip install` e `npm install` continuam livres; `install -m 644 a b` (o comando do coreutils) pede. Os verbos da tabela (`touch`, `cp`, `mv`...) casam em **qualquer posição**, como o resto da tabela já fazia, então `echo touch` pede aprovação (o plano dizia o contrário).
- **Testes antigos mudaram de lado de propósito:** `mkdir -p`, `cp` e `echo > arquivo` estavam como "passa livre" nos testes da denylist; a regra do Arthur inverte exatamente isso.
- **Quirk antigo, não é desta fatia:** `du -sh .` pede aprovação desde antes (o classificador apara o hífen e `-sh` vira `sh`, que é verbo de shell). Anotado como dívida, sem mexer.
- **`prompts/sistema.md` agora é LF**, não CRLF (o git normalizou num commit anterior): a edição detecta o fim de linha.
- **Portão com o Haiku 5.5** (CA14): 20 de 20 em duas rodadas; os sete pedidos de arquivo nunca caem no terminal. "Troca a palavra oi por olá no arquivo" começa por `arquivos` (olha antes de editar), o que é o certo. "Lista os arquivos da pasta atual" oscila entre `arquivos` e terminal (relativo e ambíguo de propósito).
- **Prova ao vivo com dado real:** o `.env` do projeto tem 5 valores; `ler` e `buscar` no projeto inteiro não deixaram nenhum aparecer, e os 5 nomes de chave ficaram visíveis.

### O que falta

- Task 17 (aceite com o Arthur: voz, cartão de aprovação, lixeira do KDE, celular).
- Task 18 (revisão, memória, merge e push só com o OK dele).
