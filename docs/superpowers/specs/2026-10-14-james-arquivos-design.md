# James: autonomia nos arquivos

Status: **rascunho para aprovação do Arthur** (2026-10-14, versão 2)
Branch: `feat/arquivos` (a partir de `main` em `67237dd`)
Origem: `docs/superpowers/decisoes/2026-10-13-james-autonomia-no-sistema.md`

## 0. O que mudou em relação à versão 1

A versão 1 (aprovada mais cedo, escrita com o modelo errado) foi refeita lendo o código de novo. Ela errou ou deixou de ver cinco coisas:

1. **"Não guardar na memória" não bastava.** O resultado de toda ferramenta vai para a Anthropic (é o modelo que lê), fica gravado no Postgres (tabela de mensagens, JSON dos `Resultados`), entra na janela de contexto e no resumo rolante da sessão, e é enviado ao app (`ferramenta_concluida`). Bloquear só o `salvar_na_memoria` deixava o valor de um `.env` gravado no banco e mandado para a nuvem. **Agora os valores secretos são mascarados antes de sair do jamesd** (D6), e mostrar o valor real é uma ação à parte, com aprovação (D7).
2. **O shell lia `.env` livre.** A lista de caminhos sensíveis do bash tem `.ssh/`, mas não tem `.env`: `cat .env` contornaria a máscara. Agora tem (D15).
3. **O detector de credencial do repositório não conhece a chave da Anthropic** (`sk-ant-`), a chave que o James usa. Uma chave commitada por engano passaria pela higiene. Entra (D17).
4. **Turno de voz com aprovação fica mudo.** A escrita sempre pede aprovação; por voz, o James esperava o clique por até 5 minutos sem dizer nada. Agora ele avisa (D14).
5. **Windows:** a versão 1 se contradizia sobre o que funciona lá. Agora as duas ferramentas novas não são registradas no Windows: o comportamento de lá não muda (D18).

Também ficaram mais precisos: link simbólico (a máscara e o piso olham o destino real), a corrida entre aprovar e executar (o `Executar` reconfere tudo), a lixeira que falha (nunca vira apagar de vez) e o compactador (`tar`, `unzip`, `gzip`) que cria ou substitui arquivo pelo shell.

## 1. Contexto

### 1.1 O que o Arthur pediu

"Quero que ele possa navegar entre os arquivos do meu sistema de forma mais autônoma; porém criar, apagar ou editar necessitam da minha aprovação." Depois: ler `.ssh` e `.env` sem perguntar "desde que não guarde na memória"; ferramenta própria **e** shell ampliado; leitura em qualquer lugar que o usuário lê.

### 1.2 Como é hoje (lido no código em 2026-10-14)

- **Ler** só por shell (`executar_comando_sistema`). A denylist do bash pede aprovação para ler `.ssh/`, `/etc/`, `.bashrc` e outros, mas **não** para `.env`.
- **Apagar** pelo shell pede aprovação (`rm`, `rmdir`, `shred`, `truncate`, `unlink`).
- **Criar e editar não pedem nada.** `touch`, `mkdir`, `cp`, `mv`, `ln`, `install`, `tee`, `rsync`, `patch`, `sed -i` rodam livres. O classificador usa `>`, `<`, `=` e aspas como separadores e apara o hífen do token, então `echo x > f` e `sed -i` não deixam nenhum token suspeito. O comentário do arquivo diz que o redirecionamento "não reprova".
- O resultado das ferramentas: vai ao modelo; fica no Postgres; entra na janela e no resumo; vai ao app. O terminal corta a saída em 64 KB (`max_bytes_saida`).
- O loop executa as chamadas de ferramenta **em sequência**.
- `ExigeConfirmacao` (decide pelos argumentos) e `DescreveConfirmacao` (texto do cartão) já existem.
- Perfil: o celular (`Restrito`) só usa `NivelSeguro` sem pedir; o desktop (`Privilegiado`) usa tudo. Aprovação aparece só no app do desktop; o timeout é 300 s.
- `prompts/sistema.md`, regra 1: "Instalar, criar, escrever, abrir, configurar, rodar build e teste: faça, não pergunte."

## 2. Escopo

### 2.1 Entra

1. **`arquivos`** (`listar`, `ler`, `buscar`): sem aprovação no desktop, com aprovação no celular. Valores secretos chegam **mascarados**.
2. **`revelar`** (ação de `arquivos`): mostra os valores reais de um arquivo, **sempre com aprovação**.
3. **`alterar_arquivos`** (`criar`, `editar`, `mover`, `copiar`, `apagar`): sempre com aprovação; o cartão mostra o que muda; `apagar` vai para a lixeira.
4. **Shell ampliado**: criar, copiar, mover, editar, compactar e redirecionar para arquivo pedem aprovação; ler `.env` e afins também.
5. **Trava na memória**: valor revelado e credencial reconhecível não entram no `salvar_na_memoria`.
6. **Aviso de voz** quando um turno falado espera aprovação.
7. **Prompt** atualizado.

### 2.2 Não entra

- `sudo`, arquivo de outro usuário, edição de binário.
- Windows (as ferramentas novas não são registradas lá; o classificador do PowerShell não muda).
- Programas que escrevem arquivo como efeito colateral (`git`, `go build`, `npm install`, `ffmpeg`, compiladores): seguem pelo shell sem aprovação. Limite honesto, como o de hoje para comando ofuscado.
- `chmod`/`chown` (já pedem pelo shell).
- Observar mudança de arquivo, indexar o disco, busca semântica.

## 3. Decisões

| # | Decisão | Por quê |
|---|---|---|
| D1 | Duas ferramentas: `arquivos` (olhar) e `alterar_arquivos` (mexer). | `Nivel()` é por ferramenta. Separar deixa regras diferentes sem condicional escondido. |
| D2 | `arquivos` é `NivelPerigoso`, sem `ExigeConfirmacao` para `listar`/`ler`/`buscar`. | Desktop roda livre; celular (`Restrito`) pede aprovação a cada leitura. |
| D3 | `alterar_arquivos` é `NivelPerigoso` e `ConfirmacaoNecessaria` sempre `true` (inclusive ação desconhecida). | Regra do Arthur, no desktop também. |
| D4 | Leitura alcança o que o usuário do jamesd lê. Sem `sudo`; erro de permissão é erro normal. | Decisão do Arthur. |
| D5 | Caminho absoluto ou com `~`. Relativo é erro ("diga o caminho completo"). | O jamesd não tem pasta atual visível ao Arthur. |
| D6 | **Máscara** em todo resultado de `ler` e `buscar`, antes de sair do jamesd: (a) **qualquer arquivo**: credencial de formato reconhecível (`higiene.ParecaCredencial`, D17) e bloco de chave privada PEM; (b) **arquivo sensível** (D8): o valor de toda linha `CHAVE=valor`/`chave: valor`, e a senha dentro de URL (`postgres://u:••••@host`). Formato: `CHAVE=•••• (oculto, 107 caracteres)`; o PEM vira `[chave privada omitida: N linhas]`. Nome da chave, comentário e estrutura continuam visíveis. | O valor não chega à Anthropic, ao Postgres, ao resumo, ao app nem à memória. O James ainda responde "o `.env` tem a chave do Claude preenchida?". |
| D7 | **`revelar`** (`arquivos`, `acao: revelar`): devolve o arquivo sem máscara. **Sempre** pede aprovação, com cartão "mostrar os valores secretos de ~/.env ao modelo (vão para a Anthropic e ficam no histórico desta conversa)". Os valores revelados entram no `SegredosLidos` (D11). | Quando o Arthur precisar do valor de verdade, a decisão é dele e informada. |
| D8 | **Arquivo sensível** = o caminho pedido **ou o destino real** (link simbólico resolvido) casa com: `.env` e `.env.*` (exceto `.env.example`, `.env.sample`, `.env.template`), `.ssh/`, `.aws/credentials`, `.gnupg/`, `.netrc`, `.git-credentials`, `.npmrc`, `.pypirc`, `.kube/config`, `.docker/config.json`, `*.pem`, `*.key`, `*.p12`, `*.pfx`, `id_rsa*`, `id_ed25519*`, `id_ecdsa*`, `/etc/shadow`, `/etc/gshadow`, `/etc/sudoers*`. | Link `~/atalho → ~/.ssh/id_rsa` não escapa da máscara. |
| D9 | `buscar` por **conteúdo** em arquivo sensível devolve só o caminho e o nome da chave que casou (`~/.env:3: CLAUDE_API_KEY=••••`), nunca o valor. | Sem isso, buscar seria um jeito de ler sem máscara. |
| D10 | `editar` troca um trecho **exato**, que tem de aparecer uma vez (ou `todas: true`). `criar`, `mover` e `copiar` **não sobrescrevem** (exceto `criar` com `sobrescrever: true`). | Evita reescrever o arquivo inteiro e o furo do `mv` de hoje. |
| D11 | **`SegredosLidos`**: guarda os valores **revelados** (D7) por 1 hora, teto de 500. `salvar_na_memoria` recusa nota que contenha um deles ou que case com `higiene.ParecaCredencial`. A mensagem de recusa não repete o valor. | A máscara já impede o caso comum; a trava cobre o revelado e a credencial ditada em conversa. |
| D12 | **Apagar = lixeira** (`gio trash -- caminho`, lista de argumentos, timeout 10 s). Se a lixeira falhar (sistema de arquivos sem lixeira, como `/tmp`), o erro volta ao modelo: **nunca** cai para apagar de vez. | Apagar recuperável, sempre. |
| D13 | **Piso**: `apagar` e `mover` recusam, mesmo aprovados, `/`, a home inteira e as pastas de sistema de topo (`/etc`, `/usr`, `/var`, `/boot`, `/bin`, `/sbin`, `/lib`, `/lib64`, `/dev`, `/proc`, `/sys`, `/opt`, `/srv`, `/root`), olhando o caminho pedido **e** o destino real. | Clique apressado não pode apagar a home. |
| D14 | **Aviso de voz**: num turno falado, quando o loop pede aprovação, o device fala "preciso da sua aprovação na tela" antes de esperar. | Sem isso o James fica mudo até 5 minutos. |
| D15 | **Shell**: pedem aprovação `touch`, `mkdir`, `cp`, `mv`, `ln`, `install`, `tee`, `rsync`, `patch`, `zip`, `unzip`, `gzip`, `gunzip`, `bzip2`, `xz`, `unxz`, `zstd`, `7z`, `split`; `tar` com criação ou extração; `sed`, `perl`, `awk` com edição no lugar; **redirecionamento** para arquivo (`>`, `>>`, `>|`, `&>`), exceto `/dev/null`, `/dev/stdout`, `/dev/stderr` e `&N`. Caminhos sensíveis do bash ganham `.env`, `.aws/`, `.gnupg/`, `.netrc`, `.git-credentials`, `.npmrc`, `.pypirc`, `id_rsa`, `id_ed25519`, `id_ecdsa`. | Fecha o buraco que a verificação achou, e o `cat .env` que contornaria a máscara. |
| D16 | **Escrita atômica** (temporário na mesma pasta + `rename`, modo preservado), teto de 1 MB por `criar`/`editar`. O `Executar` **reconfere** tudo o que foi aprovado (o trecho ainda existe uma vez, o destino ainda não existe, o piso). Cada alteração loga ação e caminho, **nunca** conteúdo. | Queda não deixa arquivo pela metade; o arquivo pode mudar entre o cartão e o clique. |
| D17 | `higiene.ParecaCredencial` exportada a partir da lista da varredura do repositório (uma lista só), e a lista ganha `sk-ant-` (Anthropic). | Mesma régua para o repositório, a máscara e a memória; e a chave que o James usa passa a ser reconhecida. |
| D18 | **Windows**: `arquivos` e `alterar_arquivos` só são registradas em unix (constante por build tag, como `TerminalSuportado`). | "Windows não pode quebrar": lá nada muda. |

## 4. `arquivos`

| Ação | Parâmetros | Regras |
|---|---|---|
| `listar` | `caminho`, `incluir_ocultos`? | Pastas primeiro, depois arquivos, por nome. Linha: tipo, tamanho, data, nome. Até 200 entradas, com "… e mais N". Link aparece como `link -> destino`, sem ser seguido. |
| `ler` | `caminho`, `a_partir_da_linha`?, `linhas`? | Só texto (NUL nos primeiros 8 KB = binário, não lê). Linhas numeradas; padrão 200 linhas e 32 KB por chamada, teto de 2000 linhas; linha acima de 8 KB é cortada com aviso; diz de onde continuar. Máscara D6. |
| `buscar` | `caminho`, `nome`?, `conteudo`?, `incluir_ocultos`? | `nome`: trecho sem caixa ou glob. `conteudo`: texto simples sem caixa, em texto até 2 MB, devolve `caminho:linha: trecho` (160 caracteres). Profundidade 8, 100 resultados, 5 s, com aviso de corte. Pula `.git`, `node_modules`, `.venv`, `__pycache__`, `.cache`, `vendor` quando o caminho pedido não está dentro deles. Não segue link de pasta. Máscara D6 e regra D9. |
| `revelar` | `caminho` | Igual a `ler`, sem máscara; aprovação sempre (D7). |

## 5. `alterar_arquivos`

| Ação | Parâmetros | Cartão de aprovação |
|---|---|---|
| `criar` | `caminho`, `conteudo`, `sobrescrever`?, `criar_pastas`? | `criar /x/y.txt (120 bytes)` / `sobrescrever /x/y.txt (120 bytes)` |
| `editar` | `caminho`, `trecho_antigo`, `trecho_novo`, `todas`? | `editar /x/y.go: trocar 1 trecho (de "abc…" por "def…")` |
| `mover` | `origem`, `destino` | `mover /a → /b` |
| `copiar` | `origem`, `destino` | `copiar /a → /b` |
| `apagar` | `caminho` | `apagar /x/y.txt (vai para a lixeira)` |

- Trechos do cartão: 60 caracteres, quebra de linha escapada. Em arquivo sensível, o cartão não mostra trecho: `editar ~/.env: trocar 1 trecho`.
- Pedido inválido de saída (editar com 0 ou 2+ ocorrências, destino que já existe, caminho relativo, piso) **não pede aprovação**: devolve o erro direto.
- `editar` em arquivo CRLF acha o trecho escrito com `\n` e mantém o CRLF.
- `copiar` pasta: até 5000 arquivos e 500 MB, senão erro antes de começar; link copiado como link.
- `mover` entre sistemas de arquivos: copia, confere o tamanho, só então remove a origem.
- O conteúdo de `criar` fica gravado na tabela de aprovações do Postgres (o cartão guarda os argumentos). É texto que o próprio Arthur aprovou; anotado.

## 6. Prompt (`prompts/sistema.md`, CRLF)

- Regra 1 ("AUTONOMIA TOTAL"): sai "criar, escrever"; entra que criar, editar, mover, copiar e apagar **arquivos** pedem aprovação e vão por `alterar_arquivos`.
- Nova regra: olhar pastas, arquivos e conteúdo pelo `arquivos`; não usar `cat`, `ls`, `find`, `grep` por shell para ler. Valor aparece como `••••`: é proposital; só use `revelar` quando o Arthur pedir o valor.
- Regra de memória: `arquivos` entra na lista de ferramentas cuja saída não vai para a memória.

## 7. Critérios de aceite

| # | Critério | Como verificar |
|---|---|---|
| CA1 | Suítes verdes: `go vet ./...`, `GOOS=windows go vet ./...`, `GOOS=windows go build ./...`, `go test ./...`, pytest, vitest | saída dos comandos |
| CA2 | `listar`, `ler`, `buscar` funcionam na home e em `/etc`, respeitam os tetos, recusam binário e caminho relativo | `go test` com pasta temporária |
| CA3 | **Máscara**: `ler` de um `.env` mostra os nomes das chaves e `••••` no lugar dos valores; `ler` de um arquivo comum com uma chave de formato reconhecível e um bloco PEM mascara os dois; um link para `.ssh/id_rsa` também é mascarado; `buscar` por conteúdo no `.env` não devolve valor | `go test` |
| CA4 | `revelar` sempre pede aprovação (no desktop também), com o cartão da D7 | `go test` |
| CA5 | Celular (`Restrito`) pede aprovação para `ler`; desktop não | `go test` |
| CA6 | **Memória**: depois de `revelar` um `.env` com `CHAVE=abcdef123456`, `salvar_na_memoria` recusa a nota com `abcdef123456` sem repetir o valor no erro; aceita "o projeto usa uma chave de API"; recusa credencial reconhecível mesmo sem leitura | `go test` |
| CA7 | `criar`, `editar`, `mover`, `copiar`, `apagar` sempre pedem aprovação, com a descrição certa; negar não altera nada | `go test` + sessão ao vivo |
| CA8 | Pedido inválido de saída não pede aprovação; `criar`/`mover`/`copiar` não sobrescrevem sem pedido; `Executar` reconfere (arquivo mudado depois do cartão → erro, nada escrito) | `go test` |
| CA9 | `apagar` vai para a lixeira; falha da lixeira vira erro e o arquivo continua lá; o piso vale mesmo aprovado, também por link | `go test` (executor falso) + sessão ao vivo |
| CA10 | Escrita atômica, modo e CRLF preservados; log sem conteúdo | `go test` |
| CA11 | **Shell**: a tabela da D15 pede aprovação; `ls`, `cat f`, `sed s/a/b/ f`, `tar -tf x.tar`, `cmd 2>&1`, `cmd > /dev/null`, `grep ">" f`, `awk '$1 > 3' f` não pedem; `cat .env` pede | testes da denylist (tabela) |
| CA12 | **Higiene** acusa uma chave `sk-ant-` montada em teste; a varredura do repositório continua verde | `go test ./internal/higiene/` |
| CA13 | **Voz**: turno falado que pede aprovação faz o device falar o aviso antes de esperar | `go test` em `internal/api` |
| CA14 | **Portão com o modelo real**: pedidos de listar, ler, buscar, criar e apagar viram `tool_use` na ferramenta certa, nenhum por `executar_comando_sistema` | teste de integração opcional (`JAMES_TESTE_ANTHROPIC=1`) |
| CA15 | **O pedido do Arthur, por voz**: "me mostra o que tem na minha pasta de projetos", "lê o config.toml", "o meu .env tem a chave do Claude?", "cria um arquivo de teste na pasta teste-james" (aviso de voz + cartão), "apaga o arquivo de teste" (lixeira) | sessão ao vivo |
| CA16 | Windows compila e testa, e as ferramentas novas não aparecem lá | `GOOS=windows go vet/build` + teste de registro |

## 8. Riscos

| # | Risco | Mitigação |
|---|---|---|
| R1 | A máscara não reconhece um segredo em formato desconhecido fora de arquivo sensível (ex.: senha num `notas.txt`) | declarado; a máscara cobre o caminho sensível inteiro e os formatos conhecidos. O `revelar` não é o único jeito de vazar, mas é o único **fácil** |
| R2 | Máscara atrapalha edição: o modelo não consegue `editar` um valor que só viu como `••••` | `revelar` (com aprovação) antes; ou o Arthur dita o valor novo |
| R3 | Aprovação apressada | lixeira (D12), piso (D13), cartão com caminho completo |
| R4 | Falso positivo da denylist (`> /dev/null`, `2>&1`, `grep ">"`) | exceções e varredura ciente de aspas, com teste |
| R5 | `buscar` lento em pasta enorme | tetos de profundidade, resultados e tempo, com aviso |
| R6 | Copiar `~/.env` para `x.txt` (aprovado) e ler `x.txt` sem máscara de caminho | a cópia pede aprovação e o cartão mostra a origem; a máscara (a) ainda pega formato conhecido |
| R7 | Aprovação no celular depende do app do desktop aberto | comportamento de hoje; timeout de 300 s vira "ação negada" |

## 9. Decisões do Arthur (2026-10-14)

- Criar, editar e apagar pedem aprovação; navegar e ler são livres.
- Ler `.ssh` e `.env` sem perguntar, desde que o conteúdo não vá para a memória. **Versão 2 propõe a máscara (D6) e o `revelar` (D7) como o jeito de cumprir isso: aguarda aprovação.**
- Ferramenta própria **e** shell ampliado.
- Leitura em qualquer lugar que o usuário lê.
