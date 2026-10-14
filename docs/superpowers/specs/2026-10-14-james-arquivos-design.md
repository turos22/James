# James: autonomia nos arquivos

Status: **rascunho para aprovação do Arthur** (2026-10-14)
Branch: `feat/arquivos` (a partir de `main` em `67237dd`)
Origem: `docs/superpowers/decisoes/2026-10-13-james-autonomia-no-sistema.md`

## 1. Contexto

### 1.1 O que o Arthur pediu

"Quero que ele possa navegar entre os arquivos do meu sistema de forma mais autônoma; porém criar, apagar ou editar necessitam da minha aprovação." No aceite do B2 ele sentiu o James sem autonomia: pediu para abrir e focar janelas, e o James não tinha como olhar o disco.

### 1.2 Como é hoje (lido no código em 2026-10-14)

- **Ler arquivos** só por shell (`cat`, `ls`, `find`), pela ferramenta `executar_comando_sistema`. A denylist (`classificador_bash.go`) **pede aprovação para ler** `.ssh/`, `.bashrc`, `/etc/`, `/boot/` e outros caminhos sensíveis.
- **Apagar** está coberto na denylist (`rm`, `rmdir`, `shred`, `truncate`, `unlink`).
- **Criar e editar NÃO estão cobertos:** `touch`, `mkdir`, `cp`, `mv`, `ln`, `install`, `tee`, `rsync`, `patch`, `sed -i`, `perl -i` rodam livres, e o redirecionamento `>`/`>>` está fora **de propósito** ("não reprova", comentário do arquivo). O `mv` sobrescreve sem avisar, então na prática também apaga.
- `prompts/sistema.md`, regra 1, manda "criar, escrever, abrir… faça, não pergunte". Contradiz a regra nova.
- O gate de aprovação já tem os ganchos: `ExigeConfirmacao` (decide pelos argumentos) e `DescreveConfirmacao` (o cartão mostra o que vai acontecer, do B2).
- Perfil: o celular (`Restrito`) só usa ferramenta `NivelSeguro` sem pedir; o desktop (`Privilegiado`) usa tudo.
- A memória é gravada por um ponto só, `salvar_na_memoria`.

## 2. Escopo

### 2.1 Entra

1. **`arquivos`**: `listar`, `ler`, `buscar`. Sem aprovação no desktop.
2. **`alterar_arquivos`**: `criar`, `editar`, `mover`, `copiar`, `apagar`. **Sempre** com aprovação, e o cartão mostra o que muda. `apagar` manda para a lixeira.
3. **Denylist do shell ampliada**: criar, editar, copiar e mover passam a pedir aprovação, inclusive o redirecionamento para arquivo.
4. **Trava de segredo**: o que o James lê de arquivo sensível não pode ir para a memória.
5. **Prompt**: usar `arquivos` para olhar, `alterar_arquivos` para mexer, nunca guardar conteúdo lido na memória.

### 2.2 Não entra

- `sudo`, arquivo de outro usuário, edição de binário.
- Lixeira no Windows (as ações de escrita respondem "indisponível" lá, menos `apagar`, que fica fora).
- Mudar o classificador do PowerShell: o Windows segue como está (dívida anotada).
- Permissões (`chmod`/`chown`) continuam pelo shell, e já pedem aprovação.
- Operações conscientes de git (`git reset`, `git clean`): seguem pelo shell.

## 3. Decisões

| # | Decisão | Por quê |
|---|---|---|
| D1 | Duas ferramentas: `arquivos` (leitura) e `alterar_arquivos` (escrita). | `Nivel()` é por ferramenta, não por ação. Separar deixa a leitura e a escrita com regras diferentes sem condicional escondido. |
| D2 | `arquivos` é `NivelPerigoso`. | O desktop (`Privilegiado`) roda livre, e o celular (`Restrito`) **pede aprovação** a cada leitura, porque quem lê `.ssh` pelo celular merece um cartão no desktop. |
| D3 | `alterar_arquivos` é `NivelPerigoso` **e** `ConfirmacaoNecessaria` sempre `true`. | A regra do Arthur: criar, editar e apagar sempre com aprovação, no desktop também. |
| D4 | Leitura alcança tudo o que o usuário do jamesd já lê (home, `/etc`, `/var/log`, projetos). Sem `sudo`; erro de permissão volta como erro normal. | Decisão do Arthur (2026-10-14). |
| D5 | Ler `.ssh`, `.env` e afins **não pede aprovação** no desktop, mas marca o conteúdo como segredo lido (D9). | Decisão do Arthur: "sim, desde que não guarde na memória". |
| D6 | O shell **continua pedindo** aprovação para ler caminho sensível. | `arquivos` é o caminho livre; o shell continua conservador como defesa. O prompt manda ler por `arquivos`. |
| D7 | Caminho: absoluto ou começando por `~`. Relativo é erro ("diga o caminho completo"). | O jamesd não tem "pasta atual" que o Arthur enxergue. |
| D8 | `editar` troca um trecho **exato** (`trecho_antigo` → `trecho_novo`), que tem de aparecer exatamente uma vez (ou `todas: true`). `criar` não sobrescreve (a menos que `sobrescrever: true`). `mover` e `copiar` não sobrescrevem. | Evita reescrever o arquivo inteiro e perder o que o modelo não viu; sobrescrita acidental é o furo do `mv` de hoje. |
| D9 | **Trava de segredo**: um objeto `SegredosLidos` é compartilhado entre `arquivos` e `salvar_na_memoria`. Ao ler arquivo sensível, os valores dele (D10) entram nele por 1 hora. `salvar_na_memoria` recusa nota que contenha um desses valores ou que case com um padrão de chave. | O prompt já pede "não copie saída de ferramenta", mas é só pedido ao modelo; a trava é de código. |
| D10 | Arquivo **sensível** = caminho casa com a lista (`.env*`, `.ssh/`, `.aws/credentials`, `.gnupg/`, `.netrc`, `.git-credentials`, `.npmrc`, `.pypirc`, `.kube/config`, `.docker/config.json`, `*.pem`, `*.key`, `*.p12`, `id_*`, `/etc/shadow`, `/etc/sudoers*`). Valor secreto = o lado direito de `CHAVE=valor`/`chave: valor` com 6 ou mais caracteres, qualquer trecho de 16 ou mais caracteres sem espaço que tenha letra e dígito, e cada linha de um bloco PEM. | Cobre `.env` e chaves sem marcar como segredo qualquer palavra comprida. |
| D11 | Apagar = **lixeira** (`gio trash -- caminho`, lista de argumentos, nunca shell). Recusa, mesmo aprovado: `/`, a home inteira e as pastas de sistema de topo (`/etc`, `/usr`, `/var`, `/boot`, `/bin`, `/lib*`, `/sbin`, `/dev`, `/proc`, `/sys`). | Apagar recuperável. O piso de segurança existe porque um clique de aprovação apressado não pode apagar a home. |
| D12 | Escrita atômica: arquivo temporário na mesma pasta + `rename`, preservando o modo. Teto de 1 MB por `criar`/`editar`. | Queda no meio não deixa arquivo pela metade. |
| D13 | Cada alteração loga ação e caminho, **nunca** o conteúdo. | O log não vira outro lugar de segredo. |
| D14 | Denylist do shell passa a pedir aprovação para: `touch`, `mkdir`, `cp`, `mv`, `ln`, `install`, `tee`, `rsync`, `patch`; `sed`, `perl`, `awk` quando houver `-i`/`--in-place`; e **redirecionamento** `>`, `>>`, `>|` para destino que não seja `/dev/null`, `/dev/std*` nem `&N` (`2>&1` continua livre). | Fecha o buraco que a verificação achou. Continua valendo o limite honesto do arquivo: comando montado de forma ofuscada escapa, e `python -c "open(...).write(...)"` também. |

## 4. Ferramenta `arquivos`

| Ação | Parâmetros | Regras |
|---|---|---|
| `listar` | `caminho`, `incluir_ocultos`? | Pastas primeiro, depois arquivos, por nome. Cada linha: tipo, tamanho, data de modificação, nome. Até 200 entradas, com aviso de "mais N". Link simbólico aparece como link e não é seguido. |
| `ler` | `caminho`, `a_partir_da_linha`?, `linhas`? | Só texto UTF-8: NUL nos primeiros 8 KB = "arquivo binário, não leio". Padrão 200 linhas e 64 KB por chamada; devolve o número da próxima linha para continuar. |
| `buscar` | `caminho`, `nome`?, `conteudo`?, `incluir_ocultos`? | `nome`: trecho ou glob. `conteudo`: texto simples, sem caixa, em arquivos de texto até 2 MB. Profundidade 8, 100 resultados, 5 s. Pula `.git`, `node_modules`, `.venv`, `__pycache__` e caches, a menos que o caminho pedido esteja dentro deles. |

Cada resultado de leitura de arquivo sensível chega ao modelo normalmente; o efeito colateral é só D9.

## 5. Ferramenta `alterar_arquivos`

| Ação | Parâmetros | Cartão de aprovação |
|---|---|---|
| `criar` | `caminho`, `conteudo`, `sobrescrever`? | `criar /x/y.txt (120 bytes)`, ou `sobrescrever /x/y.txt (…)` |
| `editar` | `caminho`, `trecho_antigo`, `trecho_novo`, `todas`? | `editar /x/y.go: trocar 1 trecho (de "abc…" por "def…")` |
| `mover` | `origem`, `destino` | `mover /a → /b` |
| `copiar` | `origem`, `destino` | `copiar /a → /b` |
| `apagar` | `caminho` | `apagar /x/y.txt (vai para a lixeira)` |

- Os trechos do cartão são cortados em 60 caracteres e escapam quebra de linha.
- `editar` com zero ocorrências, ou mais de uma sem `todas`, devolve erro com a contagem e **não pede aprovação** (nada a aprovar).
- Pasta de destino inexistente é erro (não cria pastas em silêncio). `criar` aceita uma pasta pai que não existe só se o modelo pedir `criar_pastas: true`.

## 6. Trava de segredo na memória

- `SegredosLidos` guarda os valores com a hora de entrada, até um teto de 500 valores, e esquece o que passou de 1 hora.
- `salvar_na_memoria` recusa com a mensagem "essa nota contém um valor que veio de um arquivo sensível; guarde só a conclusão, sem o valor". O texto também é recusado se casar com um padrão de credencial (chave da Anthropic, do Google, token do GitHub, PEM, URL com senha) mesmo sem leitura prévia. Os padrões já existem em `internal/higiene`, mas **não são exportados**: a implementação expõe `higiene.ParecaCredencial(texto string) bool`, que usa a mesma lista da varredura do repositório (uma lista só, sem cópia).
- É melhor-esforço: o modelo pode reescrever um segredo com outras palavras, e a trava não pega isso. O prompt continua pedindo "uma frase, suas palavras".

## 7. Prompt (`prompts/sistema.md`)

- Regra 1 ("AUTONOMIA TOTAL") perde "criar, escrever": passa a dizer que criar, editar, mover, copiar e apagar **arquivos** pedem aprovação e vão por `alterar_arquivos`.
- Nova regra: olhar arquivos, pastas e conteúdo pelo `arquivos`; não usar `cat`, `ls`, `find` por shell para ler.
- Regra de memória: acrescentar `arquivos` à lista de ferramentas cuja saída não vai para a memória, citando `.env` e `.ssh`.
- O arquivo tem CRLF: editar preservando.

## 8. Critérios de aceite

| # | Critério | Como verificar |
|---|---|---|
| CA1 | Suítes verdes: `go vet ./...`, `GOOS=windows go vet ./...`, `GOOS=windows go build ./...`, `go test ./...`, pytest, vitest | saída dos comandos |
| CA2 | `listar`, `ler` e `buscar` funcionam em home e `/etc`, respeitam os tetos, recusam binário e caminho relativo | `go test` com pasta temporária |
| CA3 | Ler `.env` e `.ssh/id_*` não pede aprovação no desktop e **pede no celular** | `go test` |
| CA4 | **Trava de segredo**: depois de ler um `.env` com `CHAVE=abcdef123456`, `salvar_na_memoria` recusa a nota que contém `abcdef123456`, e aceita a que diz só "o projeto usa uma chave de API" | `go test` |
| CA5 | `criar`, `editar`, `mover`, `copiar`, `apagar` **sempre** pedem aprovação, com a descrição certa no cartão; negar não altera nada | `go test` + sessão ao vivo |
| CA6 | `editar` com 0 ou 2+ ocorrências devolve erro sem pedir aprovação; `criar`/`mover`/`copiar` não sobrescrevem sem pedido explícito | `go test` |
| CA7 | `apagar` vai para a lixeira (`gio trash`) e a recusa de `/`, home e pastas de sistema vale mesmo com aprovação | `go test` (executor falso) + sessão ao vivo |
| CA8 | Shell: `touch`, `mkdir`, `cp`, `mv`, `tee`, `ln`, `sed -i`, `echo x > f`, `cat >> f` pedem aprovação; `ls`, `cat f`, `sed s/a/b/ f`, `cmd 2>&1`, `cmd > /dev/null` não | testes da denylist (tabela) |
| CA9 | Escrita atômica e modo preservado | `go test` |
| CA10 | **O pedido do Arthur, por voz:** "me mostra o que tem na minha pasta de projetos", "lê o `config.toml`", "cria um arquivo de teste na pasta X" (pede aprovação), "apaga o arquivo de teste" (pede, e vai para a lixeira) | sessão ao vivo |
| CA11 | Windows compila e testa; as ações de escrita respondem "indisponível" quando a plataforma não suporta | `GOOS=windows go vet/build` |

## 9. Riscos

| # | Risco | Mitigação |
|---|---|---|
| R1 | Aprovação apressada apaga o que importa | lixeira (D11), piso de pastas protegidas, cartão com o caminho completo |
| R2 | A trava de segredo não pega valor reescrito | declarado em 6; o prompt segue pedindo conclusão com palavras próprias |
| R3 | Falso positivo da trava: palavra comum marcada como segredo | só valor de `CHAVE=valor` com 6+ caracteres e tokens de 16+ com letra e dígito |
| R4 | Falso positivo da denylist: `> /dev/null` ou `2>&1` pedindo aprovação | exceções explícitas, com teste |
| R5 | Resposta do celular com leitura de arquivo vaza segredo pela rede do Tailscale | D2: o celular pede aprovação a cada leitura |
| R6 | `buscar` por conteúdo lento em pasta enorme | teto de profundidade, de resultados e de 5 s, com aviso de corte |
| R7 | Turno de **voz** com aprovação: o Arthur precisa clicar na tela | o comportamento já existe para o terminal e para o clique destrutivo; o James avisa que precisa de aprovação na tela |

## 10. Decisões do Arthur (2026-10-14)

- Criar, editar e apagar pedem aprovação; navegar e ler são livres.
- Ler `.ssh` e `.env` sem perguntar, desde que o conteúdo não vá para a memória.
- Ferramenta própria **e** denylist do shell ampliada.
- Leitura em qualquer lugar que o usuário lê.
