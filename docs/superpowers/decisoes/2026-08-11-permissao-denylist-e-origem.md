# Decisão — permissão vira denylist, e o CheckOrigin fecha antes

Data: 2026-08-11. Pedida pelo Arthur, executada na mesma sessão.
Registro de decisão, não spec de fase: o trabalho foi bugfix + troca de
política, não feature nova. Existe porque a escolha é **contraintuitiva de
fora** e uma sessão futura tem tudo pra "consertar" de volta sem saber o que
está desfazendo.

## O problema

O gate de aprovação pedia confirmação o tempo todo e por isso deixou de ser
sinal. A causa era a postura de `internal/ferramentas/classificador.go`:
allowlist de leitura. Só um punhado de verbos passava direto; qualquer coisa
fora dela ia pro humano. Na prática `Get-Process | Where-Object { ... }` já
reprovava — `{` estava na lista de tokens proibidos.

Card que aparece sempre não é lido. O Arthur pediu o oposto: James livre,
menos "abrir malware, excluir, ou alterar algo que eu não pedi".

## O que foi decidido

**D1. Denylist de destruição, não allowlist de leitura.** Confirma três
famílias: o que APAGA, o que ABRE A MÁQUINA (contas, permissões, registro,
firewall, antivírus, persistência) e o que EXECUTA CÓDIGO DE PROCEDÊNCIA
DESCONHECIDA (baixar-e-rodar, `Invoke-Expression`, payload codificado). Todo o
resto roda livre, inclusive escrita.

**D2. A varredura é por token, em qualquer posição.** A versão antiga olhava só
o primeiro verbo de cada estágio do pipeline, então `Get-Process; Remove-Item x`
escapava. Hoje todo token do comando é comparado.

**D3. `CheckOrigin` fechado ANTES de afrouxar o gate — não depois.** Era
`return true`. Como `permissao.ResolverPerfil` dá Privilegiado a loopback e
WebSocket não passa por CORS, qualquer página aberta no navegador desta
máquina abria conexão com o jamesd e falava como o Arthur. Enquanto o gate
pedia aprovação pra tudo, ele era o que segurava esse ataque. Afrouxar o gate
sem fechar a origem primeiro teria transformado "site que o Arthur visita" em
execução arbitrária. Ver `internal/api/origem.go`.

**D4. `perguntar_ao_claude_code` mantém confirmação sempre.** Única exceção à
D1, e por motivo estrutural, não por excesso de zelo: o argumento é instrução
em linguagem natural pra OUTRO agente, e nenhuma denylist classifica o que ele
vai decidir fazer com "arruma esse projeto aí". Liberar abriria exatamente o
"excluir sem eu ter pedido" que a D1 existe pra impedir, por um caminho onde a
D1 não alcança.

**D5. Aliases entram na lista conferidos, nunca deduzidos.** A primeira versão
da lista foi escrita de memória e tinha dois erros que a conferência com
`(Get-Alias <x>).Definition` pegou: faltava `ri` (é `Remove-Item` — apagava sem
confirmar) e sobrava `sc`, posto ali achando que era o `sc.exe` de serviço,
quando é apelido de **`Set-Content`** — bloqueava escrita comum, o oposto do
objetivo da mudança. Travado em
`TestAliasesResolvemParaAMesmaDecisaoQueONomeCompleto`, que testa os dois
sentidos: destrutivo que tem que pegar, e inofensivo que tem que passar.

**D6. Registro é gated pelo ALVO, não pelo verbo.** `Set-ItemProperty` e
`New-ItemProperty` não podem entrar inteiros na lista (servem pra atributo de
arquivo, uso corriqueiro), mas apontados pra uma hive viram persistência — a
chave Run é o caminho clássico. A regra é por substring de caminho (`hklm:`,
`hkcu:`, `hkey_`, ...). Reprova leitura de registro junto, de propósito: ler é
raro e custa um clique, deixar passar escrita na Run custa persistência.

**D7. O system prompt mudou junto.** Sem o gate pedindo tudo, o prompt vira o
freio principal, e ele dizia só "AUTONOMIA TOTAL: nunca pergunte". Ganhou duas
regras: autonomia é sobre COMO e nunca sobre O QUÊ (nada de apagar ou
sobrescrever o que não foi citado, nem "de passagem" pra limpar), e não rodar o
que não leu.

## Riscos aceitos — declarados, não esquecidos

| # | Risco | Por que foi aceito |
|---|---|---|
| A1 | `Set-Content`, `New-Item -Force` e `>` truncam arquivo existente sem confirmar | É o preço direto de "escreve livre". Barrar isso devolveria o card pra metade dos comandos |
| A2 | `Stop-Process`/`kill` passam livre | Reversível — relança o app. Perde trabalho não salvo, e isso é conhecido |
| A3 | `Move-Item`/`Rename-Item` passam livres, e `-Force` sobrescreve | Mover e renomear é trabalho corriqueiro demais pra gatear |
| A4 | A denylist não resiste a ofuscação deliberada (`&('Rem'+'ove-Item')`) | O modelo de ameaça é o James **errando a mão**, não driblando o dono. Pra erro honesto, lista de token é a ferramenta certa. As primitivas de ofuscação (crase, base64, `-EncodedCommand`) estão na lista pra fechar a evasão ACIDENTAL |
| A5 | `CheckOrigin` não protege contra processo local da mesma conta | A conta do SO é a fronteira, e ela já foi atravessada nesse cenário. Token em disco seria teatro: o mesmo processo lê o token |
| A6 | `Start-Process` de um `.exe` já em disco passa livre | O download que o colocaria lá é gated. A corrente quebra antes |

## Como verificar que ainda vale

```bash
go test ./internal/ferramentas/ ./internal/api/ -count=1
```

Os testes que **não podem** ser afrouxados sem uma decisão nova no lugar desta:

- `TestComandoPrecisaConfirmacao` — a política em si, nos dois sentidos
- `TestAliasesResolvemParaAMesmaDecisaoQueONomeCompleto` — D5
- `TestEscritaNoRegistroPedeConfirmacao` — D6
- `TestClaudeCodeSempreExigeConfirmacao` — D4
- `TestHandshakeDeOrigemEstranhaEhRecusadoDeVerdade` — D3, e prova a FIAÇÃO
  (a função podia estar certa e não estar ligada no upgrader)

## Não coberto

Race detector não funciona nesta máquina (TSan falha ao alocar ~72 MB;
reproduzido em pacote alheio ao diff). A concorrência nova de
`internal/api/dispositivo.go` está analisada **por inspeção**, não por
ferramenta.
