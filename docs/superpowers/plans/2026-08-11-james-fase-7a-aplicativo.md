# Fase 7A — o James vira aplicativo: plano de implementação

> **Para agentes executores:** SUB-SKILL OBRIGATÓRIA: use `superpowers:subagent-driven-development` (recomendado) ou `superpowers:executing-plans` para executar este plano task a task. Os passos usam checkbox (`- [ ]`) para rastreamento.

**Objetivo:** Trocar a bolinha flutuante por um aplicativo com barra lateral e cinco abas, com o orbe reagindo ao áudio real de saída do James, conversa por sessão, histórico paginado e resumo rolante da conversa.

**Arquitetura:** O `App.tsx` continua sendo dono do `ClienteWS`, do redutor de turno e do estado de conexão — muda só o layout em volta. A aba James absorve aprovação, atividade e leitura de tela; a aba Mensagens fica só com a conversa. No Go, `processarTurno` ganha uma etapa de resumo ancorada em `mensagens.id` do Postgres, e `GET /api/sessoes` ganha paginação keyset.

**Stack:** React 18 + TypeScript + Vite 8 + Tailwind 4.3.3 + lucide-react no front; Go 1.x + pgx v5 + Postgres no back; Wails v2 no shell nativo. Testes: vitest + @testing-library/react, `go test` contra Postgres real, Playwright para ponta a ponta.

**Spec:** [`../specs/2026-08-11-james-fase-7a-aplicativo-design.md`](../specs/2026-08-11-james-fase-7a-aplicativo-design.md)
**Design de referência:** [`../referencias/painel-de-controle-james/`](../referencias/painel-de-controle-james/)

---

## Antes de começar: leia isto

Cinco fatos que custaram tempo neste projeto e vão custar de novo se forem ignorados.

1. **`web/embed.go` usa `//go:embed all:dist`.** A SPA é compilada dentro do binário. Mudou o front? `npm run build --prefix web` **e reinicie o `jamesd`**, senão você testa a interface velha e conclui que sua mudança não funcionou.

2. **Input controlado do React não reage a `type`/`fill` de automação.** O `onChange` sintético não dispara, o estado não muda, e a mensagem parece não ser enviada. Use setter nativo mais `dispatchEvent(new Event('input', {bubbles:true}))`. Metade de uma nota de metodologia da Fase 5 era exatamente esse artefato, não bug de produto.

3. **A suíte Go escreve no banco de trabalho e quase nunca limpa.** Já havia 1646 sessões lá. Isso não é bug desta fase — é cenário de teste da Task 13.

4. **Os testes deste projeto importam tudo explicitamente de `vitest`** (sem `globals`) e usam `toBeTruthy()`, não `toBeInTheDocument()` — não há `jest-dom` instalado. `web/vitest.setup.ts` registra o `cleanup()` à mão.

5. **Relato errado é mais caro que código errado.** Na Fase 6 um implementador afirmou ter escrito cobertura que não existia no arquivo. Se você não rodou, diga que não rodou.

**Rodar a suíte:**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && go vet ./... && go test ./...
```

```bash
npm test --prefix web
```

---

## Estrutura de arquivos

**Nascem:**

| Arquivo | Responsabilidade |
|---|---|
| `web/src/shell/barraLateral.tsx` | Navegação: cinco itens, colapso, contador de aprovação |
| `web/src/shell/aplicativo.tsx` | Composição do layout: barra lateral + painel ativo |
| `web/src/shell/controlesJanela.tsx` | Os três círculos; só aparecem sob o shell Wails |
| `web/src/shell/orbe.tsx` *(reescrito)* | Canvas que deforma conforme o `AnalyserNode` recebido |
| `web/src/abas/james.tsx` | Orbe + aprovação + atividade + leitura de tela, com precedência |
| `web/src/abas/mensagens.tsx` | Conversa da sessão corrente |
| `web/src/abas/historico.tsx` | Lista paginada de sessões, agrupada por dia |
| `web/src/abas/dashboard.tsx` | Estado vazio franco até a 7B |
| `web/src/lib/audio.ts` | `AudioContext` único e criação do par `Audio` + `AnalyserNode` |
| `web/src/lib/sessao.ts` | Leitura e escrita do id de sessão em `sessionStorage` |
| `internal/armazenamento/migracoes/0002_resumo_sessao.sql` | `resumo` e `resumido_ate` |
| `internal/memoria/resumo.go` | O resumidor |
| `web/e2e/james.spec.ts` | Ponta a ponta com Playwright |

**Morrem:** `web/src/trilha.tsx`, `web/src/visualizador.tsx`, `web/src/shell/orbe.css`, `web/src/abas/conversa.tsx`, `web/src/abas/atividade.tsx`, `web/src/abas/tela.tsx`, `web/src/abas/sessoes.tsx` — e os testes de cada um.

> `abas/tela.tsx` exporta `interpretarSaidaVerTela`, que **continua vivo** — migra para `web/src/lib/verTela.ts` na Task 3, com os testes junto. Não apague essa função.

**Modificados:** `web/src/App.tsx`, `web/src/style.css`, `web/src/lib/janela.ts`, `web/src/lib/api.ts`, `web/package.json`, `shell/main.go`, `shell/controle.go`, `internal/api/ws.go`, `internal/api/rest.go`, `internal/armazenamento/postgres/sessao.go`, `internal/armazenamento/postgres/mensagem.go`, `internal/config/config.go`, `config.toml`.

**Ordem:** a §13 da spec listava a morte do `visualizador.tsx` antes de a aba James existir, o que deixaria a aprovação sem interface no meio da execução. Este plano inverte: a aba James nasce primeiro (Task 3), e a limpeza vem depois (Task 5).

---

## Task 1: Tokens de cor, fontes e lucide-react

**Arquivos:**
- Modificar: `web/src/style.css`
- Modificar: `web/package.json`
- Criar: `web/src/assets/fonts/geist-latin.woff2`, `web/src/assets/fonts/source-serif-4-latin.woff2`

- [ ] **Passo 1: Instalar o lucide-react**

```bash
npm install lucide-react --prefix web
```

- [ ] **Passo 2: Baixar as fontes**

As fontes precisam ser locais — o binário roda offline e o `//go:embed` não busca recurso externo (D16).

**São dois arquivos, não três.** As duas famílias são fontes variáveis no Google Fonts: um único `.woff2` cobre todo o eixo de peso. Conferido resolvendo o CSS do Google Fonts em 2026-08-11 — Geist 400 e Geist 600 devolvem exatamente a mesma URL. As URLs abaixo são as do subconjunto **latin**, já resolvidas (o CSS devolve cinco arquivos por família, um por subconjunto; pegar o errado dá acentuação quebrada em português).

```bash
curl -sS -o "web/src/assets/fonts/geist-latin.woff2" "https://fonts.gstatic.com/s/geist/v5/gyByhwUxId8gMEwcGFU.woff2"
```

```bash
curl -sS -o "web/src/assets/fonts/source-serif-4-latin.woff2" "https://fonts.gstatic.com/s/sourceserif4/v14/vEFF2_tTDB4M7-auWDN0ahZJW3IX2ih5nk3AucvUHf6kDXr4.woff2"
```

> **Cuidado que já mordeu uma vez.** A primeira versão deste plano trazia outra URL para o Source Serif 4 (`vEFy2_…OAVIJ…`, 20.088 bytes), resolvida pedindo `wght@400` ao Google Fonts. Peso **único** faz o Google servir um **corte estático**, não a fonte variável — e aí o `font-weight: 200 900` do `@font-face` mente, e o navegador sintetiza o negrito esticando o traço. A URL acima veio de pedir a **faixa** `wght@200..900`, e o arquivo tem 50.824 bytes. Se precisar reresolver, peça sempre faixa. Confira o tamanho: ~20 KB é o corte estático errado, ~50 KB é o variável certo.

Confira que os dois existem e não estão vazios:

```bash
ls -la "web/src/assets/fonts/"
```

Esperado: os dois `.woff2` novos com tamanho na casa das dezenas de KB, além do `nunito-v16-latin-regular.woff2` que já estava lá. Arquivo de 0 byte ou de poucas centenas de bytes significa que veio uma página de erro em vez da fonte — nesse caso o `curl` falhou e o download precisa ser refeito.

Se não houver rede, **não invente um caminho alternativo**: reporte `DONE_WITH_CONCERNS`, deixe o `@font-face` no CSS mesmo assim (a pilha de fallback do `--font-sans` mantém o app legível) e diga que as fontes faltam.

- [ ] **Passo 3: Reescrever o `style.css`**

Substitua **todo** o conteúdo de `web/src/style.css` por:

```css
@import "tailwindcss";

/* Tokens do design de referência (docs/superpowers/referencias/). Tailwind 4
   expõe cada `--color-*` declarado aqui como classe utilitária: --color-app-bg
   vira bg-app-bg, text-app-bg, border-app-bg. */
@theme {
    --color-app-bg: oklch(0.09 0.004 265);
    --color-app-panel: oklch(0.05 0.004 265);
    --color-app-sidebar: oklch(0.13 0.006 265);
    --color-app-blue: oklch(0.52 0.24 267);
    --color-app-blue-foreground: oklch(0.98 0 0);
    --color-app-icon: oklch(0.58 0.22 266);
    --color-app-hover: oklch(0.82 0.01 265 / 0.16);
    --color-app-line: oklch(1 0 0 / 0.08);
    --color-app-text: oklch(0.96 0 0);
    --color-app-muted: oklch(0.68 0.01 265);

    --font-sans: "Geist", -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    --font-serif: "Source Serif 4", Georgia, "Times New Roman", serif;
}

/* Fontes variáveis: um arquivo cobre o eixo de peso inteiro, e por isso
   font-weight declara FAIXA, não valor. Declarar `font-weight: 400` aqui
   faria o navegador sintetizar o negrito (esticar o traço) em vez de usar o
   peso real que já está no arquivo. */
@font-face {
    font-family: "Geist";
    font-style: normal;
    font-weight: 100 900;
    font-display: swap;
    src: url("assets/fonts/geist-latin.woff2") format("woff2");
}

@font-face {
    font-family: "Source Serif 4";
    font-style: normal;
    font-weight: 200 900;
    font-display: swap;
    src: url("assets/fonts/source-serif-4-latin.woff2") format("woff2");
}

/* Tema escuro fixo (§9 da spec). O `text-align: center` do CSS antigo saiu:
   ele vinha do template do Wails e centralizava texto de balão de conversa. */
html {
    background-color: var(--color-app-bg);
    color: var(--color-app-text);
    color-scheme: dark;
}

body {
    margin: 0;
    font-family: var(--font-sans);
    -webkit-font-smoothing: antialiased;
}

#app {
    height: 100vh;
}
```

- [ ] **Passo 4: Verificar que o build passa**

```bash
npm run build --prefix web
```

Esperado: build concluído sem erro. Se o `tsc` reclamar de algum arquivo que usava classe removida, é falso positivo — `tsc` não lê CSS; nesse caso o erro é outro e precisa ser lido.

- [ ] **Passo 5: Commit**

```bash
git add web/src/style.css web/package.json web/package-lock.json web/src/assets/fonts
git commit -m "feat(web): tokens de cor, fontes locais e lucide-react da Fase 7A"
```

---

## Task 2: Barra lateral

**Arquivos:**
- Criar: `web/src/shell/barraLateral.tsx`
- Teste: `web/src/shell/barraLateral.test.tsx`

- [ ] **Passo 1: Escrever o teste que falha**

Crie `web/src/shell/barraLateral.test.tsx`:

```tsx
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { BarraLateral } from "./barraLateral";

describe("Barra lateral", () => {
  it("mostra as cinco abas", () => {
    render(<BarraLateral ativa="james" aoSelecionar={vi.fn()} aprovacoesPendentes={0} />);

    for (const rotulo of ["James", "Mensagens", "Memória", "Histórico", "DashBoard"]) {
      expect(screen.getByRole("button", { name: new RegExp(rotulo) })).toBeTruthy();
    }
  });

  it("marca a aba ativa com aria-current", () => {
    render(<BarraLateral ativa="mensagens" aoSelecionar={vi.fn()} aprovacoesPendentes={0} />);

    const botao = screen.getByRole("button", { name: /Mensagens/ });
    expect(botao.getAttribute("aria-current")).toBe("page");
  });

  it("avisa quem selecionou", () => {
    const aoSelecionar = vi.fn();
    render(<BarraLateral ativa="james" aoSelecionar={aoSelecionar} aprovacoesPendentes={0} />);

    fireEvent.click(screen.getByRole("button", { name: /Histórico/ }));

    expect(aoSelecionar).toHaveBeenCalledWith("historico");
  });

  it("colapsa e volta", () => {
    render(<BarraLateral ativa="james" aoSelecionar={vi.fn()} aprovacoesPendentes={0} />);

    // Expandida: o rótulo textual está no DOM.
    expect(screen.getByText("Mensagens")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Recolher menu" }));

    // Recolhida: sobra só o ícone, e o nome vive no aria-label do botão.
    expect(screen.queryByText("Mensagens")).toBeNull();
    expect(screen.getByRole("button", { name: /Mensagens/ })).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Expandir menu" }));

    expect(screen.getByText("Mensagens")).toBeTruthy();
  });

  // O contador existe porque, estando em outra aba, você não veria o James
  // travado esperando um sim — e o timeout de 300s corre igual (§4.2).
  //
  // A asserção é sobre o NOME ACESSÍVEL DO BOTÃO, e isso não é detalhe. A
  // primeira versão deste plano checava `getByLabelText("1 aprovação
  // pendente")`, mirando um aria-label no <span> do contador. Passava, e
  // mentia: `aria-label` no botão SUBSTITUI todo o conteúdo descendente no
  // cálculo do nome acessível, então esse rótulo interno nunca é anunciado.
  // O leitor de tela dizia só "James", e o contador — cuja única razão de
  // existir é avisar quem está noutra aba — não avisava ninguém.
  // `getByLabelText` varre atributos soltos na árvore; não é o que o usuário
  // ouve ao focar o botão.
  it("anuncia a aprovação pendente no nome do botão da aba James", () => {
    render(<BarraLateral ativa="mensagens" aoSelecionar={vi.fn()} aprovacoesPendentes={1} />);

    expect(screen.getByRole("button", { name: "James, 1 aprovação pendente" })).toBeTruthy();
  });

  it("pluraliza o contador", () => {
    render(<BarraLateral ativa="mensagens" aoSelecionar={vi.fn()} aprovacoesPendentes={2} />);

    expect(screen.getByRole("button", { name: "James, 2 aprovações pendentes" })).toBeTruthy();
  });

  it("sem aprovação, o nome do botão é só o rótulo", () => {
    render(<BarraLateral ativa="mensagens" aoSelecionar={vi.fn()} aprovacoesPendentes={0} />);

    expect(screen.getByRole("button", { name: "James" })).toBeTruthy();
  });

  it("o contador não aparece em aba que não é a James", () => {
    render(<BarraLateral ativa="james" aoSelecionar={vi.fn()} aprovacoesPendentes={3} />);

    expect(screen.getByRole("button", { name: "Mensagens" })).toBeTruthy();
  });
});
```

- [ ] **Passo 2: Rodar o teste e ver falhar**

```bash
npm test --prefix web -- barraLateral
```

Esperado: FALHA com `Failed to resolve import "./barraLateral"`.

- [ ] **Passo 3: Escrever o componente**

Crie `web/src/shell/barraLateral.tsx`:

```tsx
import { useState } from "react";
import {
  Gem,
  Hourglass,
  LayoutDashboard,
  MessageCircle,
  PanelLeft,
  Sparkle,
  type LucideIcon,
} from "lucide-react";

export type IdAba = "james" | "mensagens" | "memoria" | "historico" | "dashboard";

interface ItemNav {
  id: IdAba;
  rotulo: string;
  Icone: LucideIcon;
}

const ITENS: ItemNav[] = [
  { id: "james", rotulo: "James", Icone: Sparkle },
  { id: "mensagens", rotulo: "Mensagens", Icone: MessageCircle },
  { id: "memoria", rotulo: "Memória", Icone: Gem },
  { id: "historico", rotulo: "Histórico", Icone: Hourglass },
  { id: "dashboard", rotulo: "DashBoard", Icone: LayoutDashboard },
];

interface Props {
  ativa: IdAba;
  aoSelecionar: (id: IdAba) => void;
  aprovacoesPendentes: number;
}

// nomeAcessivel é o que o leitor de tela anuncia ao focar o botão. Hoje o
// estado do turno guarda no máximo uma aprovação por vez, mas a prop é um
// número livre — pluralizar custa uma linha e evita "2 aprovação pendente"
// se isso mudar.
function nomeAcessivel(rotulo: string, pendentes: number): string {
  if (pendentes <= 0) return rotulo;
  const plural = pendentes === 1 ? "aprovação pendente" : "aprovações pendentes";
  return `${rotulo}, ${pendentes} ${plural}`;
}

export function BarraLateral({ ativa, aoSelecionar, aprovacoesPendentes }: Props) {
  const [expandida, setExpandida] = useState(true);

  return (
    <nav
      aria-label="Navegação principal"
      className={`flex h-full shrink-0 flex-col border-r border-app-line bg-app-sidebar transition-[width] duration-300 ease-out ${
        expandida ? "w-52" : "w-16"
      }`}
    >
      {/* pt-11 abre espaço para os controles de janela, que são posicionados
          por cima desta área pelo aplicativo.tsx */}
      <div
        className={`flex px-3 pt-11 pb-4 ${
          expandida ? "items-center justify-between gap-2" : "flex-col items-center gap-3"
        }`}
      >
        {expandida && <span className="font-serif text-sm text-app-icon">James</span>}
        <button
          type="button"
          onClick={() => setExpandida((v) => !v)}
          aria-label={expandida ? "Recolher menu" : "Expandir menu"}
          className="grid size-8 place-items-center rounded-md text-app-muted transition-colors hover:bg-app-hover hover:text-app-text"
        >
          <PanelLeft className="size-4" />
        </button>
      </div>

      <ul className="flex flex-1 flex-col gap-1 px-2">
        {ITENS.map(({ id, rotulo, Icone }) => {
          const estaAtiva = id === ativa;
          const pendentes = id === "james" ? aprovacoesPendentes : 0;
          return (
            <li key={id}>
              <button
                type="button"
                onClick={() => aoSelecionar(id)}
                aria-current={estaAtiva ? "page" : undefined}
                // A contagem entra AQUI, no nome do próprio botão, e não num
                // aria-label do badge: aria-label no botão substitui todo o
                // conteúdo descendente no cálculo do nome acessível, então
                // rótulo posto no <span> de dentro nunca seria anunciado.
                aria-label={nomeAcessivel(rotulo, pendentes)}
                // title é a pista para quem ENXERGA e não reconhece o ícone
                // com a barra recolhida — ali o rótulo textual não existe.
                // Não interfere no nome acessível, que o aria-label já fixa.
                title={rotulo}
                className={`flex w-full items-center rounded-lg text-left transition-colors ${
                  expandida ? "gap-3 px-3 py-2" : "justify-center px-0 py-2"
                } ${estaAtiva ? "bg-app-blue text-app-blue-foreground" : "text-app-icon hover:bg-app-hover"}`}
              >
                <span className="relative grid size-5 shrink-0 place-items-center">
                  <Icone className="size-5" />
                  {pendentes > 0 && (
                    // Puramente decorativo: o texto equivalente já está no
                    // nome do botão.
                    <span
                      aria-hidden="true"
                      className="absolute -right-1.5 -top-1.5 grid size-4 place-items-center rounded-full bg-amber-500 text-[10px] font-semibold text-black"
                    >
                      {pendentes}
                    </span>
                  )}
                </span>
                {expandida && <span className="font-serif text-sm">{rotulo}</span>}
              </button>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
```

- [ ] **Passo 4: Rodar o teste e ver passar**

```bash
npm test --prefix web -- barraLateral
```

Esperado: 6 testes passando.

- [ ] **Passo 5: Commit**

```bash
git add web/src/shell/barraLateral.tsx web/src/shell/barraLateral.test.tsx
git commit -m "feat(web): barra lateral com as cinco abas e contador de aprovacao"
```

---

## Task 3: Aba James (sem o orbe ainda)

A aba absorve aprovação, atividade e leitura de tela, com a precedência da §5 da spec. O orbe entra na Task 8 — aqui a área dele fica reservada.

**Arquivos:**
- Criar: `web/src/lib/verTela.ts` (migração de `interpretarSaidaVerTela`)
- Criar: `web/src/lib/verTela.test.ts`
- Criar: `web/src/abas/james.tsx`
- Teste: `web/src/abas/james.test.tsx`

- [ ] **Passo 1: Migrar `interpretarSaidaVerTela` para `lib/`**

A função é lógica pura de parsing e sobrevive à morte de `abas/tela.tsx`. Crie `web/src/lib/verTela.ts` com **exatamente** o conteúdo abaixo (copiado de `web/src/abas/tela.tsx:1-33`, incluindo os comentários — eles registram que o formato usa espaço duplo, conferido contra o Python real):

```ts
export interface LeituraTela {
  superficie: string;
  elementos: string[];
}

// interpretarSaidaVerTela lê o texto que a ferramenta ver_tela devolve:
// spec da Fase 5, seção 5.1, e a implementação real em
// dispositivo/james_device/tela.py:_ver_tela / elementos.py:formatar.
//
// A primeira linha é o cabeçalho "superficie: <ID>[ (primaria)] <LxA>
// escala <N>%"; cada linha seguinte é um elemento, formatado como
// `<id>  <tipo>  "<nome>"  (<x>,<y>)` (dois espaços entre campos — conferido
// contra elementos.py:formatar, não presumido). O parser não quebra a linha
// de elemento em campos: cada uma vira uma string opaca, então o
// espaçamento exato do device não importa aqui, só o corte entre cabeçalho
// e o resto.
//
// Quando a varredura estoura o teto de 80 elementos, elementos.py anexa uma
// última linha de aviso ("... e mais N elementos ..."). Ela não começa com
// id, mas entra na lista de elementos igual às outras — é a forma mais
// simples de fazer o aviso chegar até a tela sem inventar um segundo canal.
export function interpretarSaidaVerTela(saida: string): LeituraTela {
  const linhas = saida.split("\n").map((l) => l.trim()).filter(Boolean);
  const primeira = linhas[0] ?? "";

  if (primeira.startsWith("superficie:")) {
    return {
      superficie: primeira.slice("superficie:".length).trim(),
      elementos: linhas.slice(1),
    };
  }
  return { superficie: "", elementos: linhas };
}
```

Copie também os casos de `interpretarSaidaVerTela` que existem em `web/src/abas/tela.test.tsx` para um `web/src/lib/verTela.test.ts` novo, ajustando o import para `./verTela`. **Não invente casos novos** — os que estão lá cobrem o formato real do device.

- [ ] **Passo 2: Rodar o teste migrado e ver passar**

```bash
npm test --prefix web -- verTela
```

Esperado: PASSA — é a mesma função, só mudou de arquivo.

- [ ] **Passo 3: Escrever o teste que falha da aba James**

Crie `web/src/abas/james.test.tsx`:

```tsx
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AbaJames } from "./james";
import { estadoInicial, type EstadoTurno } from "../estado/turno";

const APROVACAO = {
  id: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
  ferramenta: "terminal",
  argumentos: { comando: "Remove-Item x" },
};

function comEstado(parcial: Partial<EstadoTurno>): EstadoTurno {
  return { ...estadoInicial, ...parcial };
}

describe("Aba James", () => {
  it("em repouso, não mostra ferramenta nem aprovação", () => {
    render(<AbaJames estado={estadoInicial} saidaVerTela={null} aoDecidir={vi.fn()} analisador={null} />);

    expect(screen.queryByText(/quer executar/)).toBeNull();
    expect(screen.getByText(/Em repouso/)).toBeTruthy();
  });

  it("aprovação pendente vence ferramenta em curso", () => {
    const estado = comEstado({
      aprovacao: APROVACAO,
      ferramentas: [{ nome: "ver_tela", estado: "voando", iniciadaEm: Date.now() }],
    });

    render(<AbaJames estado={estado} saidaVerTela="superficie: DISPLAY1" aoDecidir={vi.fn()} analisador={null} />);

    expect(screen.getByText("terminal")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Aprovar" })).toBeTruthy();
  });

  it("aprovar e negar avisam quem decidiu", () => {
    const aoDecidir = vi.fn();
    render(
      <AbaJames estado={comEstado({ aprovacao: APROVACAO })} saidaVerTela={null} aoDecidir={aoDecidir} analisador={null} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Aprovar" }));
    expect(aoDecidir).toHaveBeenCalledWith(true);

    fireEvent.click(screen.getByRole("button", { name: "Negar" }));
    expect(aoDecidir).toHaveBeenCalledWith(false);
  });

  // CA5: iniciadaEm já existia no redutor desde a Fase 6 e ninguém consumia.
  it("mostra a duração de uma ferramenta concluída", () => {
    vi.spyOn(Date, "now").mockReturnValue(10_000);
    const estado = comEstado({
      ferramentas: [{ nome: "buscar_na_web", estado: "concluida", iniciadaEm: 7_500 }],
    });

    render(<AbaJames estado={estado} saidaVerTela={null} aoDecidir={vi.fn()} analisador={null} />);

    expect(screen.getByText("2,5 s")).toBeTruthy();
    vi.mocked(Date.now).mockRestore();
  });

  it("mostra a leitura de tela quando ver_tela é a ferramenta mais recente", () => {
    const estado = comEstado({
      ferramentas: [{ nome: "ver_tela", estado: "concluida", iniciadaEm: Date.now() }],
    });

    render(
      <AbaJames
        estado={estado}
        saidaVerTela={'superficie: DISPLAY1 (primaria) 1920x1080 escala 100%\n1  ButtonControl  "OK"  (10,20)'}
        aoDecidir={vi.fn()}
        analisador={null}
      />,
    );

    expect(screen.getByText(/DISPLAY1/)).toBeTruthy();
    expect(screen.getByText(/ButtonControl/)).toBeTruthy();
  });

  // Precedência por recência: sem isto, saidaVerTela (que guarda a última
  // captura da SESSÃO, não do turno) congelaria o painel na primeira captura.
  it("não mostra a tela quando a ferramenta mais recente é outra", () => {
    const estado = comEstado({
      ferramentas: [
        { nome: "ver_tela", estado: "concluida", iniciadaEm: 1 },
        { nome: "terminal", estado: "voando", iniciadaEm: 2 },
      ],
    });

    render(<AbaJames estado={estado} saidaVerTela="superficie: DISPLAY1" aoDecidir={vi.fn()} analisador={null} />);

    expect(screen.queryByText(/DISPLAY1/)).toBeNull();
  });

  // App.tsx só atualiza saidaVerTela quando dados.saida é não-vazio
  // (ferramenta_concluida: `if (dados.nome === "ver_tela" && dados.saida)
  // setSaidaVerTela(...)`) — um ver_tela que FALHA chega com saida="" e o
  // estado antigo fica parado. Sem checar ultima.estado aqui, essa captura
  // velha passaria por atual e o erro do turno ficaria invisível: a tela
  // mostra dado desatualizado como se fosse fresco, e o card "✗ + f.erro"
  // (que só existe na view "ferramenta") nunca aparece.
  it("mostra o erro em vez da captura velha quando ver_tela falha", () => {
    const estado = comEstado({
      ferramentas: [{ nome: "ver_tela", estado: "erro", erro: "device desconectado", iniciadaEm: Date.now() }],
    });

    render(
      <AbaJames
        estado={estado}
        saidaVerTela="superficie: DISPLAY1 (captura de um turno anterior)"
        aoDecidir={vi.fn()}
        analisador={null}
      />,
    );

    expect(screen.queryByText(/DISPLAY1/)).toBeNull();
    expect(screen.getByText("device desconectado")).toBeTruthy();
  });
});
```

- [ ] **Passo 4: Rodar e ver falhar**

```bash
npm test --prefix web -- abas/james
```

Esperado: FALHA com `Failed to resolve import "./james"`.

- [ ] **Passo 5: Escrever a aba**

Crie `web/src/abas/james.tsx`:

```tsx
import type { EstadoTurno, FerramentaNoTurno } from "../estado/turno";
import { interpretarSaidaVerTela } from "../lib/verTela";
import { Orbe } from "../shell/orbe";

interface Props {
  estado: EstadoTurno;
  saidaVerTela: string | null;
  aoDecidir: (aprovado: boolean) => void;
  // O analisador vem de fora para o componente ser testável sem áudio real
  // (§5.1 da spec). Null = orbe em respiração de repouso.
  analisador: AnalyserNode | null;
}

export type ConteudoPainel = "aprovacao" | "ferramenta" | "tela" | "repouso";

// Precedência: aprovação pendente vence tudo, porque é a única coisa que
// bloqueia o turno esperando o humano. Depois vale a recência — a leitura de
// tela só aparece enquanto ver_tela for a ferramenta mais recente.
//
// ultima.estado !== "erro" importa: App.tsx só atualiza saidaVerTela quando
// o evento ferramenta_concluida chega com saida não-vazia. Um ver_tela que
// FALHA manda saida="" e o estado antigo (de um turno anterior, às vezes)
// fica parado. Sem esta checagem, a captura velha passaria por atual e o
// erro do turno ficaria invisível — a view "ferramenta", que é a única que
// mostra f.erro, nunca seria escolhida.
export function escolherConteudo(estado: EstadoTurno, saidaVerTela: string | null): ConteudoPainel {
  if (estado.aprovacao) return "aprovacao";

  const ultima = estado.ferramentas[estado.ferramentas.length - 1];
  if (!ultima) return "repouso";
  if (ultima.nome === "ver_tela" && ultima.estado !== "erro" && saidaVerTela) return "tela";
  return "ferramenta";
}

// duracaoLegivel formata o tempo desde iniciadaEm. Ferramenta ainda voando
// mostra o tempo corrido até agora; concluída mostra o total.
function duracaoLegivel(f: FerramentaNoTurno): string {
  const segundos = (Date.now() - f.iniciadaEm) / 1000;
  return `${segundos.toFixed(1).replace(".", ",")} s`;
}

export function AbaJames({ estado, saidaVerTela, aoDecidir, analisador }: Props) {
  const conteudo = escolherConteudo(estado, saidaVerTela);

  return (
    <div className="flex h-full flex-col items-center gap-6 overflow-y-auto p-8">
      <Orbe analisador={analisador} />

      <div className="w-full max-w-xl">
        {conteudo === "aprovacao" && estado.aprovacao && (
          <div className="rounded-xl border border-amber-500/40 bg-amber-500/10 p-4">
            <p className="mb-2 text-sm text-app-text">
              O James quer executar <strong className="font-serif">{estado.aprovacao.ferramenta}</strong>
            </p>
            <pre className="mb-3 overflow-x-auto rounded bg-black/40 p-2 text-xs text-app-muted">
              {JSON.stringify(estado.aprovacao.argumentos, null, 2)}
            </pre>
            <div className="flex gap-2">
              <button
                type="button"
                onClick={() => aoDecidir(true)}
                className="rounded-lg bg-emerald-600 px-4 py-1.5 text-sm text-white transition-transform hover:scale-[1.03] active:scale-95"
              >
                Aprovar
              </button>
              <button
                type="button"
                onClick={() => aoDecidir(false)}
                className="rounded-lg bg-red-600 px-4 py-1.5 text-sm text-white transition-transform hover:scale-[1.03] active:scale-95"
              >
                Negar
              </button>
            </div>
          </div>
        )}

        {conteudo === "tela" && saidaVerTela && <LeituraDeTela saida={saidaVerTela} />}

        {conteudo === "ferramenta" && <Atividade ferramentas={estado.ferramentas} />}

        {conteudo === "repouso" && (
          <p className="text-center text-sm text-app-muted">Em repouso. Fale ou escreva para o James.</p>
        )}
      </div>
    </div>
  );
}

function Atividade({ ferramentas }: { ferramentas: FerramentaNoTurno[] }) {
  return (
    <ul className="space-y-1.5">
      {ferramentas.map((f, i) => (
        <li key={i} className="flex items-center gap-3 rounded-lg bg-white/5 px-3 py-2 text-sm">
          <span aria-hidden="true">{f.estado === "voando" ? "⚡" : f.estado === "erro" ? "✗" : "✓"}</span>
          <span className="flex-1 font-mono text-xs text-app-text">{f.nome}</span>
          <span className="text-xs text-app-muted">{duracaoLegivel(f)}</span>
          {f.erro && <span className="text-xs text-amber-300">{f.erro}</span>}
        </li>
      ))}
    </ul>
  );
}

function LeituraDeTela({ saida }: { saida: string }) {
  const { superficie, elementos } = interpretarSaidaVerTela(saida);
  return (
    <div className="space-y-2">
      {superficie && <p className="text-xs text-app-muted">{superficie}</p>}
      <div className="rounded border border-dashed border-white/20 p-3 text-[11px] opacity-50">
        A imagem capturada não é entregue ao cliente nesta fase — só a lista de elementos.
      </div>
      <ul className="max-h-64 overflow-y-auto rounded-lg bg-white/5 p-3 font-mono text-[11px] leading-relaxed text-app-text">
        {elementos.map((e, i) => (
          <li key={i}>{e}</li>
        ))}
      </ul>
    </div>
  );
}
```

- [ ] **Passo 6: Criar o orbe provisório**

A Task 8 reescreve este arquivo por inteiro com o canvas. Por ora, substitua **todo** o conteúdo de `web/src/shell/orbe.tsx` por um esqueleto que aceita a prop e não desenha nada:

```tsx
interface Props {
  analisador: AnalyserNode | null;
}

// Esqueleto: o canvas reativo entra na Task 8.
export function Orbe({ analisador }: Props) {
  void analisador;
  return <div className="size-[320px] shrink-0 rounded-full bg-black/60" aria-label="Orbe do James" />;
}
```

Apague `web/src/shell/orbe.css` e `web/src/shell/orbe.test.tsx` — os dois falam da bolinha-lançador, que morreu.

```bash
rm web/src/shell/orbe.css web/src/shell/orbe.test.tsx
```

- [ ] **Passo 7: Rodar e ver passar**

```bash
npm test --prefix web -- abas/james
```

Esperado: 6 testes passando.

- [ ] **Passo 7.5: Destravar o build — `App.tsx` ainda chama a API antiga do orbe**

`vitest` não faz checagem de tipo (roda em esbuild/swc), então o passo anterior passa mesmo com o build quebrado. Rode:

```bash
npm run build --prefix web
```

Esperado, **antes** deste passo: FALHA —

```
src/App.tsx(107,13): error TS2322: Type '{ aberto: boolean; aoAlternar: Dispatch<SetStateAction<boolean>>; }' is not assignable to type 'IntrinsicAttributes & Props'.
  Property 'aberto' does not exist on type 'IntrinsicAttributes & Props'.
```

`App.tsx:107` ainda tem `<Orbe aberto={aberto} aoAlternar={setAberto} />` — a chamada da bolinha antiga (abrir/fechar painel). O novo `Orbe` do Passo 6 tem interface diferente (`{ analisador: AnalyserNode | null }`), sem `aberto`/`aoAlternar`.

**A recomposição inteira de `App.tsx` é da Task 5, não desta.** Aqui o conserto é cirúrgico: troque **só essa linha**, nada mais no arquivo. Em `web/src/App.tsx:107`:

```tsx
      <Orbe analisador={null} />
```

`null` porque a Task 9 é quem liga o áudio real ao analisador — até lá, o orbe placeholder não anima mesmo. Não mexa em `aberto`, `setAberto` nem em qualquer outra linha de `App.tsx`: eles continuam em uso por outras partes do arquivo até a Task 5 reescrever o componente inteiro.

Rode de novo:

```bash
npm run build --prefix web
```

Esperado agora: build concluído sem erro.

Rode a suíte inteira uma última vez:

```bash
npm test --prefix web
```

Esperado: todos os arquivos passando (o total sobe em relação ao que havia antes da Task 3, e cai em 2 pela remoção de `orbe.test.tsx`).

- [ ] **Passo 8: Commit**

```bash
git add web/src/abas/james.tsx web/src/abas/james.test.tsx web/src/lib/verTela.ts web/src/lib/verTela.test.ts web/src/shell/orbe.tsx web/src/App.tsx
git rm --cached web/src/shell/orbe.css web/src/shell/orbe.test.tsx 2>/dev/null || true
git add -A web/src/shell
git commit -m "feat(web): aba James com aprovacao, atividade com duracao e leitura de tela"
```

---

## Task 4: Aba Mensagens

**Arquivos:**
- Criar: `web/src/abas/mensagens.tsx`
- Teste: `web/src/abas/mensagens.test.tsx`

- [ ] **Passo 1: Escrever o teste que falha**

Crie `web/src/abas/mensagens.test.tsx`:

```tsx
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AbaMensagens } from "./mensagens";
import { estadoInicial, type EstadoTurno } from "../estado/turno";

function comMensagens(): EstadoTurno {
  return {
    ...estadoInicial,
    sessaoAtual: "11111111-1111-1111-1111-111111111111",
    mensagens: [
      { papel: "usuario", texto: "que horas são?" },
      { papel: "james", texto: "são 14h32." },
    ],
  };
}

describe("Aba Mensagens", () => {
  it("desenha os balões da conversa", () => {
    render(
      <AbaMensagens
        estado={comMensagens()}
        conexao="conectado"
        tentativa={0}
        aoEnviar={vi.fn()}
        aoNovaSessao={vi.fn()}
      />,
    );

    expect(screen.getByText("que horas são?")).toBeTruthy();
    expect(screen.getByText("são 14h32.")).toBeTruthy();
  });

  // O "online" verde fixo do design de referência mentiria durante uma queda.
  it("mostra o estado real da conexão", () => {
    const { rerender } = render(
      <AbaMensagens estado={comMensagens()} conexao="conectado" tentativa={0} aoEnviar={vi.fn()} aoNovaSessao={vi.fn()} />,
    );
    expect(screen.getByText("online")).toBeTruthy();

    rerender(
      <AbaMensagens estado={comMensagens()} conexao="reconectando" tentativa={3} aoEnviar={vi.fn()} aoNovaSessao={vi.fn()} />,
    );
    expect(screen.getByText("Reconectando… (tentativa 3)")).toBeTruthy();

    rerender(
      <AbaMensagens estado={comMensagens()} conexao="fechado" tentativa={0} aoEnviar={vi.fn()} aoNovaSessao={vi.fn()} />,
    );
    expect(screen.getByText("desconectado")).toBeTruthy();
  });

  it("trava o campo quando a conexão não está de pé", () => {
    render(
      <AbaMensagens estado={comMensagens()} conexao="reconectando" tentativa={1} aoEnviar={vi.fn()} aoNovaSessao={vi.fn()} />,
    );

    expect((screen.getByPlaceholderText("Escreva para o James…") as HTMLInputElement).disabled).toBe(true);
  });

  it("envia com Enter e limpa o campo", () => {
    const aoEnviar = vi.fn();
    render(
      <AbaMensagens estado={comMensagens()} conexao="conectado" tentativa={0} aoEnviar={aoEnviar} aoNovaSessao={vi.fn()} />,
    );

    const campo = screen.getByPlaceholderText("Escreva para o James…") as HTMLInputElement;
    fireEvent.change(campo, { target: { value: "oi" } });
    fireEvent.keyDown(campo, { key: "Enter" });

    expect(aoEnviar).toHaveBeenCalledWith("oi");
    expect(campo.value).toBe("");
  });

  it("não envia mensagem em branco", () => {
    const aoEnviar = vi.fn();
    render(
      <AbaMensagens estado={comMensagens()} conexao="conectado" tentativa={0} aoEnviar={aoEnviar} aoNovaSessao={vi.fn()} />,
    );

    const campo = screen.getByPlaceholderText("Escreva para o James…");
    fireEvent.change(campo, { target: { value: "   " } });
    fireEvent.keyDown(campo, { key: "Enter" });

    expect(aoEnviar).not.toHaveBeenCalled();
  });

  it("pede sessão nova pelo botão", () => {
    const aoNovaSessao = vi.fn();
    render(
      <AbaMensagens estado={comMensagens()} conexao="conectado" tentativa={0} aoEnviar={vi.fn()} aoNovaSessao={aoNovaSessao} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Nova sessão" }));

    expect(aoNovaSessao).toHaveBeenCalled();
  });

  // O mockup de referência (painel-de-controle-james/messages-panel.tsx:86)
  // já tinha essa guarda; a primeira versão deste componente a perdeu na
  // tradução. Sem ela, confirmar um candidato de IME (japonês/chinês/
  // coreano) com Enter dispara o envio com texto parcial de composição.
  it("não envia durante composição de IME", () => {
    const aoEnviar = vi.fn();
    render(
      <AbaMensagens estado={comMensagens()} conexao="conectado" tentativa={0} aoEnviar={aoEnviar} aoNovaSessao={vi.fn()} />,
    );

    const campo = screen.getByPlaceholderText("Escreva para o James…") as HTMLInputElement;
    fireEvent.change(campo, { target: { value: "候補" } });
    fireEvent.keyDown(campo, { key: "Enter", isComposing: true });

    expect(aoEnviar).not.toHaveBeenCalled();
  });

  // conversa.tsx, memoria.tsx e sessoes.tsx já usam role="status" no mesmo
  // tipo de aviso — sem ele, quem usa leitor de tela só percebe a queda de
  // conexão quando o campo já está desabilitado.
  it("o indicador de conexão é um role=status", () => {
    render(
      <AbaMensagens estado={comMensagens()} conexao="reconectando" tentativa={2} aoEnviar={vi.fn()} aoNovaSessao={vi.fn()} />,
    );

    expect(screen.getByRole("status").textContent).toBe("Reconectando… (tentativa 2)");
  });
});
```

- [ ] **Passo 2: Rodar e ver falhar**

```bash
npm test --prefix web -- abas/mensagens
```

Esperado: FALHA com `Failed to resolve import "./mensagens"`.

- [ ] **Passo 3: Escrever o componente**

Crie `web/src/abas/mensagens.tsx`:

```tsx
import { useEffect, useRef, useState } from "react";
import { Plus, Send, Sparkle } from "lucide-react";
import type { EstadoTurno } from "../estado/turno";
import type { EstadoConexao } from "../lib/ws";
import { separarSegmentos } from "../lib/markdown";

interface Props {
  estado: EstadoTurno;
  conexao: EstadoConexao;
  tentativa: number;
  aoEnviar: (texto: string) => void;
  aoNovaSessao: () => void;
}

// "fechado" é encerramento deliberado (ClienteWS.fechar()), não queda de
// rede: não existe timer de reconexão rodando por trás. Dizer "Reconectando"
// ali prometeria uma tentativa automática que não vai acontecer.
function textoConexao(conexao: EstadoConexao, tentativa: number): string {
  if (conexao === "conectado") return "online";
  if (conexao === "fechado") return "desconectado";
  if (conexao === "conectando") return "conectando…";
  const sufixo = tentativa > 1 ? ` (tentativa ${tentativa})` : "";
  return `Reconectando…${sufixo}`;
}

function Corpo({ texto }: { texto: string }) {
  return (
    <>
      {separarSegmentos(texto).map((segmento, i) =>
        segmento.tipo === "texto" ? (
          <span key={i}>{segmento.conteudo}</span>
        ) : (
          <pre key={i} className="my-1 overflow-x-auto rounded bg-black/40 p-2 text-xs">
            <code className={segmento.linguagem}>{segmento.conteudo}</code>
          </pre>
        ),
      )}
    </>
  );
}

export function AbaMensagens({ estado, conexao, tentativa, aoEnviar, aoNovaSessao }: Props) {
  const [rascunho, setRascunho] = useState("");
  const listaRef = useRef<HTMLDivElement>(null);
  // Auto-scroll condicional: rolar sempre arrancaria o usuário do meio da
  // conversa toda vez que uma resposta chegasse. Só acompanha quem já estava
  // no fim.
  const grudadoNoFim = useRef(true);
  const bloqueado = conexao !== "conectado";

  useEffect(() => {
    const lista = listaRef.current;
    if (!lista || !grudadoNoFim.current) return;
    lista.scrollTop = lista.scrollHeight;
  }, [estado.mensagens]);

  const aoRolar = () => {
    const lista = listaRef.current;
    if (!lista) return;
    const distanciaDoFim = lista.scrollHeight - lista.scrollTop - lista.clientHeight;
    grudadoNoFim.current = distanciaDoFim < 40;
  };

  const enviar = () => {
    const texto = rascunho.trim();
    if (!texto) return;
    aoEnviar(texto);
    setRascunho("");
    grudadoNoFim.current = true;
  };

  return (
    <div className="flex h-full flex-col">
      <header className="flex items-center gap-3 border-b border-app-line px-6 py-4">
        <span className="grid size-9 shrink-0 place-items-center rounded-full bg-app-blue text-app-blue-foreground">
          <Sparkle className="size-4" />
        </span>
        <div className="min-w-0 flex-1">
          <p className="font-serif text-app-text">James</p>
          <p
            role="status"
            className={`text-xs ${conexao === "conectado" ? "text-emerald-400" : "text-amber-400"}`}
          >
            {textoConexao(conexao, tentativa)}
          </p>
        </div>
        <button
          type="button"
          onClick={aoNovaSessao}
          aria-label="Nova sessão"
          className="flex items-center gap-1.5 rounded-lg bg-white/5 px-3 py-1.5 text-xs text-app-text transition-colors hover:bg-app-hover"
        >
          <Plus className="size-3.5" />
          Nova sessão
        </button>
      </header>

      <div ref={listaRef} onScroll={aoRolar} className="flex-1 space-y-4 overflow-y-auto px-6 py-6">
        {estado.mensagens.map((m, i) => (
          <div key={i} className={`flex ${m.papel === "usuario" ? "justify-end" : "justify-start"}`}>
            <div
              className={`max-w-[75%] rounded-2xl px-4 py-2.5 text-left text-sm leading-relaxed ${
                m.papel === "usuario"
                  ? "rounded-br-sm bg-app-blue text-app-blue-foreground"
                  : "rounded-bl-sm bg-white/5 text-app-text"
              }`}
            >
              <Corpo texto={m.texto} />
            </div>
          </div>
        ))}
      </div>

      <div className="border-t border-app-line p-4">
        <div className="flex items-center gap-2 rounded-xl bg-white/5 px-3 py-2">
          <input
            value={rascunho}
            disabled={bloqueado}
            onChange={(e) => setRascunho(e.target.value)}
            onKeyDown={(e) => {
              // isComposing/keyCode 229: confirmar um candidato de IME
              // (japonês/chinês/coreano) também dispara Enter. Sem esta
              // guarda o texto parcial de composição seria enviado. O
              // mockup de referência já tinha isto — copiar sem checar
              // perdeu a proteção.
              if (e.key === "Enter" && !e.nativeEvent.isComposing && e.keyCode !== 229) {
                e.preventDefault();
                enviar();
              }
            }}
            placeholder="Escreva para o James…"
            className="flex-1 bg-transparent text-sm text-app-text placeholder:text-app-muted focus:outline-none disabled:opacity-40"
            autoFocus
          />
          <button
            type="button"
            onClick={enviar}
            disabled={bloqueado}
            aria-label="Enviar mensagem"
            className="grid size-8 place-items-center rounded-lg bg-app-blue text-app-blue-foreground transition-transform hover:scale-105 active:scale-95 disabled:opacity-40"
          >
            <Send className="size-4" />
          </button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Passo 4: Rodar e ver passar**

```bash
npm test --prefix web -- abas/mensagens
```

Esperado: 8 testes passando (6 originais + IME + role=status).

- [ ] **Passo 5: Commit**

```bash
git add web/src/abas/mensagens.tsx web/src/abas/mensagens.test.tsx
git commit -m "feat(web): aba Mensagens com estado real de conexao e auto-scroll condicional"
```

---

## Task 5: Composição do shell e limpeza do que morreu

**Arquivos:**
- Criar: `web/src/shell/aplicativo.tsx`
- Criar: `web/src/abas/dashboard.tsx`
- Modificar: `web/src/App.tsx`
- Apagar: `web/src/trilha.tsx`, `web/src/trilha.test.tsx`, `web/src/visualizador.tsx`, `web/src/visualizador.test.tsx`, `web/src/abas/conversa.tsx`, `web/src/abas/conversa.test.tsx`, `web/src/abas/atividade.tsx`, `web/src/abas/tela.tsx`, `web/src/abas/tela.test.tsx`
- Teste: `web/src/App.test.tsx` (ajustar o que já existe)

- [ ] **Passo 1: Escrever o DashBoard vazio**

Crie `web/src/abas/dashboard.tsx`. Estado vazio franco: **sem gráfico de exemplo e sem número de mentira** (CA13).

```tsx
import { LayoutDashboard } from "lucide-react";

export function AbaDashboard() {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-3 p-8 text-center">
      <LayoutDashboard className="size-8 text-app-icon" />
      <h2 className="font-serif text-xl text-app-text">DashBoard</h2>
      <p className="max-w-md text-sm text-app-muted">
        Custo, uso, duração e erros ainda não são medidos. A instrumentação chega na Fase 7B — até lá esta
        aba fica vazia de propósito, em vez de mostrar número inventado.
      </p>
    </div>
  );
}
```

- [ ] **Passo 2: Escrever a composição**

Crie `web/src/shell/aplicativo.tsx`:

```tsx
import { useState } from "react";
import { BarraLateral, type IdAba } from "./barraLateral";
import { ControlesJanela } from "./controlesJanela";

interface Props {
  aprovacoesPendentes: number;
  // Cada aba é montada pelo App, que é quem tem o ClienteWS e o redutor.
  // O shell só decide qual mostrar — não conhece estado de turno.
  paineis: Record<IdAba, React.ReactNode>;
}

export function Aplicativo({ aprovacoesPendentes, paineis }: Props) {
  const [ativa, setAtiva] = useState<IdAba>("james");

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-app-bg text-app-text">
      <div className="relative">
        <ControlesJanela />
        <BarraLateral ativa={ativa} aoSelecionar={setAtiva} aprovacoesPendentes={aprovacoesPendentes} />
      </div>
      <main className="min-w-0 flex-1 overflow-hidden bg-app-panel">{paineis[ativa]}</main>
    </div>
  );
}
```

- [ ] **Passo 3: Criar um `controlesJanela.tsx` provisório**

A Task 7 põe o comportamento real. Por ora, crie `web/src/shell/controlesJanela.tsx` devolvendo `null`, para o `aplicativo.tsx` compilar:

```tsx
// Os controles reais entram na Task 7, junto com os endpoints do shell.
export function ControlesJanela() {
  return null;
}
```

- [ ] **Passo 4: Recompor o `App.tsx`**

Substitua **todo** o conteúdo de `web/src/App.tsx` por:

```tsx
import { useEffect, useReducer, useRef, useState } from "react";
import { AbaDashboard } from "./abas/dashboard";
import { AbaJames } from "./abas/james";
import { AbaMensagens } from "./abas/mensagens";
import { Memoria } from "./abas/memoria";
import { estadoInicial, reduzir } from "./estado/turno";
import { ClienteWS, type EstadoConexao } from "./lib/ws";
import { Aplicativo } from "./shell/aplicativo";

function urlWS(): string {
  const protocolo = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocolo}//${window.location.host}/ws`;
}

export default function App() {
  const [conexao, setConexao] = useState<EstadoConexao>("conectando");
  const [tentativa, setTentativa] = useState(0);
  const [saidaVerTela, setSaidaVerTela] = useState<string | null>(null);
  const [estado, despachar] = useReducer(reduzir, estadoInicial);
  const clienteRef = useRef<ClienteWS | null>(null);
  // Espelho de estado.sessaoAtual só para leitura dentro do efeito de
  // conexão, que roda uma vez e não enxerga estado novo pelo closure. É
  // derivado, não uma segunda fonte de verdade: quem decide qual é a sessão
  // continua sendo o redutor.
  const sessaoAtualRef = useRef<string | null>(null);
  const jaConectouRef = useRef(false);

  useEffect(() => {
    sessaoAtualRef.current = estado.sessaoAtual;
  }, [estado.sessaoAtual]);

  useEffect(() => {
    const cliente = new ClienteWS(urlWS());
    clienteRef.current = cliente;

    cliente.aoMudarEstado((novoEstado, n) => {
      setConexao(novoEstado);
      setTentativa(n);

      if (novoEstado !== "conectado") return;

      // Reconexão abre uma conexão NOVA no jamesd, e sessão e histórico
      // vivem por conexão lá (ws.go: variáveis locais do handler). Sem
      // reemitir "retomar", a tela segue mostrando a conversa antiga
      // enquanto o modelo volta a zero e o turno seguinte grava numa sessão
      // órfã — sem erro nenhum, que é o que torna isso perigoso.
      //
      // Sai daqui e não de um efeito: ClienteWS publica "conectado" dentro
      // do onopen ANTES de descarregar a fila, então este envio entra na
      // frente da mensagem que esperou a queda. É a ordem que importa.
      if (jaConectouRef.current && sessaoAtualRef.current) {
        cliente.enviar("retomar", crypto.randomUUID(), { sessao_id: sessaoAtualRef.current });
      }
      jaConectouRef.current = true;
    });

    cliente.on("resposta_final", (dados) => despachar({ tipo: "resposta_final", dados }));
    cliente.on("erro", (dados) => despachar({ tipo: "erro", dados }));
    cliente.on("ferramenta_iniciada", (dados) => despachar({ tipo: "ferramenta_iniciada", dados }));
    cliente.on("ferramenta_concluida", (dados) => {
      despachar({ tipo: "ferramenta_concluida", dados });
      if (dados.nome === "ver_tela" && dados.saida) setSaidaVerTela(dados.saida);
    });
    cliente.on("transcricao", (dados) => despachar({ tipo: "transcricao", dados }));
    cliente.on("aprovacao_requerida", (dados) => despachar({ tipo: "aprovacao_requerida", dados }));
    cliente.on("sessao_retomada", (dados) => despachar({ tipo: "sessao_retomada", dados }));

    cliente.on("audio_pronto", (dados: { id: string }) => {
      const excluirAudio = () => {
        fetch(`/api/audio/${dados.id}`, { method: "DELETE" }).catch(() => {});
      };
      const audio = new Audio(`/api/audio/${dados.id}`);
      audio.addEventListener("ended", excluirAudio);
      // Autoplay pode ser bloqueado sem gesto recente do usuário — se falhar,
      // o áudio só não toca, mas o arquivo ainda precisa sair do disco.
      audio.play().catch(excluirAudio);
    });

    return () => cliente.fechar();
  }, []);

  const enviar = (texto: string) => {
    despachar({ tipo: "envio_local", texto });
    clienteRef.current?.enviar("mensagem", crypto.randomUUID(), { texto });
  };

  const decidir = (aprovado: boolean) => {
    if (!estado.aprovacao) return;
    clienteRef.current?.enviar(aprovado ? "aprovar" : "negar", estado.aprovacao.id, {});
    despachar({ tipo: "aprovacao_decidida" });
  };

  // Provisório: a Task 14 liga isto ao sessionStorage e ao comando
  // nova_sessao. Deixar sem efeito aqui manteria o botão mentindo.
  const novaSessao = () => {};

  return (
    <Aplicativo
      aprovacoesPendentes={estado.aprovacao ? 1 : 0}
      paineis={{
        james: <AbaJames estado={estado} saidaVerTela={saidaVerTela} aoDecidir={decidir} analisador={null} />,
        mensagens: (
          <AbaMensagens
            estado={estado}
            conexao={conexao}
            tentativa={tentativa}
            aoEnviar={enviar}
            aoNovaSessao={novaSessao}
          />
        ),
        memoria: <Memoria />,
        historico: <p className="p-8 text-sm text-app-muted">Histórico chega na Task 13.</p>,
        dashboard: <AbaDashboard />,
      }}
    />
  );
}
```

- [ ] **Passo 5: Apagar o que morreu**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && git rm web/src/trilha.tsx web/src/trilha.test.tsx web/src/visualizador.tsx web/src/visualizador.test.tsx web/src/abas/conversa.tsx web/src/abas/conversa.test.tsx web/src/abas/atividade.tsx web/src/abas/tela.tsx web/src/abas/tela.test.tsx
```

Se algum desses caminhos não existir, o `git rm` falha e lista qual — remova-o do comando e siga.

- [ ] **Passo 6: Ajustar o `App.test.tsx`**

Os quatro casos de `web/src/App.test.tsx` exercitam a continuidade de sessão pelo `SocketFalso`, não pela interface — **nenhum deles clica na bolinha**, então os quatro continuam válidos. Eles guardam a correção mais cara da Fase 6; não reescreva a lógica.

Uma única mudança é necessária, no caso `"retomar sai antes da mensagem que estava na fila"` (linha ~113): o comentário das linhas 127-129 explica o `expect(campo).toBeNull()` dizendo que o painel só existe com o orbe aberto. Isso deixou de ser verdade. A asserção continua correta por outro motivo — a aba inicial é a James, e o campo de texto vive na aba Mensagens. Troque o comentário por:

```tsx
    const campo = document.querySelector("input") as HTMLInputElement | null;
    // A aba inicial é a James, e o campo de texto vive na aba Mensagens —
    // não há input no DOM aqui. Exercitamos a fila pelo próprio ClienteWS,
    // que é onde ela vive.
    expect(campo).toBeNull();
```

- [ ] **Passo 7: Rodar a suíte inteira do front**

```bash
npm test --prefix web
```

Esperado: tudo verde. Falhas restantes só podem ser de arquivos que importam o que foi apagado — corrija os imports, não apague o teste.

- [ ] **Passo 8: Build e commit**

```bash
npm run build --prefix web
```

Esperado: build sem erro.

```bash
git add -A web/src
git commit -m "feat(web): shell de aplicativo substitui a bolinha, trilha e visualizador"
```

---

## Task 6: Minimizar, maximizar e fechar no servidor de controle

**Arquivos:**
- Modificar: `shell/controle.go`
- Teste: `shell/controle_test.go`

- [ ] **Passo 1: Escrever o teste que falha**

Abra `shell/controle_test.go` e **acrescente** (o arquivo já tem testes de ancoragem e roteamento — não os apague).

O handler de `/fechar` responde 204 e só **depois** chama `Fechar` numa goroutine (Passo 4) — de propósito, porque `Quit` mataria o processo antes do cliente receber a resposta, senão. Isso significa que checar um `bool` logo depois de `ServeHTTP` retornar é uma corrida real: a goroutine spawnada não tem nenhum ponto de trabalho bloqueante, então nada força o scheduler do Go a rodá-la antes da asserção — o teste falharia **de forma determinística**, não ocasional. Por isso `fechou` é canal, não bool, e a checagem de `/fechar` espera nele com um teto curto de tempo.

```go
// espiaoJanela registra o que foi chamado, para o teste conferir o
// roteamento sem uma janela nativa de verdade. fechou é canal, e não bool,
// pelo motivo explicado acima — checagem síncrona falharia sempre.
type espiaoJanela struct {
	chamadasJanela
	minimizou bool
	alternou  bool
	fechou    chan struct{}
}

func (e *espiaoJanela) Minimizar(ctx context.Context)          { e.minimizou = true }
func (e *espiaoJanela) AlternarMaximizado(ctx context.Context) { e.alternou = true }
func (e *espiaoJanela) Fechar(ctx context.Context)             { close(e.fechou) }

func TestControlesDeJanelaRoteiam(t *testing.T) {
	casos := []struct {
		rota     string
		conferir func(*espiaoJanela) bool
	}{
		{"/minimizar", func(e *espiaoJanela) bool { return e.minimizou }},
		{"/maximizar", func(e *espiaoJanela) bool { return e.alternou }},
		{"/fechar", func(e *espiaoJanela) bool {
			select {
			case <-e.fechou:
				return true
			case <-time.After(200 * time.Millisecond):
				return false
			}
		}},
	}

	for _, caso := range casos {
		t.Run(caso.rota, func(t *testing.T) {
			espiao := &espiaoJanela{fechou: make(chan struct{})}
			mux := montarMuxControle(context.Background(), espiao)

			req := httptest.NewRequest(http.MethodPost, caso.rota, nil)
			gravador := httptest.NewRecorder()
			mux.ServeHTTP(gravador, req)

			if gravador.Code != http.StatusOK && gravador.Code != http.StatusNoContent {
				t.Fatalf("status %d para %s", gravador.Code, caso.rota)
			}
			if !caso.conferir(espiao) {
				t.Fatalf("%s não chamou a função da janela", caso.rota)
			}
		})
	}
}
```

`controle_test.go` hoje só importa `net/http/httptest`, não `net/http` puro nem `time` — acrescente os dois aos imports.

**Um segundo furo, achado no mesmo passo:** `chamadasJanelaFalsa` — a struct concreta que os testes antigos de `/redimensionar` e `/posicao` já usam — precisa implementar os três métodos novos da interface `chamadasJanela` para o pacote continuar compilando (é struct concreta, não composição por embedding como `espiaoJanela`; Go exige todos os métodos da interface). Acrescente, junto dos métodos que ela já tem:

```go
func (c *chamadasJanelaFalsa) Minimizar(_ context.Context)          {}
func (c *chamadasJanelaFalsa) AlternarMaximizado(_ context.Context) {}
func (c *chamadasJanelaFalsa) Fechar(_ context.Context)             {}
```

No-op: os testes antigos não exercitam esses três métodos, só precisam que a struct compile contra a interface maior.

- [ ] **Passo 2: Rodar e ver falhar**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts/shell" && go test ./... -run TestControlesDeJanelaRoteiam
```

Esperado: FALHA de compilação — `espiaoJanela` não satisfaz `chamadasJanela` porque a interface não tem os métodos novos.

- [ ] **Passo 3: Acrescentar os métodos à interface e ao adaptador**

Em `shell/controle.go`, dentro de `type chamadasJanela interface { ... }`, acrescente:

```go
	Minimizar(ctx context.Context)
	// AlternarMaximizado alterna entre maximizado e restaurado — é o que o
	// círculo verde do design faz, e é a única operação que o Wails expõe
	// (não há "maximizar" e "restaurar" separados).
	AlternarMaximizado(ctx context.Context)
	Fechar(ctx context.Context)
```

E acrescente os três adaptadores, junto dos que já existem:

```go
func (chamadasJanelaWails) Minimizar(ctx context.Context) {
	wailsruntime.WindowMinimise(ctx)
}

func (chamadasJanelaWails) AlternarMaximizado(ctx context.Context) {
	wailsruntime.WindowToggleMaximise(ctx)
}

func (chamadasJanelaWails) Fechar(ctx context.Context) {
	wailsruntime.Quit(ctx)
}
```

- [ ] **Passo 4: Acrescentar as rotas**

Em `montarMuxControle`, antes do `return mux`:

```go
	mux.HandleFunc("POST /minimizar", func(w http.ResponseWriter, r *http.Request) {
		chamadas.Minimizar(ctx)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /maximizar", func(w http.ResponseWriter, r *http.Request) {
		chamadas.AlternarMaximizado(ctx)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /fechar", func(w http.ResponseWriter, r *http.Request) {
		// Responde ANTES de fechar: Quit derruba o processo, e o cliente
		// ficaria com um fetch pendurado até o timeout.
		w.WriteHeader(http.StatusNoContent)
		go chamadas.Fechar(ctx)
	})
```

- [ ] **Passo 5: Rodar e ver passar**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts/shell" && go vet ./... && go test ./...
```

Esperado: todos os testes do pacote passando, inclusive os antigos.

- [ ] **Passo 6: Commit**

```bash
git add shell/controle.go shell/controle_test.go
git commit -m "feat(shell): minimizar, maximizar e fechar no servidor de controle"
```

---

## Task 7: A janela de aplicativo e os três círculos

**Arquivos:**
- Modificar: `shell/main.go`
- Modificar: `web/src/lib/janela.ts`
- Modificar: `web/src/shell/controlesJanela.tsx`
- Teste: `web/src/shell/controlesJanela.test.tsx`

- [ ] **Passo 1: Escrever o teste que falha**

Crie `web/src/shell/controlesJanela.test.tsx`:

```tsx
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ControlesJanela } from "./controlesJanela";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("Controles de janela", () => {
  // Fora do shell (navegador, celular) a porta 17313 não existe. Desenhar
  // três círculos que não fazem nada seria pior que não desenhar nada.
  it("não desenha nada quando o shell não responde", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("recusado")));

    const { container } = render(<ControlesJanela />);

    await waitFor(() => expect(container.querySelector("button")).toBeNull());
  });

  it("desenha os três controles quando o shell responde", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true }));

    render(<ControlesJanela />);

    await waitFor(() => expect(screen.getByRole("button", { name: "Fechar" })).toBeTruthy());
    expect(screen.getByRole("button", { name: "Minimizar" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Maximizar" })).toBeTruthy();
  });

  it("chama o endpoint certo em cada círculo", async () => {
    const fetchFalso = vi.fn().mockResolvedValue({ ok: true });
    vi.stubGlobal("fetch", fetchFalso);

    render(<ControlesJanela />);
    await waitFor(() => screen.getByRole("button", { name: "Minimizar" }));

    fireEvent.click(screen.getByRole("button", { name: "Minimizar" }));
    expect(fetchFalso).toHaveBeenCalledWith("http://127.0.0.1:17313/minimizar", { method: "POST" });

    fireEvent.click(screen.getByRole("button", { name: "Maximizar" }));
    expect(fetchFalso).toHaveBeenCalledWith("http://127.0.0.1:17313/maximizar", { method: "POST" });

    fireEvent.click(screen.getByRole("button", { name: "Fechar" }));
    expect(fetchFalso).toHaveBeenCalledWith("http://127.0.0.1:17313/fechar", { method: "POST" });
  });
});
```

- [ ] **Passo 2: Rodar e ver falhar**

```bash
npm test --prefix web -- controlesJanela
```

Esperado: FALHA — o componente hoje devolve `null` sempre.

- [ ] **Passo 3: Reescrever o `janela.ts`**

Substitua **todo** o conteúdo de `web/src/lib/janela.ts` por:

```ts
// Controle da janela nativa do shell, via o servidor de controle local que
// shell/controle.go expõe em loopback — nunca window.runtime do Wails, que
// não é injetado na página que o jamesd serve (ver o comentário no topo de
// shell/controle.go pro porquê).
// Fora do shell (navegador, celular), a porta não existe: todo fetch falha
// e vira no-op silencioso.
const ENDERECO_CONTROLE_SHELL = "http://127.0.0.1:17313";

// redimensionarJanela saiu na Fase 7A: existia para a bolinha crescer ao
// abrir o chat, e a bolinha morreu. Maximizar passa por alternarMaximizado.

export async function posicaoJanela(): Promise<{ x: number; y: number } | null> {
  try {
    const resp = await fetch(`${ENDERECO_CONTROLE_SHELL}/posicao`);
    if (!resp.ok) return null;
    return await resp.json();
  } catch {
    return null;
  }
}

export function moverJanela(x: number, y: number): void {
  fetch(`${ENDERECO_CONTROLE_SHELL}/posicao?x=${Math.round(x)}&y=${Math.round(y)}`, { method: "POST" }).catch(() => {});
}

// shellPresente sonda o servidor de controle. É como a SPA descobre que
// está rodando dentro do Wails — não por user-agent, que mentiria em
// qualquer WebView.
export async function shellPresente(): Promise<boolean> {
  try {
    const resp = await fetch(`${ENDERECO_CONTROLE_SHELL}/posicao`);
    return resp.ok;
  } catch {
    return false;
  }
}

export function minimizarJanela(): void {
  fetch(`${ENDERECO_CONTROLE_SHELL}/minimizar`, { method: "POST" }).catch(() => {});
}

export function alternarMaximizado(): void {
  fetch(`${ENDERECO_CONTROLE_SHELL}/maximizar`, { method: "POST" }).catch(() => {});
}

export function fecharJanela(): void {
  fetch(`${ENDERECO_CONTROLE_SHELL}/fechar`, { method: "POST" }).catch(() => {});
}
```

Se `web/src/lib/janela.test.ts` tiver casos de `redimensionarJanela`, apague só esses casos.

- [ ] **Passo 4: Escrever os controles**

Substitua **todo** o conteúdo de `web/src/shell/controlesJanela.tsx` por:

```tsx
import { useEffect, useState } from "react";
import { alternarMaximizado, fecharJanela, minimizarJanela, shellPresente } from "../lib/janela";

export function ControlesJanela() {
  const [dentroDoShell, setDentroDoShell] = useState(false);

  useEffect(() => {
    let vivo = true;
    shellPresente().then((presente) => {
      if (vivo) setDentroDoShell(presente);
    });
    return () => {
      vivo = false;
    };
  }, []);

  if (!dentroDoShell) return null;

  return (
    <div className="absolute left-4 top-4 z-10 flex gap-2">
      <button type="button" aria-label="Fechar" onClick={fecharJanela} className="size-3 rounded-full bg-[#ff5f57]" />
      <button type="button" aria-label="Minimizar" onClick={minimizarJanela} className="size-3 rounded-full bg-[#febc2e]" />
      <button
        type="button"
        aria-label="Maximizar"
        onClick={alternarMaximizado}
        className="size-3 rounded-full bg-[#28c840]"
      />
    </div>
  );
}
```

- [ ] **Passo 5: Rodar e ver passar**

```bash
npm test --prefix web -- controlesJanela
```

Esperado: 3 testes passando.

- [ ] **Passo 6: Transformar a janela nativa**

Em `shell/main.go`, dentro de `wails.Run(&options.App{...})`:

- `Width: 100` → `Width: 1100`
- `Height: 100` → `Height: 1100` **não** — use `Height: 760`
- `AlwaysOnTop: true` → **remova a linha**
- Remova o bloco `Windows: &windows.Options{WebviewIsTransparent: true}` inteiro, e o import `.../options/windows` que fica sem uso
- `BackgroundColour` → troque para opaco: `&options.RGBA{R: 10, G: 10, B: 12, A: 255}`
- `Frameless: true` → **mantenha**, é o que permite os três círculos serem os controles
- Acrescente `MinWidth: 900` e `MinHeight: 600`

Substitua também o comentário do topo do arquivo, que ainda descreve a bolinha:

```go
// Comando shell: invólucro Wails da janela do James — frameless (os
// controles de janela são os três círculos da SPA, ver
// web/src/shell/controlesJanela.tsx), opaca e redimensionável, com o
// WebView2 apontando pro jamesd local. Não tem lógica de aplicação
// própria: toda UI e todo estado vivem no jamesd + web/.
//
// Era um widget 100x100 transparente e always-on-top, porque a interface
// era uma bolinha flutuante que crescia ao abrir o chat. A Fase 7A trocou
// isso por um aplicativo com barra lateral, e o modelo de widget perdeu a
// função.
```

- [ ] **Passo 7: Conferir que o shell compila**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts/shell" && go vet ./... && go build ./...
```

Esperado: sem erro. Se `windows` ficou importado sem uso, o compilador aponta — remova o import.

- [ ] **Passo 8: Commit**

```bash
git add shell/main.go web/src/lib/janela.ts web/src/lib/janela.test.ts web/src/shell/controlesJanela.tsx web/src/shell/controlesJanela.test.tsx
git commit -m "feat(shell): janela de aplicativo com controles reais no lugar do widget flutuante"
```

---

## Task 8: O orbe reativo

**Arquivos:**
- Modificar: `web/src/shell/orbe.tsx`
- Teste: `web/src/shell/orbe.test.tsx`

O canvas não é testável em jsdom (`getContext` devolve `null` sem a lib `canvas`). Por isso a matemática da deformação sai em funções puras, que **são** testadas, e o componente só as liga ao canvas.

- [ ] **Passo 1: Escrever o teste que falha**

Crie `web/src/shell/orbe.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Orbe, energiaDaFaixa, raioDoOrbe } from "./orbe";

describe("Matemática do orbe", () => {
  it("energia de faixa vazia é zero", () => {
    expect(energiaDaFaixa(new Uint8Array([0, 0, 0, 0]), 0, 1)).toBe(0);
  });

  it("energia de faixa saturada é um", () => {
    expect(energiaDaFaixa(new Uint8Array([255, 255, 255, 255]), 0, 1)).toBe(1);
  });

  // Graves = primeiro quarto dos bins. É o que puxa o raio (§5.1).
  it("isola a faixa pedida", () => {
    const dados = new Uint8Array([255, 255, 0, 0, 0, 0, 0, 0]);
    expect(energiaDaFaixa(dados, 0, 0.25)).toBe(1);
    expect(energiaDaFaixa(dados, 0.25, 1)).toBe(0);
  });

  it("o raio cresce com a energia, a partir do raio de repouso", () => {
    expect(raioDoOrbe(0)).toBe(96);
    expect(raioDoOrbe(1)).toBe(130);
    expect(raioDoOrbe(0.5)).toBe(113);
  });
});

describe("Orbe", () => {
  it("desenha o canvas mesmo sem analisador", () => {
    render(<Orbe analisador={null} />);
    expect(screen.getByLabelText("Visualização de áudio do James")).toBeTruthy();
  });

  it("não quebra quando o canvas não tem contexto 2d (jsdom)", () => {
    const analisadorFalso = {
      frequencyBinCount: 64,
      getByteFrequencyData: vi.fn(),
    } as unknown as AnalyserNode;

    expect(() => render(<Orbe analisador={analisadorFalso} />)).not.toThrow();
  });
});
```

- [ ] **Passo 2: Rodar e ver falhar**

```bash
npm test --prefix web -- shell/orbe
```

Esperado: FALHA — `energiaDaFaixa` e `raioDoOrbe` não existem.

- [ ] **Passo 3: Reescrever o orbe**

Substitua **todo** o conteúdo de `web/src/shell/orbe.tsx` por:

```tsx
import { useEffect, useRef } from "react";

const TAMANHO = 320;
const RAIO_REPOUSO = 96;
const GANHO_RAIO = 34;
const PONTOS_BORDA = 96;

// energiaDaFaixa devolve 0..1 para uma fatia do espectro. inicio e fim são
// frações (0 = primeiro bin, 1 = último), o que deixa a chamada legível:
// graves são energiaDaFaixa(dados, 0, 0.25).
export function energiaDaFaixa(dados: Uint8Array, inicio: number, fim: number): number {
  const de = Math.floor(inicio * dados.length);
  const ate = Math.max(de + 1, Math.floor(fim * dados.length));
  let soma = 0;
  for (let i = de; i < ate; i++) soma += dados[i];
  return soma / (ate - de) / 255;
}

// raioDoOrbe traduz energia em raio. Arredonda para o teste poder afirmar
// valor exato — meio pixel não muda nada na tela.
export function raioDoOrbe(energia: number): number {
  return Math.round(RAIO_REPOUSO + energia * GANHO_RAIO);
}

interface Props {
  // Null = sem áudio tocando: o orbe respira devagar em vez de ficar parado.
  analisador: AnalyserNode | null;
}

export function Orbe({ analisador }: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const analisadorRef = useRef<AnalyserNode | null>(analisador);

  // O analisador troca a cada áudio novo. Guardar em ref (e não fechar sobre
  // a prop) evita cancelar e recriar o requestAnimationFrame a cada resposta
  // do James — o laço de desenho nasce uma vez e vive enquanto a aba viver.
  useEffect(() => {
    analisadorRef.current = analisador;
  }, [analisador]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    // jsdom não implementa canvas 2d. Sair aqui mantém o componente
    // renderizável em teste sem a lib `canvas` instalada.
    if (!ctx) return;

    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    canvas.width = TAMANHO * dpr;
    canvas.height = TAMANHO * dpr;
    ctx.scale(dpr, dpr);

    const centro = TAMANHO / 2;
    let fase = 0;
    let quadro = 0;

    const desenhar = () => {
      fase += 0.03;

      const analisadorAtual = analisadorRef.current;
      // A ANOTAÇÃO precisa ser Uint8Array<ArrayBuffer>, não Uint8Array puro:
      // TS 5.9 com o lib.dom.d.ts atual tipa
      // AnalyserNode.getByteFrequencyData exigindo esse genérico exato, e o
      // ALIAS "Uint8Array" sem parâmetro widena para Uint8Array<ArrayBufferLike>
      // (supertipo, não atribuível) — mesmo que o valor atribuído já seja
      // Uint8Array<ArrayBuffer>. `new Uint8Array(n)` sozinho já devolve
      // Uint8Array<ArrayBuffer> (ver a sobrecarga `new (length: number)` em
      // lib.es5.d.ts); não precisa de `new ArrayBuffer(n)` por baixo —
      // testado isoladamente antes de escrever este comentário. Achado
      // rodando o build de verdade: o texto anterior deste plano não
      // compilava.
      let dados: Uint8Array<ArrayBuffer> | null = null;
      if (analisadorAtual) {
        dados = new Uint8Array(analisadorAtual.frequencyBinCount);
        analisadorAtual.getByteFrequencyData(dados);
      }

      // Graves controlam o tamanho; agudos, a ondulação fina (§5.1). Sem
      // áudio, respiração senoidal lenta.
      const graves = dados ? energiaDaFaixa(dados, 0, 0.25) : 0.12 + (Math.sin(fase) * 0.5 + 0.5) * 0.1;
      const agudos = dados ? energiaDaFaixa(dados, 0.5, 1) : 0;
      const raioBase = raioDoOrbe(graves);

      ctx.clearRect(0, 0, TAMANHO, TAMANHO);
      ctx.beginPath();
      for (let i = 0; i <= PONTOS_BORDA; i++) {
        const angulo = (i / PONTOS_BORDA) * Math.PI * 2;
        const ondulacao =
          Math.sin(angulo * 3 + fase * 1.6) * (4 + graves * 14) +
          Math.sin(angulo * 6 - fase * 1.1) * (3 + agudos * 10) +
          Math.cos(angulo * 2 + fase) * 3;
        const raio = raioBase + ondulacao;
        const x = centro + Math.cos(angulo) * raio;
        const y = centro + Math.sin(angulo) * raio;
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      }
      ctx.closePath();

      ctx.save();
      ctx.shadowColor = "rgba(0,0,0,0.85)";
      ctx.shadowBlur = 50 + graves * 60;
      const gradiente = ctx.createRadialGradient(centro, centro - 20, 20, centro, centro, raioBase + 20);
      gradiente.addColorStop(0, "#1a1a1f");
      gradiente.addColorStop(0.7, "#0a0a0c");
      gradiente.addColorStop(1, "#000000");
      ctx.fillStyle = gradiente;
      ctx.fill();
      ctx.restore();

      ctx.save();
      ctx.lineWidth = 2;
      ctx.strokeStyle = `rgba(120,140,255,${0.15 + graves * 0.4})`;
      ctx.shadowColor = "rgba(90,110,255,0.6)";
      ctx.shadowBlur = 18 + graves * 30;
      ctx.stroke();
      ctx.restore();

      quadro = requestAnimationFrame(desenhar);
    };

    desenhar();
    return () => cancelAnimationFrame(quadro);
  }, []);

  return (
    <canvas
      ref={canvasRef}
      style={{ width: TAMANHO, height: TAMANHO }}
      className="max-w-full shrink-0"
      aria-label="Visualização de áudio do James"
    />
  );
}
```

- [ ] **Passo 4: Rodar e ver passar**

```bash
npm test --prefix web -- shell/orbe
```

Esperado: 6 testes passando.

- [ ] **Passo 5: Commit**

```bash
git add web/src/shell/orbe.tsx web/src/shell/orbe.test.tsx
git commit -m "feat(web): orbe deforma conforme graves e agudos do analisador"
```

---

## Task 9: Ligar o áudio real ao orbe

Três armadilhas de falha silenciosa, todas da §5.1 da spec, ficam encapsuladas aqui.

**Arquivos:**
- Criar: `web/src/lib/audio.ts`
- Criar: `web/src/lib/audio.test.ts`
- Modificar: `web/src/App.tsx`

- [ ] **Passo 1: Escrever o teste que falha**

Crie `web/src/lib/audio.test.ts`:

```ts
import { describe, expect, it, vi } from "vitest";
import { criarAudioAnalisado } from "./audio";

function contextoFalso() {
  const analisador = { fftSize: 0, connect: vi.fn() };
  const fonte = { connect: vi.fn() };
  return {
    state: "running" as const,
    destination: { nome: "destino" },
    resume: vi.fn(),
    createAnalyser: vi.fn(() => analisador),
    createMediaElementSource: vi.fn(() => fonte),
    _analisador: analisador,
    _fonte: fonte,
  };
}

describe("criarAudioAnalisado", () => {
  it("liga elemento → fonte → analisador → destino", () => {
    const ctx = contextoFalso();

    const { analisador } = criarAudioAnalisado("/api/audio/x", ctx as unknown as AudioContext);

    expect(ctx.createMediaElementSource).toHaveBeenCalledTimes(1);
    expect(ctx._fonte.connect).toHaveBeenCalledWith(ctx._analisador);
    // Sem este connect o orbe anima em silêncio absoluto, e o sintoma
    // parece "o TTS quebrou" (R2).
    expect(ctx._analisador.connect).toHaveBeenCalledWith(ctx.destination);
    expect(analisador).toBe(ctx._analisador);
  });

  // createMediaElementSource só pode ser chamado uma vez por elemento (R1).
  // Criar o nó junto com o Audio é o que garante isso.
  it("cria um elemento e uma fonte novos a cada chamada", () => {
    const ctx = contextoFalso();

    const primeiro = criarAudioAnalisado("/api/audio/a", ctx as unknown as AudioContext);
    const segundo = criarAudioAnalisado("/api/audio/b", ctx as unknown as AudioContext);

    expect(ctx.createMediaElementSource).toHaveBeenCalledTimes(2);
    expect(primeiro.audio).not.toBe(segundo.audio);
  });

  it("retoma o contexto suspenso", () => {
    const ctx = { ...contextoFalso(), state: "suspended" as const };

    criarAudioAnalisado("/api/audio/x", ctx as unknown as AudioContext);

    expect(ctx.resume).toHaveBeenCalled();
  });
});
```

- [ ] **Passo 2: Rodar e ver falhar**

```bash
npm test --prefix web -- lib/audio
```

Esperado: FALHA com `Failed to resolve import "./audio"`.

- [ ] **Passo 3: Escrever o módulo**

Crie `web/src/lib/audio.ts`:

```ts
// Ponte entre o áudio do TTS e o orbe. Três armadilhas de falha silenciosa
// vivem aqui, e só aqui:
//
// 1. createMediaElementSource só pode ser chamado UMA VEZ por elemento. Por
//    isso o nó nasce junto com o Audio, nesta função, e nunca depois.
// 2. Roteado pelo grafo de áudio, o som só sai pelo destination. Esquecer
//    esse connect deixa o orbe animando em silêncio absoluto — e o sintoma
//    parece "o TTS parou de funcionar".
// 3. AudioContext nasce suspended sem gesto do usuário. Um contexto único
//    no app, com resume() aqui e no primeiro clique (ver despertarAudio).

let contextoUnico: AudioContext | null = null;

export function contextoDeAudio(): AudioContext {
  if (!contextoUnico) {
    const Construtor =
      window.AudioContext ?? (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
    contextoUnico = new Construtor();
  }
  return contextoUnico;
}

// despertarAudio é chamado no primeiro clique em qualquer lugar do app.
// Sem gesto do usuário o navegador mantém o contexto suspenso, e nem o
// áudio toca nem o analisador produz dado.
export function despertarAudio(): void {
  const ctx = contextoDeAudio();
  if (ctx.state === "suspended") void ctx.resume();
}

export interface AudioAnalisado {
  audio: HTMLAudioElement;
  analisador: AnalyserNode;
}

export function criarAudioAnalisado(url: string, ctx: AudioContext = contextoDeAudio()): AudioAnalisado {
  if (ctx.state === "suspended") void ctx.resume();

  const audio = new Audio(url);
  // Same-origin: o jamesd serve /api/audio e a SPA. Sem isso o
  // createMediaElementSource devolveria silêncio por CORS.
  audio.crossOrigin = "anonymous";

  const fonte = ctx.createMediaElementSource(audio);
  const analisador = ctx.createAnalyser();
  analisador.fftSize = 128;

  fonte.connect(analisador);
  analisador.connect(ctx.destination);

  return { audio, analisador };
}
```

- [ ] **Passo 4: Rodar e ver passar**

```bash
npm test --prefix web -- lib/audio
```

Esperado: 3 testes passando.

- [ ] **Passo 5: Ligar no `App.tsx`**

Em `web/src/App.tsx`:

Acrescente ao topo:

```tsx
import { criarAudioAnalisado, despertarAudio } from "./lib/audio";
```

Acrescente o estado do analisador, junto dos outros `useState`:

```tsx
const [analisador, setAnalisador] = useState<AnalyserNode | null>(null);
```

Substitua **todo** o manipulador de `audio_pronto` por:

```tsx
    cliente.on("audio_pronto", (dados: { id: string }) => {
      const excluirAudio = () => {
        fetch(`/api/audio/${dados.id}`, { method: "DELETE" }).catch(() => {});
      };

      let audio: HTMLAudioElement;
      let analisadorDoAudio: AnalyserNode | null = null;
      try {
        const analisado = criarAudioAnalisado(`/api/audio/${dados.id}`);
        audio = analisado.audio;
        analisadorDoAudio = analisado.analisador;
      } catch (erro) {
        // Web Audio indisponível (WebView antigo, política restritiva): o
        // áudio ainda deve tocar, só o orbe fica em repouso. console.warn
        // (não silencioso) segue o padrão de ws.ts: um catch mudo aqui
        // esconderia um bug de programação real atrás do mesmo rótulo de
        // "navegador não suporta".
        console.warn("Web Audio indisponível, orbe fica em repouso:", erro);
        audio = new Audio(`/api/audio/${dados.id}`);
      }

      setAnalisador(analisadorDoAudio);
      const encerrar = () => {
        setAnalisador(null);
        excluirAudio();
      };
      audio.addEventListener("ended", encerrar);
      // Autoplay pode ser bloqueado sem gesto recente do usuário — se falhar,
      // o áudio só não toca, mas o arquivo ainda precisa sair do disco.
      audio.play().catch(encerrar);
    });
```

E substitua **todo** o `return` final por (o texto anterior deste plano tinha `paineis={{ /* ...igual... */ }}` — um vácuo real; a versão abaixo é o `App.tsx` completo depois da Task 5, com `analisador={null}` trocado por `analisador={analisador}` e tudo envolvido no despertador do contexto de áudio):

```tsx
  return (
    <div className="contents" onClick={despertarAudio}>
      <Aplicativo
        aprovacoesPendentes={estado.aprovacao ? 1 : 0}
        paineis={{
          james: <AbaJames estado={estado} saidaVerTela={saidaVerTela} aoDecidir={decidir} analisador={analisador} />,
          mensagens: (
            <AbaMensagens
              estado={estado}
              conexao={conexao}
              tentativa={tentativa}
              aoEnviar={enviar}
              aoNovaSessao={novaSessao}
            />
          ),
          memoria: <Memoria />,
          historico: <p className="p-8 text-sm text-app-muted">Histórico chega na Task 13.</p>,
          dashboard: <AbaDashboard />,
        }}
      />
    </div>
  );
```

- [ ] **Passo 6: Rodar a suíte e o build**

```bash
npm test --prefix web && npm run build --prefix web
```

Esperado: tudo verde e build sem erro.

- [ ] **Passo 7: Commit**

```bash
git add web/src/lib/audio.ts web/src/lib/audio.test.ts web/src/App.tsx
git commit -m "feat(web): roteia o audio do TTS por um analisador e alimenta o orbe"
```

---

## Task 10: Migração 0002

**Arquivos:**
- Criar: `internal/armazenamento/migracoes/0002_resumo_sessao.sql`
- Modificar: `internal/armazenamento/postgres/sessao.go` (campos na struct)
- Teste: `internal/armazenamento/postgres/migrar_test.go`

- [ ] **Passo 1: Escrever o teste que falha**

Acrescente a `internal/armazenamento/postgres/migrar_test.go`:

```go
// A 0002 roda em banco que já tem dados — o do Arthur tem mais de mil
// sessões. DEFAULT '' e NULL permitido são o que tornam isso seguro (R10).
func TestMigracao0002PreservaSessaoExistente(t *testing.T) {
	pool := abrirPoolMigrado(t)
	ctx := context.Background()

	repo := NovoSessaoRepositorio(pool)
	sessao, err := repo.Criar(ctx, "conversa anterior à 0002")
	if err != nil {
		t.Fatalf("erro ao criar sessão: %v", err)
	}

	// Reaplicar é no-op (AplicarMigracoes é idempotente), mas o que
	// interessa é o estado das colunas novas numa linha que já existia.
	if err := AplicarMigracoes(ctx, pool); err != nil {
		t.Fatalf("erro ao reaplicar migrações: %v", err)
	}

	var resumo string
	var resumidoAte *int64
	err = pool.QueryRow(ctx, `SELECT resumo, resumido_ate FROM sessoes WHERE id=$1`, sessao.ID).
		Scan(&resumo, &resumidoAte)
	if err != nil {
		t.Fatalf("erro ao ler colunas novas: %v", err)
	}
	if resumo != "" {
		t.Fatalf("resumo devia nascer vazio, veio %q", resumo)
	}
	if resumidoAte != nil {
		t.Fatalf("resumido_ate devia nascer NULL, veio %d", *resumidoAte)
	}
}
```

- [ ] **Passo 2: Rodar e ver falhar**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && go test ./internal/armazenamento/postgres/ -run TestMigracao0002 -v
```

Esperado: FALHA com `column "resumo" does not exist`.

- [ ] **Passo 3: Escrever a migração**

Crie `internal/armazenamento/migracoes/0002_resumo_sessao.sql`:

```sql
-- Resumo rolante da conversa (Fase 7A, §7 da spec).
--
-- resumido_ate guarda o mensagens.id da última linha já dobrada no resumo,
-- e NÃO uma contagem: depois de uma retomada o histórico em memória é só a
-- cauda dos 200 registros mais recentes (ws.go, limiteMensagensSessao), e
-- contagem sobre ele resumiria o trecho errado — em silêncio.
--
-- DEFAULT '' e NULL permitido para a migração ser segura em banco que já
-- tem sessões (o de trabalho tem mais de mil).
ALTER TABLE sessoes ADD COLUMN resumo TEXT NOT NULL DEFAULT '';
ALTER TABLE sessoes ADD COLUMN resumido_ate BIGINT;
```

- [ ] **Passo 4: Acrescentar os campos à struct**

Em `internal/armazenamento/postgres/sessao.go`, na struct `Sessao`:

```go
type Sessao struct {
	ID           uuid.UUID
	Titulo       string
	CriadaEm     time.Time
	AtualizadaEm time.Time
	Resumo       string
	// ResumidoAte é nil quando nada foi resumido ainda.
	ResumidoAte *int64
}
```

E acrescente as colunas ao `SELECT` de `BuscarPorID`:

```go
func (r *SessaoRepositorio) BuscarPorID(ctx context.Context, id uuid.UUID) (Sessao, error) {
	var s Sessao
	err := r.pool.QueryRow(ctx,
		`SELECT id, titulo, criada_em, atualizada_em, resumo, resumido_ate FROM sessoes WHERE id=$1`, id,
	).Scan(&s.ID, &s.Titulo, &s.CriadaEm, &s.AtualizadaEm, &s.Resumo, &s.ResumidoAte)
	if err != nil {
		return Sessao{}, fmt.Errorf("erro ao buscar sessão %s: %w", id, err)
	}
	return s, nil
}
```

- [ ] **Passo 5: Rodar e ver passar**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && go test ./internal/armazenamento/postgres/ -v
```

Esperado: todos os testes do pacote verdes, inclusive o novo.

- [ ] **Passo 6: Commit**

```bash
git add internal/armazenamento/migracoes/0002_resumo_sessao.sql internal/armazenamento/postgres/sessao.go internal/armazenamento/postgres/migrar_test.go
git commit -m "feat(banco): migracao 0002 com resumo e resumido_ate na sessao"
```

---

## Task 11: Repositório de sessão — paginação, busca e resumo

**Arquivos:**
- Modificar: `internal/armazenamento/postgres/sessao.go`
- Teste: `internal/armazenamento/postgres/sessao_test.go`

- [ ] **Passo 1: Escrever os testes que falham**

Acrescente a `internal/armazenamento/postgres/sessao_test.go`:

```go
func TestListarPaginadoRespeitaOLimite(t *testing.T) {
	pool := abrirPoolMigrado(t)
	ctx := context.Background()
	repo := NovoSessaoRepositorio(pool)

	marca := uuid.NewString()
	for i := 0; i < 5; i++ {
		if _, err := repo.Criar(ctx, fmt.Sprintf("%s pagina %d", marca, i)); err != nil {
			t.Fatalf("erro ao criar sessão: %v", err)
		}
	}

	pagina, err := repo.ListarPaginado(ctx, FiltroSessoes{Limite: 3, Busca: marca})
	if err != nil {
		t.Fatalf("erro ao listar: %v", err)
	}
	if len(pagina) != 3 {
		t.Fatalf("esperava 3 sessões, veio %d", len(pagina))
	}
	// Mais recente primeiro.
	for i := 1; i < len(pagina); i++ {
		if pagina[i].AtualizadaEm.After(pagina[i-1].AtualizadaEm) {
			t.Fatalf("ordem errada: item %d é mais novo que o %d", i, i-1)
		}
	}
}

func TestListarPaginadoAvancaPeloCursor(t *testing.T) {
	pool := abrirPoolMigrado(t)
	ctx := context.Background()
	repo := NovoSessaoRepositorio(pool)

	marca := uuid.NewString()
	for i := 0; i < 5; i++ {
		if _, err := repo.Criar(ctx, fmt.Sprintf("%s cursor %d", marca, i)); err != nil {
			t.Fatalf("erro ao criar sessão: %v", err)
		}
	}

	primeira, err := repo.ListarPaginado(ctx, FiltroSessoes{Limite: 2, Busca: marca})
	if err != nil {
		t.Fatalf("erro na primeira página: %v", err)
	}
	cursor := primeira[len(primeira)-1].AtualizadaEm

	segunda, err := repo.ListarPaginado(ctx, FiltroSessoes{Limite: 2, Busca: marca, Antes: &cursor})
	if err != nil {
		t.Fatalf("erro na segunda página: %v", err)
	}
	if len(segunda) == 0 {
		t.Fatal("segunda página veio vazia")
	}
	for _, s := range segunda {
		for _, anterior := range primeira {
			if s.ID == anterior.ID {
				t.Fatalf("sessão %s repetiu entre as páginas", s.ID)
			}
		}
	}
}

func TestListarPaginadoFiltraPorTitulo(t *testing.T) {
	pool := abrirPoolMigrado(t)
	ctx := context.Background()
	repo := NovoSessaoRepositorio(pool)

	marca := uuid.NewString()
	if _, err := repo.Criar(ctx, marca+" abre a CALCULADORA"); err != nil {
		t.Fatalf("erro ao criar: %v", err)
	}
	if _, err := repo.Criar(ctx, marca+" pesquisa sobre Go"); err != nil {
		t.Fatalf("erro ao criar: %v", err)
	}

	// ILIKE: buscar minúsculo tem que achar o título maiúsculo.
	achadas, err := repo.ListarPaginado(ctx, FiltroSessoes{Limite: 10, Busca: "calculadora"})
	if err != nil {
		t.Fatalf("erro ao buscar: %v", err)
	}
	var encontrou bool
	for _, s := range achadas {
		if strings.Contains(s.Titulo, marca) {
			encontrou = true
		}
		if strings.Contains(s.Titulo, "pesquisa sobre Go") {
			t.Fatal("a busca trouxe sessão que não bate com o termo")
		}
	}
	if !encontrou {
		t.Fatal("a busca não achou a sessão que bate com o termo")
	}
}

func TestAtualizarResumoGravaOsDoisCampos(t *testing.T) {
	pool := abrirPoolMigrado(t)
	ctx := context.Background()
	repo := NovoSessaoRepositorio(pool)

	sessao, err := repo.Criar(ctx, "conversa longa")
	if err != nil {
		t.Fatalf("erro ao criar: %v", err)
	}

	if err := repo.AtualizarResumo(ctx, sessao.ID, "o Arthur pediu o clima e depois a agenda", 42); err != nil {
		t.Fatalf("erro ao atualizar resumo: %v", err)
	}

	recarregada, err := repo.BuscarPorID(ctx, sessao.ID)
	if err != nil {
		t.Fatalf("erro ao recarregar: %v", err)
	}
	if recarregada.Resumo != "o Arthur pediu o clima e depois a agenda" {
		t.Fatalf("resumo não gravou: %q", recarregada.Resumo)
	}
	if recarregada.ResumidoAte == nil || *recarregada.ResumidoAte != 42 {
		t.Fatalf("resumido_ate não gravou: %v", recarregada.ResumidoAte)
	}
}
```

Acrescente aos imports do arquivo, se faltarem: `fmt`, `strings`, `github.com/google/uuid`.

- [ ] **Passo 2: Rodar e ver falhar**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && go test ./internal/armazenamento/postgres/ -run "TestListarPaginado|TestAtualizarResumo"
```

Esperado: FALHA de compilação — `FiltroSessoes`, `ListarPaginado` e `AtualizarResumo` não existem.

- [ ] **Passo 3: Implementar**

Em `internal/armazenamento/postgres/sessao.go`, **substitua** o método `Listar` por:

```go
// FiltroSessoes descreve uma página da lista de sessões. Antes é o cursor
// keyset: devolve só o que for MAIS ANTIGO que ele. Keyset e não OFFSET
// porque a lista é ordenada por atividade recente e muda embaixo de quem
// rola — com OFFSET, uma sessão que recebe mensagem durante a rolagem faz
// item pular ou repetir.
type FiltroSessoes struct {
	Limite int
	Antes  *time.Time
	Busca  string
}

const limiteMaximoSessoes = 200

// ListarPaginado devolve uma página de sessões, mais recente primeiro.
// Substituiu o Listar() sem limite, que devolvia a tabela inteira — a aba
// Sessões chegou a renderizar 1646 botões porque a suíte de teste escreve
// no banco de trabalho e quase nunca limpa.
func (r *SessaoRepositorio) ListarPaginado(ctx context.Context, filtro FiltroSessoes) ([]Sessao, error) {
	limite := filtro.Limite
	if limite <= 0 || limite > limiteMaximoSessoes {
		limite = 50
	}

	consulta := `SELECT id, titulo, criada_em, atualizada_em, resumo, resumido_ate
	             FROM sessoes WHERE ($1::timestamptz IS NULL OR atualizada_em < $1)
	               AND ($2 = '' OR titulo ILIKE '%' || $2 || '%')
	             ORDER BY atualizada_em DESC LIMIT $3`

	linhas, err := r.pool.Query(ctx, consulta, filtro.Antes, filtro.Busca, limite)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar sessões: %w", err)
	}
	defer linhas.Close()

	sessoes := []Sessao{}
	for linhas.Next() {
		var s Sessao
		if err := linhas.Scan(&s.ID, &s.Titulo, &s.CriadaEm, &s.AtualizadaEm, &s.Resumo, &s.ResumidoAte); err != nil {
			return nil, err
		}
		sessoes = append(sessoes, s)
	}
	return sessoes, linhas.Err()
}

// AtualizarResumo grava o resumo rolante e até onde ele cobre. Não mexe em
// atualizada_em de propósito: resumir não é atividade da conversa, e mexer
// ali reordenaria a lista do Histórico sem o Arthur ter feito nada.
func (r *SessaoRepositorio) AtualizarResumo(ctx context.Context, id uuid.UUID, resumo string, resumidoAte int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE sessoes SET resumo=$2, resumido_ate=$3 WHERE id=$1`, id, resumo, resumidoAte)
	if err != nil {
		return fmt.Errorf("erro ao atualizar resumo da sessão %s: %w", id, err)
	}
	return nil
}
```

> `Listar()` sai. O compilador vai apontar quem chamava. **São dois chamadores, não um**: `internal/api/rest.go:78` (a Task 12 conserta) e `cmd/migrar-historico/main.go:50` — este segundo não estava registrado neste plano antes desta revisão, e a Task 12 não toca nele. Sem o Passo 3.5 abaixo, o build da raiz continuaria quebrado mesmo depois da Task 12.

- [ ] **Passo 3.5: Consertar o segundo chamador, `cmd/migrar-historico/main.go`**

Essa é uma ferramenta de ETL de execução única (SQLite → Postgres, já rodada historicamente), mas ela ainda precisa compilar — o `go vet ./... && go test ./...` deste projeto roda sobre o módulo inteiro. `executar()` usa `Listar(ctx)` para checar, de forma idempotente, se a sessão de histórico (`"Histórico Athena"`) já existe, varrendo todas as sessões e comparando título.

Em `cmd/migrar-historico/main.go:50`, troque:

```go
	existentes, err := sessoes.Listar(ctx)
```

por:

```go
	// Busca em vez de listar tudo: Listar() saiu (Fase 7A, Task 11) e
	// ListarPaginado com filtro de título é mais barato que paginar o banco
	// inteiro só pra achar uma sessão com nome conhecido.
	existentes, err := sessoes.ListarPaginado(ctx, postgres.FiltroSessoes{Busca: tituloSessaoHistorico})
```

O laço logo abaixo (`for _, s := range existentes { if s.Titulo == tituloSessaoHistorico {...} }`) não muda — ele continua fazendo a comparação exata dentro do conjunto (agora menor) que a busca traz. `ILIKE` é substring/case-insensitive, então o filtro pode trazer falso-positivo largo em teoria; a comparação exata que já existia é o que resolve isso, sem precisar mudar a lógica de decisão.

Confirme que compila:

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && go build ./...
```

Esperado: sem erro em `cmd/migrar-historico`. (`rest.go` ainda vai falhar — isso é esperado até a Task 12, e é o único erro que deve sobrar.)

**Um QUARTO chamador, achado só na execução real desta task**: `go build` não pega arquivo `_test.go`, e `cmd/migrar-historico/main_test.go` tem **três** usos de `sessoes.Listar(...)` (`TestExecutarMigraMensagensEMapeiaPapel`, `TestExecutarEIdempotente`, `TestExecutarFalhaNoMeioNaoDeixaSessaoParcial`) — só `go vet ./...`/`go test ./...` na raiz acusam. Mesmo padrão exato do `main.go`: listam e depois comparam `s.Titulo == tituloSessaoHistorico` num loop. Aplique a mesma troca nos três pontos — `sessoes.Listar(context.Background())` vira `sessoes.ListarPaginado(context.Background(), postgres.FiltroSessoes{Busca: tituloSessaoHistorico})`, preservando se a linha original descartava o erro (`_`) ou não. O loop de comparação exata não muda em nenhum dos três. Depois, `go vet ./...` não deve apontar mais nada em `cmd/migrar-historico`.

- [ ] **Passo 4: Rodar e ver passar**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && go test ./internal/armazenamento/postgres/ -v
```

Esperado: os quatro testes novos passando. `go build ./...` ainda falha **só** em `rest.go` — é esperado e a Task 12 resolve.

- [ ] **Passo 5: Commit**

```bash
git add internal/armazenamento/postgres/sessao.go internal/armazenamento/postgres/sessao_test.go cmd/migrar-historico/main.go
git commit -m "feat(banco): paginacao keyset, busca por titulo e gravacao de resumo na sessao"
```

---

## Task 12: `GET /api/sessoes` com limite, cursor e busca

**Arquivos:**
- Modificar: `internal/api/rest.go`
- Modificar: `web/src/lib/api.ts`
- Teste: `internal/api/rest_test.go`

- [ ] **Passo 1: Escrever o teste que falha**

Acrescente a `internal/api/rest_test.go`. O helper `servidorDeTeste(t)` já existe no arquivo (linha 30) e devolve um `*Servidor` com os repositórios ligados ao Postgres real:

```go
func TestListarSessoesAceitaLimiteCursorEBusca(t *testing.T) {
	s := servidorDeTeste(t)
	ctx := context.Background()

	// Marca única no título: a suíte escreve no banco de trabalho, que já
	// tem mais de mil sessões de execuções anteriores. Sem a marca, a busca
	// pegaria lixo das outras rodadas.
	marca := uuid.NewString()
	for i := 0; i < 4; i++ {
		if _, err := s.Sessoes.Criar(ctx, fmt.Sprintf("%s item %d", marca, i)); err != nil {
			t.Fatalf("erro ao criar sessão: %v", err)
		}
	}

	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	resp, err := http.Get(servidor.URL + "/api/sessoes?limite=2&busca=" + marca)
	if err != nil {
		t.Fatalf("erro na requisição: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var pagina []postgres.Sessao
	if err := json.NewDecoder(resp.Body).Decode(&pagina); err != nil {
		t.Fatalf("json ilegível: %v", err)
	}
	if len(pagina) != 2 {
		t.Fatalf("limite não foi respeitado: vieram %d", len(pagina))
	}
}

func TestListarSessoesRejeitaCursorInvalido(t *testing.T) {
	s := servidorDeTeste(t)
	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	resp, err := http.Get(servidor.URL + "/api/sessoes?antes=ontem")
	if err != nil {
		t.Fatalf("erro na requisição: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("cursor inválido devia dar 400, deu %d", resp.StatusCode)
	}
}
```

Acrescente `"fmt"` aos imports de `rest_test.go` — o arquivo hoje não importa `fmt`, e o teste acima usa `fmt.Sprintf`. `uuid` e `postgres` já estão importados.

- [ ] **Passo 2: Rodar e ver falhar**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && go test ./internal/api/ -run TestListarSessoes
```

Esperado: FALHA de compilação em `rest.go` (o `Listar` não existe mais) — que é justamente o que precisa ser consertado.

- [ ] **Passo 3: Reescrever o handler**

Em `internal/api/rest.go`, substitua `listarSessoes` por:

```go
func (s *Servidor) listarSessoes(w http.ResponseWriter, r *http.Request) {
	filtro := postgres.FiltroSessoes{Busca: r.URL.Query().Get("busca")}

	if bruto := r.URL.Query().Get("limite"); bruto != "" {
		limite, err := strconv.Atoi(bruto)
		if err != nil {
			http.Error(w, "limite inválido", http.StatusBadRequest)
			return
		}
		filtro.Limite = limite
	}

	// O cursor vem em RFC 3339, que é o formato em que atualizada_em sai
	// deste mesmo endpoint — o cliente devolve o que recebeu, sem converter.
	if bruto := r.URL.Query().Get("antes"); bruto != "" {
		antes, err := time.Parse(time.RFC3339Nano, bruto)
		if err != nil {
			http.Error(w, "cursor inválido", http.StatusBadRequest)
			return
		}
		filtro.Antes = &antes
	}

	sessoes, err := s.Sessoes.ListarPaginado(r.Context(), filtro)
	if err != nil {
		// Erro cru aqui carregaria host, banco e usuário para uma tela que
		// o celular abre pela VPN — mesma regra de mensagemDeErroDoTurno.
		slog.Error("erro ao listar sessões", "erro", err)
		http.Error(w, "não consegui listar as conversas", http.StatusInternalServerError)
		return
	}
	responderJSON(w, http.StatusOK, sessoes)
}
```

Acrescente `"strconv"` aos imports de `rest.go`.

- [ ] **Passo 4: Rodar e ver passar**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && go vet ./... && go test ./internal/api/
```

Esperado: pacote verde.

**Um terceiro furo, achado na execução real desta task**: `internal/api` é **um pacote só**, e `ws_test.go:847,1077` e `dispositivo_test.go:382` também chamam `s.Sessoes.Listar(ctx)` — mesmo padrão exato do 2º/4º chamador que a Task 11 já consertou (listam tudo, comparam `sess.Titulo == textoUnico` num loop, onde `textoUnico` é a marca única do teste). Como estão no mesmo pacote do teste desta task, nem os testes novos do Passo 1 compilam sem isso. Troque nas três linhas:

```go
	todas, err := s.Sessoes.Listar(ctx)
```

por:

```go
	todas, err := s.Sessoes.ListarPaginado(ctx, postgres.FiltroSessoes{Busca: textoUnico})
```

O loop de comparação exata (`if sess.Titulo == textoUnico`) não muda em nenhum dos três pontos. Confirme que `postgres` já está importado nos dois arquivos (deve estar, ambos já usam tipos de `postgres` em outros lugares).

- [ ] **Passo 5: Atualizar o cliente do front**

Em `web/src/lib/api.ts`, acrescente os campos novos à interface `Sessao` e troque `listarSessoes`:

```ts
export interface Sessao {
  ID: string;
  Titulo: string;
  CriadaEm: string;
  AtualizadaEm: string;
  Resumo: string;
  ResumidoAte: number | null;
}

export interface PaginaSessoes {
  limite?: number;
  // Cursor keyset: o AtualizadaEm do último item da página anterior.
  antes?: string;
  busca?: string;
}

export async function listarSessoes(pagina: PaginaSessoes = {}): Promise<Sessao[]> {
  const parametros = new URLSearchParams();
  if (pagina.limite) parametros.set("limite", String(pagina.limite));
  if (pagina.antes) parametros.set("antes", pagina.antes);
  if (pagina.busca) parametros.set("busca", pagina.busca);

  // web/src/lib/api.test.ts:18 (pré-existente, não citado neste plano antes
  // desta correção) espera literalmente "/api/sessoes", sem "?", na chamada
  // sem argumentos. `fetch(`/api/sessoes?${parametros}`)` sempre grudava um
  // "?" mesmo com URLSearchParams vazio — só concatena quando há query de
  // verdade.
  const query = parametros.toString();
  const resp = await fetch(`/api/sessoes${query ? `?${query}` : ""}`);
  if (!resp.ok) throw new Error(`erro ao listar sessões: ${resp.status}`);
  return resp.json();
}
```

- [ ] **Passo 5.5: `web/src/abas/sessoes.tsx` ainda existe — testei e `Sessao` com campos novos quebra o build dele**

`abas/sessoes.tsx` só morre na Task 13, não nesta. Ele importa `Sessao` de `lib/api.ts`, e `sessoes.test.tsx` constrói fixtures à mão (`SESSOES`) sem os campos `Resumo`/`ResumidoAte`. Testei isolado antes de escrever este passo: com a interface `Sessao` do Passo 5, `npx tsc --noEmit` acusa

```
src/abas/sessoes.test.tsx(18,48): error TS2345: ... missing the following properties from type 'Sessao': Resumo, ResumidoAte
```

nas três chamadas de `mockResolvedValue(SESSOES)`. Em `web/src/abas/sessoes.test.tsx`, acrescente os dois campos aos dois objetos do array `SESSOES`:

```ts
const SESSOES = [
  {
    ID: "11111111-1111-1111-1111-111111111111",
    Titulo: "abre a calculadora",
    CriadaEm: "2026-08-08T22:00:00Z",
    AtualizadaEm: "2026-08-08T22:05:00Z",
    Resumo: "",
    ResumidoAte: null,
  },
  {
    ID: "22222222-2222-2222-2222-222222222222",
    Titulo: "pesquisa sobre Go",
    CriadaEm: "2026-08-07T10:00:00Z",
    AtualizadaEm: "2026-08-07T10:30:00Z",
    Resumo: "",
    ResumidoAte: null,
  },
];
```

Só isso — o componente `sessoes.tsx` em si não referencia `Resumo`/`ResumidoAte`, então não precisa mudar. Este arquivo inteiro morre na Task 13; o patch é temporário de propósito, não vale mais esforço que isso.

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && npm run build --prefix web
```

Esperado: limpo, sem os três `TS2345`.

Antes de commitar, confirme que `go vet ./... && go test ./internal/api/...` está limpo (o terceiro furo, acima, faz parte do mesmo pacote) e que `npm test --prefix web` está 100% verde (o quarto furo — `api.test.ts:18` — vive no mesmo pacote do Passo 5).

- [ ] **Passo 6: Commit**

```bash
git add internal/api/rest.go internal/api/rest_test.go internal/api/ws_test.go internal/api/dispositivo_test.go web/src/lib/api.ts web/src/abas/sessoes.test.tsx
git commit -m "feat(api): GET /api/sessoes com limite, cursor keyset e busca no servidor"
```

---

## Task 13: Aba Histórico

**Arquivos:**
- Criar: `web/src/abas/historico.tsx`
- Teste: `web/src/abas/historico.test.tsx`
- Apagar: `web/src/abas/sessoes.tsx`, `web/src/abas/sessoes.test.tsx`
- Modificar: `web/src/App.tsx`

- [ ] **Passo 1: Escrever o teste que falha**

Crie `web/src/abas/historico.test.tsx`:

```tsx
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AbaHistorico, preview } from "./historico";

vi.mock("../lib/api", () => ({
  listarSessoes: vi.fn(),
}));

import { listarSessoes } from "../lib/api";

// Sem isto, mock.calls acumula entre os `it`s (o projeto não liga
// clearMocks/resetMocks em lugar nenhum — nem vite.config.ts, nem
// vitest.setup.ts) e o teste "carrega mais páginas pelo cursor" indexaria
// mock.calls[1] esperando a 2ª chamada DESTE teste, quando na prática seria
// a 2ª chamada do ARQUIVO inteiro.
beforeEach(() => {
  vi.clearAllMocks();
});

const HOJE = new Date();
const ONTEM = new Date(HOJE.getTime() - 24 * 60 * 60 * 1000);

function sessao(id: string, titulo: string, quando: Date) {
  return {
    ID: id,
    Titulo: titulo,
    CriadaEm: quando.toISOString(),
    AtualizadaEm: quando.toISOString(),
    Resumo: "",
    ResumidoAte: null,
  };
}

describe("Aba Histórico", () => {
  it("agrupa por dia", async () => {
    vi.mocked(listarSessoes).mockResolvedValue([
      sessao("1", "abre a calculadora", HOJE),
      sessao("2", "pesquisa sobre Go", ONTEM),
    ]);

    render(<AbaHistorico sessaoAtual={null} aoRetomar={vi.fn()} />);

    await waitFor(() => expect(screen.getByText("Hoje")).toBeTruthy());
    expect(screen.getByText("Ontem")).toBeTruthy();
  });

  it("retoma a sessão clicada", async () => {
    vi.mocked(listarSessoes).mockResolvedValue([sessao("1", "abre a calculadora", HOJE)]);
    const aoRetomar = vi.fn();

    render(<AbaHistorico sessaoAtual={null} aoRetomar={aoRetomar} />);

    await waitFor(() => screen.getByText("abre a calculadora"));
    fireEvent.click(screen.getByText("abre a calculadora"));

    expect(aoRetomar).toHaveBeenCalledWith("1");
  });

  // Busca no cliente com paginação mostraria resultado incompleto e
  // pareceria bug (D11).
  it("manda a busca para o servidor", async () => {
    vi.mocked(listarSessoes).mockResolvedValue([]);

    render(<AbaHistorico sessaoAtual={null} aoRetomar={vi.fn()} />);
    await waitFor(() => expect(listarSessoes).toHaveBeenCalled());

    fireEvent.change(screen.getByPlaceholderText("Buscar conversas…"), { target: { value: "calc" } });

    await waitFor(() =>
      expect(vi.mocked(listarSessoes).mock.calls.some(([arg]) => arg?.busca === "calc")).toBe(true),
    );
  });

  it("carrega mais páginas pelo cursor", async () => {
    const primeira = Array.from({ length: 50 }, (_, i) => sessao(String(i), `conversa ${i}`, HOJE));
    vi.mocked(listarSessoes).mockResolvedValueOnce(primeira).mockResolvedValueOnce([
      sessao("50", "conversa 50", ONTEM),
    ]);

    render(<AbaHistorico sessaoAtual={null} aoRetomar={vi.fn()} />);
    await waitFor(() => screen.getByText("conversa 0"));

    fireEvent.click(screen.getByRole("button", { name: "Carregar mais" }));

    await waitFor(() => expect(screen.getByText("conversa 50")).toBeTruthy());
    // A segunda chamada tem que levar o cursor da última linha da primeira.
    // mock.calls[1] só é a 2ª chamada DESTE teste porque o beforeEach abaixo
    // limpa o mock a cada `it` — sem ele, é a 2ª chamada do ARQUIVO inteiro
    // (o projeto não liga clearMocks/resetMocks em lugar nenhum), e o índice
    // pega a chamada errada. Achado rodando a suíte inteira: passava isolado,
    // falhava 5/5 no arquivo completo — determinístico, não flake.
    expect(vi.mocked(listarSessoes).mock.calls[1][0]?.antes).toBe(primeira[49].AtualizadaEm);
  });

  it("mostra o erro sem quebrar a aba", async () => {
    vi.mocked(listarSessoes).mockRejectedValue(new Error("sem rede"));

    render(<AbaHistorico sessaoAtual={null} aoRetomar={vi.fn()} />);

    await waitFor(() => expect(screen.getByRole("status")).toBeTruthy());
    expect(screen.getByRole("button", { name: "Tentar de novo" })).toBeTruthy();
  });

  // Sem guarda de geração: busca e "carregar mais" digitados/clicados perto
  // um do outro competem, e se a rede devolver fora de ordem (paginação
  // atrasada resolve DEPOIS da busca nova), o append da paginação grudaria
  // resultado do termo velho em cima da lista do termo novo — misturando
  // duas buscas na tela, sem erro nenhum visível pro usuário.
  it("descarta resposta antiga quando busca nova chega antes da paginação pendente", async () => {
    const primeira = Array.from({ length: 50 }, (_, i) => sessao(String(i), `conversa ${i}`, HOJE));
    let resolverPaginacao: (v: unknown) => void = () => {};
    const paginacaoPendente = new Promise((resolve) => {
      resolverPaginacao = resolve;
    });

    vi.mocked(listarSessoes)
      .mockResolvedValueOnce(primeira) // carga inicial
      .mockImplementationOnce(() => paginacaoPendente as ReturnType<typeof listarSessoes>) // "carregar mais", fica pendente
      .mockResolvedValueOnce([sessao("nova", "resultado da busca nova", HOJE)]); // busca nova, resolve rápido

    render(<AbaHistorico sessaoAtual={null} aoRetomar={vi.fn()} />);
    await waitFor(() => screen.getByText("conversa 0"));

    fireEvent.click(screen.getByRole("button", { name: "Carregar mais" }));
    await waitFor(() => expect(listarSessoes).toHaveBeenCalledTimes(2));

    fireEvent.change(screen.getByPlaceholderText("Buscar conversas…"), { target: { value: "nova" } });
    await waitFor(() => expect(screen.getByText("resultado da busca nova")).toBeTruthy());

    // A paginação atrasada só resolve DEPOIS da busca nova já ter atualizado a tela.
    resolverPaginacao([{ ...primeira[0], ID: "atrasada", Titulo: "pagina atrasada" }]);
    await new Promise((r) => setTimeout(r, 20));

    expect(screen.queryByText("pagina atrasada")).toBeNull();
    expect(screen.getByText("resultado da busca nova")).toBeTruthy();
  });
});

describe("preview", () => {
  it("não corta um emoji ao meio quando o título passa de 80 caracteres", () => {
    // Cada emoji fora do BMP ocupa 2 code units UTF-16 — um slice ingênuo em
    // string partiria o 40º emoji ao meio e produziria par substituto solto.
    const tituloLongo = "🎉".repeat(85);
    const resultado = preview(tituloLongo);
    expect(Array.from(resultado.replace("…", "")).length).toBe(80);
    expect(resultado.includes("�")).toBe(false);
  });
});
```

- [ ] **Passo 2: Rodar e ver falhar**

```bash
npm test --prefix web -- abas/historico
```

Esperado: FALHA com `Failed to resolve import "./historico"`.

- [ ] **Passo 3: Escrever a aba**

Crie `web/src/abas/historico.tsx`:

```tsx
import { useCallback, useEffect, useRef, useState } from "react";
import { Clock, Hourglass, Search } from "lucide-react";
import { listarSessoes, type Sessao } from "../lib/api";

const POR_PAGINA = 50;

interface Props {
  sessaoAtual: string | null;
  aoRetomar: (id: string) => void;
}

// rotuloDoDia agrupa como o design de referência: Hoje, Ontem, depois data.
export function rotuloDoDia(iso: string, agora = new Date()): string {
  const data = new Date(iso);
  const dia = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  const diferenca = (dia(agora) - dia(data)) / 86_400_000;
  if (diferenca === 0) return "Hoje";
  if (diferenca === 1) return "Ontem";
  return data.toLocaleDateString("pt-BR", { day: "2-digit", month: "short" });
}

// O título é a primeira mensagem inteira do usuário (ws.go: Criar(ctx, texto)),
// sem truncar na origem. O corte é aqui, na exibição. Array.from (não
// slice/length direto) porque títulos vêm de mensagem livre do usuário e
// podem ter emoji fora do BMP (2 code units UTF-16 cada) — slice ingênuo
// partiria um desses ao meio e renderizaria par substituto solto.
export function preview(titulo: string): string {
  const limpo = titulo.trim().replace(/\s+/g, " ");
  const caracteres = Array.from(limpo);
  return caracteres.length > 80 ? `${caracteres.slice(0, 80).join("")}…` : limpo || "(sem título)";
}

export function AbaHistorico({ sessaoAtual, aoRetomar }: Props) {
  const [sessoes, setSessoes] = useState<Sessao[]>([]);
  const [busca, setBusca] = useState("");
  const [carregando, setCarregando] = useState(true);
  const [acabou, setAcabou] = useState(false);
  const [falha, setFalha] = useState("");
  // Descarta resposta desatualizada quando busca e paginação correm ao
  // mesmo tempo e a rede devolve fora de ordem: cada carregar() incrementa
  // a geração, e só a resposta da geração mais recente atualiza o estado.
  // Sem isso, uma paginação atrasada (termo velho) que resolve DEPOIS de
  // uma busca nova grudaria resultado do termo velho em cima da lista do
  // termo novo — sem erro nenhum visível pro usuário.
  const geracaoRef = useRef(0);

  const carregar = useCallback(
    async (termo: string, cursor?: string) => {
      const geracao = ++geracaoRef.current;
      setCarregando(true);
      setFalha("");
      try {
        const pagina = await listarSessoes({ limite: POR_PAGINA, busca: termo || undefined, antes: cursor });
        if (geracao !== geracaoRef.current) return;
        setSessoes((anteriores) => (cursor ? [...anteriores, ...pagina] : pagina));
        setAcabou(pagina.length < POR_PAGINA);
      } catch (e) {
        if (geracao !== geracaoRef.current) return;
        setFalha((e as Error).message);
      } finally {
        if (geracao === geracaoRef.current) setCarregando(false);
      }
    },
    [],
  );

  // Debounce para a busca não disparar uma requisição por tecla.
  useEffect(() => {
    const timer = setTimeout(() => {
      void carregar(busca);
    }, 250);
    return () => clearTimeout(timer);
  }, [busca, carregar]);

  const carregarMais = () => {
    const ultima = sessoes[sessoes.length - 1];
    if (ultima) void carregar(busca, ultima.AtualizadaEm);
  };

  const dias: string[] = [];
  for (const s of sessoes) {
    const dia = rotuloDoDia(s.AtualizadaEm);
    if (!dias.includes(dia)) dias.push(dia);
  }

  return (
    <div className="flex h-full flex-col">
      <header className="space-y-4 border-b border-app-line px-6 py-5">
        <div className="flex items-center gap-2">
          <Hourglass className="size-5 text-app-icon" />
          <h2 className="font-serif text-xl text-app-text">Histórico</h2>
        </div>
        <div className="flex items-center gap-2 rounded-lg bg-white/5 px-3 py-2">
          <Search className="size-4 text-app-muted" />
          <input
            value={busca}
            onChange={(e) => setBusca(e.target.value)}
            placeholder="Buscar conversas…"
            className="flex-1 bg-transparent text-sm text-app-text placeholder:text-app-muted focus:outline-none"
          />
        </div>
      </header>

      <div className="flex-1 overflow-y-auto px-6 py-5">
        {falha && (
          <div className="space-y-2">
            <p role="status" className="text-sm text-amber-300">
              {falha}
            </p>
            <button
              type="button"
              onClick={() => void carregar(busca)}
              className="rounded-lg bg-white/5 px-3 py-1.5 text-xs text-app-text hover:bg-app-hover"
            >
              Tentar de novo
            </button>
          </div>
        )}

        {dias.map((dia) => (
          <div key={dia} className="mb-6">
            <p className="mb-2 text-xs font-medium uppercase tracking-wide text-app-muted">{dia}</p>
            <ul className="space-y-1">
              {sessoes
                .filter((s) => rotuloDoDia(s.AtualizadaEm) === dia)
                .map((s) => (
                  <li key={s.ID}>
                    <button
                      type="button"
                      aria-current={s.ID === sessaoAtual ? "true" : undefined}
                      onClick={() => aoRetomar(s.ID)}
                      className={`flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left transition-colors hover:bg-app-hover ${
                        s.ID === sessaoAtual ? "bg-app-hover" : ""
                      }`}
                    >
                      <span className="grid size-8 shrink-0 place-items-center rounded-md bg-white/5 text-app-muted">
                        <Clock className="size-4" />
                      </span>
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-sm text-app-text">{preview(s.Titulo)}</span>
                        <span className="text-xs text-app-muted">
                          {new Date(s.AtualizadaEm).toLocaleTimeString("pt-BR", {
                            hour: "2-digit",
                            minute: "2-digit",
                          })}
                          {/* A coluna de tokens do design chega na Fase 7B:
                              nada mede token hoje, e número inventado é pior
                              que coluna ausente (D12). */}
                        </span>
                      </span>
                    </button>
                  </li>
                ))}
            </ul>
          </div>
        ))}

        {!carregando && !falha && sessoes.length === 0 && (
          <p className="mt-10 text-center text-sm text-app-muted">Nenhuma conversa encontrada.</p>
        )}

        {!acabou && sessoes.length > 0 && (
          <button
            type="button"
            onClick={carregarMais}
            disabled={carregando}
            className="mx-auto block rounded-lg bg-white/5 px-4 py-2 text-xs text-app-text hover:bg-app-hover disabled:opacity-40"
          >
            Carregar mais
          </button>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Passo 4: Ligar no `App.tsx` e apagar a aba antiga**

Em `web/src/App.tsx`, importe `AbaHistorico`, acrescente a função de retomada e troque o painel provisório:

```tsx
  const retomar = (id: string) => {
    clienteRef.current?.enviar("retomar", crypto.randomUUID(), { sessao_id: id });
  };
```

```tsx
        historico: <AbaHistorico sessaoAtual={estado.sessaoAtual} aoRetomar={retomar} />,
```

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && git rm web/src/abas/sessoes.tsx web/src/abas/sessoes.test.tsx
```

- [ ] **Passo 5: Rodar e ver passar**

```bash
npm test --prefix web && npm run build --prefix web
```

Esperado: suíte verde e build sem erro.

- [ ] **Passo 6: Commit**

```bash
git add -A web/src
git commit -m "feat(web): aba Historico paginada, agrupada por dia e com busca no servidor"
```

---

## Task 14: Ciclo de vida da sessão no cliente

**Arquivos:**
- Criar: `web/src/lib/sessao.ts`
- Criar: `web/src/lib/sessao.test.ts`
- Modificar: `web/src/App.tsx`

- [ ] **Passo 1: Escrever o teste que falha**

Crie `web/src/lib/sessao.test.ts`:

```ts
import { beforeEach, describe, expect, it } from "vitest";
import { guardarSessao, limparSessao, sessaoGuardada } from "./sessao";

beforeEach(() => {
  sessionStorage.clear();
});

describe("Sessão guardada", () => {
  it("nasce vazia", () => {
    expect(sessaoGuardada()).toBeNull();
  });

  it("guarda e devolve o id", () => {
    guardarSessao("11111111-1111-1111-1111-111111111111");
    expect(sessaoGuardada()).toBe("11111111-1111-1111-1111-111111111111");
  });

  it("limpa", () => {
    guardarSessao("11111111-1111-1111-1111-111111111111");
    limparSessao();
    expect(sessaoGuardada()).toBeNull();
  });

  // O servidor manda sessao_id vazio no sessao_retomada de uma sessão nova
  // (ver §6.3 da spec). Guardar string vazia faria o cliente pedir
  // "retomar" de uma sessão que não existe no próximo carregamento.
  it("ignora id vazio", () => {
    guardarSessao("");
    expect(sessaoGuardada()).toBeNull();
  });

  // sessionStorage some em WebView com armazenamento desligado. Falhar aí
  // derrubaria o app inteiro por causa de uma conveniência.
  it("não explode quando o sessionStorage não está disponível", () => {
    const original = Object.getOwnPropertyDescriptor(window, "sessionStorage");
    Object.defineProperty(window, "sessionStorage", {
      configurable: true,
      get() {
        throw new Error("bloqueado");
      },
    });

    expect(() => guardarSessao("x")).not.toThrow();
    expect(sessaoGuardada()).toBeNull();

    if (original) Object.defineProperty(window, "sessionStorage", original);
  });
});
```

- [ ] **Passo 2: Rodar e ver falhar**

```bash
npm test --prefix web -- lib/sessao
```

Esperado: FALHA com `Failed to resolve import "./sessao"`.

- [ ] **Passo 3: Escrever o módulo**

Crie `web/src/lib/sessao.ts`:

```ts
// Ciclo de vida da sessão no cliente (D6).
//
// sessionStorage e não localStorage: sobrevive a F5 e a queda de rede — que
// continuam RETOMANDO a mesma conversa, preservando a correção da Fase 6 —
// mas morre quando a janela fecha, que é o gatilho de sessão nova que o
// Arthur pediu. Também mantém celular e desktop independentes: cada aba tem
// o seu.
const CHAVE = "james.sessao";

export function sessaoGuardada(): string | null {
  try {
    return sessionStorage.getItem(CHAVE) || null;
  } catch {
    return null;
  }
}

export function guardarSessao(id: string): void {
  try {
    if (!id) {
      sessionStorage.removeItem(CHAVE);
      return;
    }
    sessionStorage.setItem(CHAVE, id);
  } catch {
    // WebView com armazenamento desligado: seguir sem persistir é melhor
    // que derrubar o app.
  }
}

export function limparSessao(): void {
  try {
    sessionStorage.removeItem(CHAVE);
  } catch {
    // idem
  }
}
```

- [ ] **Passo 4: Rodar e ver passar**

```bash
npm test --prefix web -- lib/sessao
```

Esperado: 5 testes passando.

- [ ] **Passo 5: Ligar no `App.tsx`**

Importe:

```tsx
import { guardarSessao, limparSessao, sessaoGuardada } from "./lib/sessao";
```

Troque a inicialização do espelho de sessão, para que ele já nasça com o que estava guardado:

```tsx
  const sessaoAtualRef = useRef<string | null>(sessaoGuardada());
```

**`jaConectouRef` sai por inteiro** — nem a declaração, nem a linha que o seta. A condição de reemissão perde o `jaConectouRef.current &&`, o que já deixa a variável sem nenhum leitor; escrever num `useRef` que ninguém lê é estado morto, não uma decisão deliberada.

No efeito que espelha `estado.sessaoAtual`, **a ordem de efeitos importa e o texto ingênuo tem um bug**: no mount, `estado.sessaoAtual` ainda é `null` (vem de `estadoInicial`, em `estado/turno.ts` — o redutor nunca volta a pôr `null` ali sozinho, só `novaSessao()` limpa direto). Sem guarda, este efeito roda **antes** do efeito de conexão (ordem de declaração) e sobrescreve com `null` o valor que `sessaoAtualRef` acabou de carregar do `sessionStorage` — a sessão persistida de um F5 nunca seria retomada, porque quando a conexão WS abre de verdade (sempre assíncrono), o ref já foi clobbed. É o objetivo central desta task falhando exatamente no caminho que nenhum teste do plano original exercitava:

```tsx
  // Só espelha/persiste quando há sessão de verdade. No mount,
  // estado.sessaoAtual começa null (estadoInicial) — sem esta guarda, esse
  // null sobrescreveria (antes mesmo da conexão WS abrir) o valor que
  // sessaoAtualRef acabou de carregar do sessionStorage num F5, e a sessão
  // persistida nunca seria retomada. novaSessao() já limpa o ref e o
  // storage diretamente quando o usuário pede sessão nova — este efeito não
  // precisa (e não deve) fazer isso de novo.
  useEffect(() => {
    if (estado.sessaoAtual) {
      sessaoAtualRef.current = estado.sessaoAtual;
      guardarSessao(estado.sessaoAtual);
    }
  }, [estado.sessaoAtual]);
```

No `aoMudarEstado`, a condição de reemissão perde o `jaConectouRef.current &&` — agora a primeira conexão também retoma, quando há sessão guardada de um F5:

```tsx
      // Retoma tanto na reconexão (sessão viva no redutor) quanto na
      // primeira conexão depois de um F5 (sessão veio do sessionStorage).
      // O que NÃO retoma é janela nova: ali o sessionStorage está vazio, e
      // a sessão nasce no servidor no primeiro turno.
      if (sessaoAtualRef.current) {
        cliente.enviar("retomar", crypto.randomUUID(), { sessao_id: sessaoAtualRef.current });
      }
```

(Sem a linha `jaConectouRef.current = true;` no fim — a variável não existe mais.)

E implemente o botão de sessão nova, no lugar do `novaSessao` vazio:

```tsx
  const novaSessao = () => {
    limparSessao();
    sessaoAtualRef.current = null;
    // Limpar só no cliente deixaria o servidor na conversa velha:
    // sessaoID e historico são variáveis locais do handler em ws.go.
    clienteRef.current?.enviar("nova_sessao", crypto.randomUUID(), {});
  };
```

- [ ] **Passo 6: Isolar o `sessionStorage` entre testes**

**Isto quebra a suíte se for esquecido.** `App.test.tsx` tem um caso que grava sessão via `resposta_final`; a partir de agora isso persiste em `sessionStorage`, e o jsdom **não limpa entre testes**. O caso seguinte — `"não pede retomada na primeira conexão, quando ainda não há sessão"` — passaria a achar a sessão do caso anterior e a pedir retomada, falhando por contaminação, não por regressão real.

Acrescente ao topo de `web/src/App.test.tsx`, junto dos outros imports e hooks:

```tsx
import { beforeEach } from "vitest";

beforeEach(() => {
  // A sessão agora vive em sessionStorage, que o jsdom mantém entre testes.
  // Sem esta limpeza, um caso vaza sessão para o próximo.
  sessionStorage.clear();
});
```

- [ ] **Passo 6.5: Teste de regressão para o bug do efeito-espelho**

Nenhum teste do Passo 1 ou dos 4 casos que já existiam em `App.test.tsx` exercita "primeira conexão com sessão já guardada de um F5" — que é o caminho que o efeito-espelho sem guarda quebrava (ver comentário do Passo 5). Acrescente ao import de `App.test.tsx`:

```tsx
import { guardarSessao } from "./lib/sessao";
```

E acrescente este teste ao `describe("App: continuidade da sessão através da reconexão", ...)`, junto dos outros quatro:

```tsx
  it("retoma a sessão guardada em sessionStorage na primeira conexão (F5)", () => {
    guardarSessao(SESSAO);
    render(<App />);
    const primeiro = SocketFalso.instancias[0];

    act(() => primeiro.abrir());

    expect(primeiro.envelopes()).toEqual([
      { tipo: "retomar", id: expect.any(String), dados: { sessao_id: SESSAO } },
    ]);
  });
```

- [ ] **Passo 7: Rodar a suíte**

```bash
npm test --prefix web
```

Esperado: verde, os quatro casos de continuidade de sessão inclusive. Se `"não pede retomada na primeira conexão"` falhar, é o `sessionStorage` não limpo — volte ao passo anterior.

- [ ] **Passo 8: Commit**

```bash
git add web/src/lib/sessao.ts web/src/lib/sessao.test.ts web/src/App.tsx web/src/App.test.tsx
git commit -m "feat(web): sessao guardada em sessionStorage e botao de sessao nova"
```

---

## Task 15: O comando `nova_sessao` no servidor

O ponto mais fácil de sair errado da fase. A verificação é **no Postgres**, não na tela.

**Arquivos:**
- Modificar: `internal/api/ws.go`
- Teste: `internal/api/ws_test.go`

- [ ] **Passo 1: Escrever o teste que falha**

Acrescente a `internal/api/ws_test.go`. Os helpers `servidorWSDeTeste`, `capturarEventosDeUmTurnoNoServidor` e `apagarSessaoDeTeste` já existem no arquivo:

```go
// R3: cliente e servidor discordando sobre qual é a sessão não gera erro
// nenhum — a conversa simplesmente continua na sessão errada. Por isso este
// teste confere o id que voltou do SERVIDOR em dois turnos, e não o estado
// da tela.
func TestNovaSessaoFazOServidorEsquecerASessao(t *testing.T) {
	servidor := servidorWSDeTeste(t)
	defer servidor.Close()

	conexao, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(servidor.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatalf("erro ao conectar: %v", err)
	}
	defer conexao.Close()

	primeira := sessaoDoTurno(t, conexao, "primeira mensagem")
	if primeira == "" {
		t.Fatal("primeiro turno não devolveu sessão")
	}

	if err := conexao.WriteJSON(Envelope{Tipo: "nova_sessao", ID: uuid.NewString(), Dados: json.RawMessage(`{}`)}); err != nil {
		t.Fatalf("erro ao pedir sessão nova: %v", err)
	}
	// O servidor confirma com sessao_retomada vazio antes de o próximo
	// turno começar — sem esperar, a mensagem seguinte poderia chegar antes
	// da limpeza e o teste passaria por acidente.
	confirmacao := lerAteEvento(t, conexao, "sessao_retomada")
	var payload payloadSessaoRetomada
	if err := json.Unmarshal(confirmacao.Dados, &payload); err != nil {
		t.Fatalf("payload ilegível: %v", err)
	}
	if payload.SessaoID != "" {
		t.Fatalf("sessão nova devia vir sem id, veio %q", payload.SessaoID)
	}
	if len(payload.Mensagens) != 0 {
		t.Fatalf("sessão nova devia vir sem mensagens, vieram %d", len(payload.Mensagens))
	}

	segunda := sessaoDoTurno(t, conexao, "segunda mensagem")
	if segunda == "" {
		t.Fatal("segundo turno não devolveu sessão")
	}
	if segunda == primeira {
		t.Fatalf("o servidor continuou na sessão velha: %s", segunda)
	}

	primeiroUUID, _ := uuid.Parse(primeira)
	segundoUUID, _ := uuid.Parse(segunda)
	t.Cleanup(func() {
		apagarSessaoDeTeste(t, primeiroUUID)
		apagarSessaoDeTeste(t, segundoUUID)
	})
}

// sessaoDoTurno manda uma mensagem e devolve o sessao_id do resposta_final.
func sessaoDoTurno(t *testing.T, conexao *websocket.Conn, texto string) string {
	t.Helper()
	dados, _ := json.Marshal(map[string]string{"texto": texto})
	if err := conexao.WriteJSON(Envelope{Tipo: "mensagem", ID: uuid.NewString(), Dados: dados}); err != nil {
		t.Fatalf("erro ao enviar mensagem: %v", err)
	}
	evento := lerAteEvento(t, conexao, "resposta_final")
	var payload payloadRespostaFinal
	if err := json.Unmarshal(evento.Dados, &payload); err != nil {
		t.Fatalf("payload ilegível: %v", err)
	}
	return payload.SessaoID
}

// lerAteEvento consome envelopes até achar o tipo pedido, ignorando os
// intermediários (ferramenta_iniciada, audio_pronto e afins).
func lerAteEvento(t *testing.T, conexao *websocket.Conn, tipo string) Envelope {
	t.Helper()
	conexao.SetReadDeadline(time.Now().Add(10 * time.Second))
	for {
		var envelope Envelope
		if err := conexao.ReadJSON(&envelope); err != nil {
			t.Fatalf("erro ao ler esperando %q: %v", tipo, err)
		}
		if envelope.Tipo == tipo {
			return envelope
		}
	}
}
```

> `Envelope.Dados` é `json.RawMessage` (`internal/api/envelope.go`), por isso o `json.RawMessage(`{}`)` acima compila direto.

- [ ] **Passo 2: Rodar e ver falhar**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && go test ./internal/api/ -run TestNovaSessaoFazOServidorEsquecer -v
```

Esperado: FALHA — o servidor ignora `nova_sessao` (o `switch` da goroutine de leitura não tem esse caso), então o teste trava lendo o `sessao_retomada` que nunca chega e estoura o deadline de 10s.

- [ ] **Passo 3: Rotear o comando**

Em `internal/api/ws.go`, na goroutine de leitura, acrescente o caso:

```go
			case "nova_sessao":
				mensagens <- envelope
```

E no laço sequencial:

```go
	for envelope := range mensagens {
		switch envelope.Tipo {
		case "retomar":
			s.tratarRetomar(r.Context(), cliente, envelope, &sessaoID, &historico)
		case "nova_sessao":
			s.tratarNovaSessao(cliente, envelope, &sessaoID, &historico)
		default:
			s.tratarMensagem(r.Context(), cliente, envelope, &sessaoID, &historico)
		}
	}
```

- [ ] **Passo 4: Escrever o handler**

Em `internal/api/ws.go`, junto de `tratarRetomar`:

```go
// tratarNovaSessao esquece a conversa corrente NO SERVIDOR. Roda no laço
// sequencial, junto com "mensagem" e "retomar", porque mexe em sessaoID e
// historico — o mesmo motivo que mantém "mensagem" fora da goroutine de
// leitura.
//
// Zerar só no cliente não bastaria: sessaoID e historico são variáveis
// locais deste handler, então o turno seguinte continuaria gravando na
// sessão velha, sem erro nenhum. É a mesma classe de bug que a auditoria da
// Fase 6 achou no retomar.
//
// Responde com sessao_retomada vazio em vez de inventar um evento novo: o
// cliente já sabe tratar esse evento, e a semântica é a mesma — "esta é a
// conversa em que você está agora". Nenhuma sessão é criada aqui; ela nasce
// no Postgres no primeiro turno, como processarTurno já faz quando sessaoID
// é uuid.Nil.
func (s *Servidor) tratarNovaSessao(cliente *conexaoCliente, envelope Envelope, sessaoID *uuid.UUID, historico *[]agente.Mensagem) {
	*sessaoID = uuid.Nil
	*historico = nil
	cliente.Enviar("sessao_retomada", envelope.ID, payloadSessaoRetomada{
		SessaoID: "",
		// Slice vazio e não nil: map/slice nil viram `null` no JSON, e o
		// tipo TypeScript promete array. Já mordeu neste projeto, no card
		// de aprovação com argumentos nil.
		Mensagens: []mensagemDoHistorico{},
	})
}
```

- [ ] **Passo 5: Rodar e ver passar**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && go vet ./... && go test ./internal/api/ -v
```

Esperado: pacote verde, incluindo o teste novo.

- [ ] **Passo 6: Acrescentar o caso ao redutor do front**

O redutor já trata `sessao_retomada`, e um payload vazio zera as mensagens — que é o comportamento certo. Acrescente o caso de guarda a `web/src/estado/turno.test.ts`:

```ts
it("sessao_retomada vazia zera a conversa", () => {
  const cheio = reduzir(estadoInicial, {
    tipo: "sessao_retomada",
    dados: { sessao_id: "11111111-1111-1111-1111-111111111111", mensagens: [{ papel: "usuario", texto: "oi" }] },
  });

  const zerado = reduzir(cheio, { tipo: "sessao_retomada", dados: { sessao_id: "", mensagens: [] } });

  expect(zerado.mensagens).toHaveLength(0);
  expect(zerado.sessaoAtual).toBe("");
});
```

```bash
npm test --prefix web -- estado/turno
```

Esperado: PASSA sem mudar o redutor.

- [ ] **Passo 7: Commit**

```bash
git add internal/api/ws.go internal/api/ws_test.go web/src/estado/turno.test.ts
git commit -m "feat(api): comando nova_sessao faz o servidor esquecer a conversa corrente"
```

---

## Task 16: Ancoragem do resumo no Postgres

**Arquivos:**
- Modificar: `internal/armazenamento/postgres/mensagem.go`
- Teste: `internal/armazenamento/postgres/mensagem_test.go`

- [ ] **Passo 1: Escrever o teste que falha**

Acrescente a `internal/armazenamento/postgres/mensagem_test.go`:

```go
func TestIDDeCorteEListarNaoResumidas(t *testing.T) {
	pool := abrirPoolMigrado(t)
	ctx := context.Background()
	sessoes := NovoSessaoRepositorio(pool)
	mensagens := NovoMensagemRepositorio(pool)

	sessao, err := sessoes.Criar(ctx, "conversa de dez mensagens")
	if err != nil {
		t.Fatalf("erro ao criar sessão: %v", err)
	}

	var ids []int64
	for i := 0; i < 10; i++ {
		m, err := mensagens.Salvar(ctx, sessao.ID, "usuario", fmt.Sprintf("mensagem %d", i), nil)
		if err != nil {
			t.Fatalf("erro ao salvar: %v", err)
		}
		ids = append(ids, m.ID)
	}

	// Janela de 4: as 4 últimas ficam dentro, o corte é o id da 7ª (índice 6).
	corte, err := mensagens.IDDeCorte(ctx, sessao.ID, 4)
	if err != nil {
		t.Fatalf("erro no corte: %v", err)
	}
	if corte != ids[6] {
		t.Fatalf("corte errado: esperava %d, veio %d", ids[6], corte)
	}

	// Nada resumido ainda: tudo abaixo do corte é candidato.
	fora, err := mensagens.ListarNaoResumidas(ctx, sessao.ID, corte, 0)
	if err != nil {
		t.Fatalf("erro ao listar não resumidas: %v", err)
	}
	if len(fora) != 6 {
		t.Fatalf("esperava 6 mensagens fora da janela, vieram %d", len(fora))
	}
	if fora[0].ID != ids[0] {
		t.Fatalf("ordem errada: primeira devia ser %d, veio %d", ids[0], fora[0].ID)
	}

	// Já resumido até a terceira: sobram três.
	restantes, err := mensagens.ListarNaoResumidas(ctx, sessao.ID, corte, ids[2])
	if err != nil {
		t.Fatalf("erro ao listar restantes: %v", err)
	}
	if len(restantes) != 3 {
		t.Fatalf("esperava 3 restantes, vieram %d", len(restantes))
	}

	// Nada novo: já cobre tudo que está fora da janela.
	nenhuma, err := mensagens.ListarNaoResumidas(ctx, sessao.ID, corte, ids[5])
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if len(nenhuma) != 0 {
		t.Fatalf("esperava nenhuma, vieram %d", len(nenhuma))
	}
}

// R4: numa sessão retomada com mais de 200 mensagens o histórico em memória
// é só a cauda, e contar sobre ele resumiria o trecho errado — em silêncio.
// A ancoragem por id do Postgres é o que salva, e é isto que este teste
// prova: o corte não depende de quantas mensagens o processo tem em mãos.
func TestIDDeCorteIgnoraOQueOProcessoTemEmMemoria(t *testing.T) {
	pool := abrirPoolMigrado(t)
	ctx := context.Background()
	sessoes := NovoSessaoRepositorio(pool)
	mensagens := NovoMensagemRepositorio(pool)

	sessao, err := sessoes.Criar(ctx, "conversa de 250 mensagens")
	if err != nil {
		t.Fatalf("erro ao criar sessão: %v", err)
	}
	var ids []int64
	for i := 0; i < 250; i++ {
		m, err := mensagens.Salvar(ctx, sessao.ID, "usuario", fmt.Sprintf("m%d", i), nil)
		if err != nil {
			t.Fatalf("erro ao salvar: %v", err)
		}
		ids = append(ids, m.ID)
	}

	// A retomada carregaria só as 200 mais recentes. O corte para uma
	// janela de 10 tem que apontar para a 241ª da SESSÃO (índice 240), não
	// para a 241ª daquele recorte.
	corte, err := mensagens.IDDeCorte(ctx, sessao.ID, 10)
	if err != nil {
		t.Fatalf("erro no corte: %v", err)
	}
	if corte != ids[240] {
		t.Fatalf("corte ancorou no recorte em vez da sessão: esperava %d, veio %d", ids[240], corte)
	}
}
```

- [ ] **Passo 2: Rodar e ver falhar**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && go test ./internal/armazenamento/postgres/ -run TestIDDeCorte
```

Esperado: FALHA de compilação — `IDDeCorte` e `ListarNaoResumidas` não existem.

- [ ] **Passo 3: Implementar**

Acrescente a `internal/armazenamento/postgres/mensagem.go`:

```go
// IDDeCorte devolve o id da mensagem mais ANTIGA que ainda cabe na janela,
// dado o tamanho dela. Tudo com id menor está fora da janela e é candidato
// a resumo.
//
// Ordenação por id, não por criada_em: id é BIGSERIAL, monotônico e sem
// empate. criada_em tem DEFAULT now() e duas mensagens do mesmo turno podem
// cair no mesmo instante — ordem instável ali produziria resumo com buraco
// ou com repetição, sem erro nenhum.
//
// Devolve 0 quando a janela cobre a sessão inteira (não há nada fora dela).
func (r *MensagemRepositorio) IDDeCorte(ctx context.Context, sessaoID uuid.UUID, tamanhoJanela int) (int64, error) {
	if tamanhoJanela <= 0 {
		return 0, nil
	}
	var id int64
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM mensagens WHERE sessao_id=$1 ORDER BY id DESC OFFSET $2 LIMIT 1`,
		sessaoID, tamanhoJanela-1,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("erro ao calcular corte da sessão %s: %w", sessaoID, err)
	}
	return id, nil
}

// ListarNaoResumidas devolve, em ordem crescente de id, as mensagens da
// sessão que estão FORA da janela (id < corte) e que o resumo ainda não
// cobre (id > resumidoAte). resumidoAte igual a 0 significa "nada resumido".
func (r *MensagemRepositorio) ListarNaoResumidas(ctx context.Context, sessaoID uuid.UUID, corte, resumidoAte int64) ([]Mensagem, error) {
	if corte <= 0 {
		return nil, nil
	}
	linhas, err := r.pool.Query(ctx,
		`SELECT id, sessao_id, papel, conteudo, chamadas, criada_em FROM mensagens
		 WHERE sessao_id=$1 AND id < $2 AND id > $3 ORDER BY id ASC`,
		sessaoID, corte, resumidoAte)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar mensagens não resumidas da sessão %s: %w", sessaoID, err)
	}
	defer linhas.Close()

	var mensagens []Mensagem
	for linhas.Next() {
		var m Mensagem
		if err := linhas.Scan(&m.ID, &m.SessaoID, &m.Papel, &m.Conteudo, &m.Chamadas, &m.CriadaEm); err != nil {
			return nil, err
		}
		mensagens = append(mensagens, m)
	}
	return mensagens, linhas.Err()
}
```

Acrescente `"errors"` e `"github.com/jackc/pgx/v5"` aos imports de `mensagem.go`.

- [ ] **Passo 4: Rodar e ver passar**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && go test ./internal/armazenamento/postgres/ -run TestIDDeCorte -v
```

Esperado: os dois testes passando.

- [ ] **Passo 5: Commit**

```bash
git add internal/armazenamento/postgres/mensagem.go internal/armazenamento/postgres/mensagem_test.go
git commit -m "feat(banco): corte da janela e listagem de mensagens ainda nao resumidas"
```

- [ ] **Passo 6: Índice para a query nova, achado pela revisão de qualidade**

`IDDeCorte`/`ListarNaoResumidas` ordenam e filtram por `id`, não por `criada_em` — e o único índice que existe em `mensagens` é `idx_mensagens_sessao (sessao_id, criada_em)` (`0001_inicial.sql`), que não cobre a query nova. Medido com `EXPLAIN ANALYZE` contra uma sessão de 100 mil mensagens: sem índice, `IDDeCorte` varre a sessão inteira (9,8ms, 1116 buffers); com o índice certo, vira `Index Only Scan Backward` direto (0,045ms, 5 buffers) — ~200x mais rápido. O custo cresce com o **tamanho total da sessão**, não com o tamanho da janela pedida — o oposto do que a feature deveria garantir, e essa é justamente a query que o resumo rolante (Task 17/18) vai chamar toda vez que checar se há trabalho a fazer.

Crie `internal/armazenamento/migracoes/0003_indice_mensagens_id.sql`:

```sql
-- Índice para IDDeCorte/ListarNaoResumidas (Fase 7A, Task 16): a query
-- ordena e filtra por id, não por criada_em, e sem este índice o Postgres
-- varre a sessão inteira em vez de só a janela pedida — ~200x mais lento
-- numa sessão de 100 mil mensagens, medido com EXPLAIN ANALYZE antes desta
-- migração existir.
--
-- CREATE INDEX simples, não CONCURRENTLY: AplicarMigracoes roda cada
-- migração dentro de uma transação (internal/armazenamento/postgres/
-- migrar.go), e Postgres recusa CONCURRENTLY dentro de transação. Trava
-- escrita em `mensagens` brevemente ao aplicar — aceitável no tamanho atual
-- do banco de trabalho.
CREATE INDEX idx_mensagens_sessao_id ON mensagens (sessao_id, id);
```

Acrescente a `internal/armazenamento/postgres/migrar_test.go`, no mesmo padrão de `TestMigracao0002PreservaSessaoExistente` (Task 10) — a migração precisa ser segura em banco que já tem dados:

```go
// A 0003 roda em banco de trabalho já populado. Reaplicar (via
// AplicarMigracoes, idempotente) não pode falhar, e o índice precisa
// existir depois — é isso que garante que IDDeCorte/ListarNaoResumidas não
// voltem a fazer scan completo da sessão.
func TestMigracao0003CriaIndiceEmBancoPopulado(t *testing.T) {
	pool := abrirPoolMigrado(t)
	ctx := context.Background()

	if err := AplicarMigracoes(ctx, pool); err != nil {
		t.Fatalf("erro ao reaplicar migrações: %v", err)
	}

	var existe bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE tablename='mensagens' AND indexname='idx_mensagens_sessao_id')`,
	).Scan(&existe)
	if err != nil {
		t.Fatalf("erro ao checar o índice: %v", err)
	}
	if !existe {
		t.Fatal("idx_mensagens_sessao_id não existe depois da migração")
	}
}
```

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && go test ./internal/armazenamento/postgres/ -run TestMigracao0003 -v
```

Esperado: passa. Rode também a suíte inteira do pacote de novo, para confirmar que o índice novo não muda nenhum resultado dos testes já existentes (só a velocidade):

```bash
go test ./internal/armazenamento/postgres/ -v
```

```bash
git add internal/armazenamento/migracoes/0003_indice_mensagens_id.sql internal/armazenamento/postgres/migrar_test.go
git commit -m "perf(banco): indice sessao_id+id, IDDeCorte parava de varrer a sessao inteira"
```

---

## Task 17: O resumidor

**Arquivos:**
- Criar: `internal/memoria/resumo.go`
- Criar: `internal/memoria/resumo_test.go`
- Modificar: `internal/config/config.go`
- Modificar: `config.toml`

- [ ] **Passo 1: Escrever o teste que falha**

Crie `internal/memoria/resumo_test.go`:

```go
package memoria

import (
	"context"
	"errors"
	"strings"
	"testing"

	"james/internal/agente"
)

type providerDeResumo struct {
	resposta   string
	erro       error
	recebido   agente.Requisicao
	chamadas   int
}

func (p *providerDeResumo) Gerar(ctx context.Context, req agente.Requisicao) (agente.Resposta, error) {
	p.chamadas++
	p.recebido = req
	if p.erro != nil {
		return agente.Resposta{}, p.erro
	}
	return agente.Resposta{Texto: p.resposta}, nil
}

func (p *providerDeResumo) GerarStream(ctx context.Context, req agente.Requisicao, emitir func(string)) (agente.Resposta, error) {
	return p.Gerar(ctx, req)
}

func TestResumidorProduzOPrimeiroResumo(t *testing.T) {
	provider := &providerDeResumo{resposta: "o Arthur pediu o clima e depois a agenda"}
	resumidor := Resumidor{Provider: provider, OrcamentoTokens: 1000}

	novo, err := resumidor.Resumir(context.Background(), "", []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "como está o clima?"},
		{Papel: agente.PapelModelo, Texto: "22 graus e nublado"},
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if novo != "o Arthur pediu o clima e depois a agenda" {
		t.Fatalf("resumo errado: %q", novo)
	}
	if provider.chamadas != 1 {
		t.Fatalf("esperava 1 chamada, houve %d", provider.chamadas)
	}
}

// O resumo anterior tem que entrar no prompt, senão cada rodada esquece
// tudo que veio antes e o "rolante" não rola.
func TestResumidorLevaOResumoAnteriorNoPrompt(t *testing.T) {
	provider := &providerDeResumo{resposta: "resumo novo"}
	resumidor := Resumidor{Provider: provider, OrcamentoTokens: 1000}

	if _, err := resumidor.Resumir(context.Background(), "resumo velho aqui", []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "e a agenda?"},
	}); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	var achou bool
	for _, m := range provider.recebido.Mensagens {
		if strings.Contains(m.Texto, "resumo velho aqui") {
			achou = true
		}
	}
	if !achou {
		t.Fatal("o resumo anterior não foi ao provider")
	}
}

func TestResumidorSemMensagensNaoChamaOProvider(t *testing.T) {
	provider := &providerDeResumo{resposta: "não devia ser chamado"}
	resumidor := Resumidor{Provider: provider, OrcamentoTokens: 1000}

	novo, err := resumidor.Resumir(context.Background(), "resumo velho", nil)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if novo != "resumo velho" {
		t.Fatalf("devia devolver o resumo velho intacto, veio %q", novo)
	}
	if provider.chamadas != 0 {
		t.Fatalf("não devia chamar o provider, chamou %d vezes", provider.chamadas)
	}
}

func TestResumidorPropagaOErroDoProvider(t *testing.T) {
	provider := &providerDeResumo{erro: errors.New("cota estourada")}
	resumidor := Resumidor{Provider: provider, OrcamentoTokens: 1000}

	if _, err := resumidor.Resumir(context.Background(), "", []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "oi"},
	}); err == nil {
		t.Fatal("esperava erro do provider")
	}
}

// R5: sem teto, o resumo cresce sem parar e come a janela que ele deveria
// proteger.
func TestResumidorCortaOResumoNoOrcamento(t *testing.T) {
	longo := strings.Repeat("palavra ", 5000)
	provider := &providerDeResumo{resposta: longo}
	resumidor := Resumidor{Provider: provider, OrcamentoTokens: 100}

	novo, err := resumidor.Resumir(context.Background(), "", []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "oi"},
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if EstimarTokens(novo) > 100 {
		t.Fatalf("resumo passou do orçamento: %d tokens", EstimarTokens(novo))
	}
}

// PapelFerramenta nunca tem Texto — só Resultados (loop.go grava
// Mensagem{Papel: PapelFerramenta, Resultados: resultados}, sem campo
// Texto). E PapelModelo quando só chama ferramenta, sem texto de
// acompanhamento (comum em function calling), também fica com Texto vazio,
// só Chamadas preenchido. Sem isso, o resumo esquece que qualquer
// ferramenta rodou — que é boa parte do que o James faz.
func TestResumidorIncluiChamadasEResultadosDeFerramenta(t *testing.T) {
	provider := &providerDeResumo{resposta: "resumo novo"}
	resumidor := Resumidor{Provider: provider, OrcamentoTokens: 1000}

	if _, err := resumidor.Resumir(context.Background(), "", []agente.Mensagem{
		{Papel: agente.PapelModelo, Chamadas: []agente.ChamadaFerramenta{
			{ID: "1", Nome: "criar_arquivo", Args: map[string]any{"caminho": "/home/x/rel.pdf"}},
		}},
		{Papel: agente.PapelFerramenta, Resultados: []agente.ResultadoFerramenta{
			{ID: "1", Nome: "criar_arquivo", Saida: "arquivo criado em /home/x/rel.pdf"},
		}},
	}); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	var texto string
	for _, m := range provider.recebido.Mensagens {
		texto += m.Texto
	}
	if !strings.Contains(texto, "criar_arquivo") {
		t.Fatalf("chamada de ferramenta não chegou ao provider: %q", texto)
	}
	if !strings.Contains(texto, "arquivo criado em /home/x/rel.pdf") {
		t.Fatalf("resultado da ferramenta não chegou ao provider: %q", texto)
	}
}
```

- [ ] **Passo 2: Rodar e ver falhar**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && go test ./internal/memoria/ -run TestResumidor
```

Esperado: FALHA de compilação — `Resumidor` não existe.

- [ ] **Passo 3: Escrever o resumidor**

Crie `internal/memoria/resumo.go`:

```go
package memoria

import (
	"context"
	"fmt"
	"strings"

	"james/internal/agente"
)

// promptResumo instrui o modelo a produzir o resumo rolante. Fala em
// primeira pessoa do James porque o texto volta pro system prompt dele: um
// resumo escrito como relatório de terceiro faz o modelo se referir a si
// mesmo na terceira pessoa no turno seguinte.
const promptResumo = `Você mantém o resumo de uma conversa entre o Arthur e o James.

Receberá o resumo atual (pode estar vazio) e o trecho de conversa que acabou de sair
da janela de contexto. Devolva UM resumo novo, que substitui o atual e incorpora o
trecho.

Regras:
- Preserve fatos, decisões, nomes, números e pendências. São o que o James vai
  precisar lembrar.
- Descarte cortesia, repetição e o passo a passo de ferramenta que já terminou.
- Escreva em português, em prosa corrida, sem cabeçalho e sem marcador.
- Não invente nada que não esteja no material recebido.
- Devolva apenas o resumo, sem preâmbulo.`

// Resumidor produz o resumo rolante de uma sessão. É separado da Janela de
// propósito: a Janela decide o que cabe no contexto, o Resumidor decide o
// que fazer com o que não coube.
type Resumidor struct {
	Provider agente.Provider
	// OrcamentoTokens é o teto do resumo. Sem ele, o resumo cresce a cada
	// rodada e passa a comer a janela que deveria proteger.
	OrcamentoTokens int
}

// Resumir devolve o resumo atualizado. Sem mensagens novas, devolve o
// anterior intacto e não gasta chamada ao provider.
func (r Resumidor) Resumir(ctx context.Context, resumoAtual string, novas []agente.Mensagem) (string, error) {
	if len(novas) == 0 {
		return resumoAtual, nil
	}

	var corpo strings.Builder
	corpo.WriteString("RESUMO ATUAL:\n")
	if resumoAtual == "" {
		corpo.WriteString("(vazio — esta é a primeira rodada)\n")
	} else {
		corpo.WriteString(resumoAtual)
		corpo.WriteString("\n")
	}
	corpo.WriteString("\nTRECHO QUE SAIU DA JANELA:\n")
	for _, m := range novas {
		// Uma mensagem pode ter Texto, Chamadas e Resultados em qualquer
		// combinação (PapelModelo com function calling sem texto de
		// acompanhamento fica só com Chamadas; PapelFerramenta nunca tem
		// Texto, só Resultados) — por isso os três são escritos
		// independentemente, não com um "continue" que descartaria a
		// mensagem inteira quando Texto estiver vazio.
		if m.Texto != "" {
			fmt.Fprintf(&corpo, "%s: %s\n", m.Papel, m.Texto)
		}
		for _, c := range m.Chamadas {
			fmt.Fprintf(&corpo, "%s chamou %s(%v)\n", m.Papel, c.Nome, c.Args)
		}
		for _, res := range m.Resultados {
			if res.Erro != "" {
				fmt.Fprintf(&corpo, "%s falhou: %s\n", res.Nome, res.Erro)
			} else {
				fmt.Fprintf(&corpo, "%s devolveu: %s\n", res.Nome, res.Saida)
			}
		}
	}

	resposta, err := r.Provider.Gerar(ctx, agente.Requisicao{
		Sistema:   promptResumo,
		Mensagens: []agente.Mensagem{{Papel: agente.PapelUsuario, Texto: corpo.String()}},
	})
	if err != nil {
		return "", fmt.Errorf("erro ao resumir a conversa: %w", err)
	}

	return cortarNoOrcamento(strings.TrimSpace(resposta.Texto), r.OrcamentoTokens), nil
}

// cortarNoOrcamento é a rede de segurança para quando o modelo ignora a
// instrução de ser breve. Corta em byte porque EstimarTokens é a mesma régua
// grosseira de 4 bytes por token usada no resto do pacote.
func cortarNoOrcamento(texto string, orcamentoTokens int) string {
	if orcamentoTokens <= 0 || EstimarTokens(texto) <= orcamentoTokens {
		return texto
	}
	limite := orcamentoTokens * 4
	if limite > len(texto) {
		return texto
	}
	// Não corta no meio de uma runa multibyte.
	for limite > 0 && !isInicioDeRuna(texto[limite]) {
		limite--
	}
	return strings.TrimSpace(texto[:limite])
}

func isInicioDeRuna(b byte) bool {
	return b&0xC0 != 0x80
}
```

> `agente.Provider` é a interface que o `Loop` já usa (`internal/agente/loop.go:19`). O `Resumidor` a reaproveita — não crie interface nova.

- [ ] **Passo 4: Rodar e ver passar**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && go test ./internal/memoria/ -v
```

Esperado: os seis testes novos passando, e os antigos do pacote intactos.

- [ ] **Passo 5: Acrescentar o orçamento à configuração**

Em `internal/config/config.go`:

Na struct `Config`, junto de `OrcamentoNotas`:

```go
	OrcamentoResumo  int
```

Na struct `arquivoToml`, dentro de `Memoria`:

```go
		OrcamentoResumo  int    `toml:"orcamento_resumo"`
```

Na montagem do `cfg`:

```go
		OrcamentoResumo:  intOuDefault(arq.Memoria.OrcamentoResumo, 1000),
```

Em `config.toml`, na seção `[memoria]`:

```toml
# Teto do resumo rolante da sessão. Sem teto, o resumo cresce a cada rodada
# e passa a comer a janela de conversa que ele deveria proteger.
orcamento_resumo = 1000
```

- [ ] **Passo 6: Rodar a suíte de config e commitar**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && go test ./internal/config/ ./internal/memoria/
```

Esperado: verde.

```bash
git add internal/memoria/resumo.go internal/memoria/resumo_test.go internal/config/config.go config.toml
git commit -m "feat(memoria): resumidor rolante com orcamento proprio"
```

---

## Task 18: Ligar o resumo ao turno

**Arquivos:**
- Modificar: `internal/api/rest.go` (campo no `Servidor`)
- Modificar: `internal/api/ws.go` (`processarTurno`)
- Modificar: `cmd/jamesd/main.go` (montagem)
- Teste: `internal/api/ws_test.go`

- [ ] **Passo 1: Escrever o teste que falha**

Acrescente a `internal/api/ws_test.go`:

```go
// CA10: conversa que ultrapassa o orçamento produz resumo, e o James
// continua sabendo do começo.
func TestTurnoQueTransbordaGravaOResumo(t *testing.T) {
	s := servidorDeTeste(t)
	ctx := context.Background()

	// Orçamento minúsculo: qualquer conversa transborda na segunda mensagem.
	s.Memoria = memoria.Janela{OrcamentoTokens: 40}
	s.Resumidor = &memoria.Resumidor{
		Provider:        &providerDeResumoFixo{texto: "o Arthur perguntou das capitais"},
		OrcamentoTokens: 1000,
	}

	sessao, err := s.Sessoes.Criar(ctx, "conversa que transborda")
	if err != nil {
		t.Fatalf("erro ao criar sessão: %v", err)
	}
	t.Cleanup(func() { apagarSessaoDeTeste(t, sessao.ID) })

	sessaoID := sessao.ID
	historico := []agente.Mensagem{}
	for i := 0; i < 6; i++ {
		if _, _, err := s.processarTurno(
			ctx, permissao.Privilegiado, &sessaoID, &historico,
			strings.Repeat("qual a capital do estado vizinho ", 3),
			func(agente.ChamadaFerramenta) {}, func(agente.ResultadoFerramenta) {},
		); err != nil {
			t.Fatalf("turno %d falhou: %v", i, err)
		}
	}

	recarregada, err := s.Sessoes.BuscarPorID(ctx, sessaoID)
	if err != nil {
		t.Fatalf("erro ao recarregar sessão: %v", err)
	}
	if recarregada.Resumo == "" {
		t.Fatal("a conversa transbordou e nenhum resumo foi gravado")
	}
	if recarregada.ResumidoAte == nil || *recarregada.ResumidoAte == 0 {
		t.Fatalf("resumido_ate não avançou: %v", recarregada.ResumidoAte)
	}
}

// CA11 / D10: resumidor quebrado degrada para o comportamento de hoje
// (janela truncada), nunca derruba o turno.
func TestFalhaDoResumidorNaoDerrubaOTurno(t *testing.T) {
	s := servidorDeTeste(t)
	ctx := context.Background()

	s.Memoria = memoria.Janela{OrcamentoTokens: 40}
	s.Resumidor = &memoria.Resumidor{
		Provider:        &providerDeResumoQueFalha{},
		OrcamentoTokens: 1000,
	}

	sessao, err := s.Sessoes.Criar(ctx, "conversa com resumidor quebrado")
	if err != nil {
		t.Fatalf("erro ao criar sessão: %v", err)
	}
	t.Cleanup(func() { apagarSessaoDeTeste(t, sessao.ID) })

	sessaoID := sessao.ID
	historico := []agente.Mensagem{}
	for i := 0; i < 4; i++ {
		if _, _, err := s.processarTurno(
			ctx, permissao.Privilegiado, &sessaoID, &historico,
			strings.Repeat("mensagem comprida o suficiente para transbordar ", 3),
			func(agente.ChamadaFerramenta) {}, func(agente.ResultadoFerramenta) {},
		); err != nil {
			t.Fatalf("o resumidor quebrado derrubou o turno %d: %v", i, err)
		}
	}
}

type providerDeResumoFixo struct{ texto string }

func (p *providerDeResumoFixo) Gerar(context.Context, agente.Requisicao) (agente.Resposta, error) {
	return agente.Resposta{Texto: p.texto}, nil
}

func (p *providerDeResumoFixo) GerarStream(ctx context.Context, req agente.Requisicao, emitir func(string)) (agente.Resposta, error) {
	return p.Gerar(ctx, req)
}

type providerDeResumoQueFalha struct{}

func (p *providerDeResumoQueFalha) Gerar(context.Context, agente.Requisicao) (agente.Resposta, error) {
	return agente.Resposta{}, errors.New("cota do resumidor estourada")
}

func (p *providerDeResumoQueFalha) GerarStream(ctx context.Context, req agente.Requisicao, emitir func(string)) (agente.Resposta, error) {
	return p.Gerar(ctx, req)
}
```

- [ ] **Passo 2: Rodar e ver falhar**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && go test ./internal/api/ -run "TestTurnoQueTransborda|TestFalhaDoResumidor"
```

Esperado: FALHA de compilação — `Servidor` não tem campo `Resumidor`.

- [ ] **Passo 3: Acrescentar o campo ao `Servidor`**

Em `internal/api/rest.go`, na struct `Servidor`, junto de `Memoria`:

```go
	// Resumidor é opcional: nil desliga o resumo rolante e a janela volta a
	// simplesmente truncar, que é o comportamento anterior à Fase 7A. Isso
	// mantém todo teste que monta Servidor à mão funcionando sem alteração.
	Resumidor *memoria.Resumidor
```

- [ ] **Passo 4: Ligar em `processarTurno`**

Em `internal/api/ws.go`, dentro de `processarTurno`, logo depois de `janela := s.Memoria.Montar(*historico)`:

```go
	// Resumo rolante (§7 da spec). len(janela) < len(*historico) é o gatilho
	// de transbordo: Montar devolve um sufixo, então encolher significa que
	// descartou. Não é preciso mudar a assinatura de Montar para saber isso.
	//
	// O resumidor lê do Postgres, não das mensagens descartadas:
	// agente.Mensagem não carrega id, e depois de uma retomada o histórico em
	// memória é só a cauda de 200 registros — contar sobre ele resumiria o
	// trecho errado, em silêncio (R4).
	resumoDaSessao := s.atualizarResumo(ctx, *sessaoID, len(janela) < len(*historico), len(janela))
```

E logo depois do bloco que monta `sistemaPrompt` com as notas do Vault, acrescente:

```go
	// Depois do bloco do Vault, pelo mesmo motivo que ele não entra na
	// janela: resumo não é conversa e não pode competir com recência por
	// orçamento.
	if resumoDaSessao != "" {
		sistemaPrompt = sistemaPrompt + "\n\nRESUMO DA CONVERSA ATÉ AQUI:\n" + resumoDaSessao
	}
```

E acrescente o método, junto de `processarTurno`:

```go
// atualizarResumo devolve o resumo da sessão, atualizando-o antes se a
// janela transbordou. Melhor-esforço em toda linha: resumo quebrado degrada
// para janela truncada — o comportamento anterior a esta fase — e nunca
// derruba o turno (D10).
func (s *Servidor) atualizarResumo(ctx context.Context, sessaoID uuid.UUID, transbordou bool, tamanhoJanela int) string {
	if s.Resumidor == nil || sessaoID == uuid.Nil {
		return ""
	}

	sessao, err := s.Sessoes.BuscarPorID(ctx, sessaoID)
	if err != nil {
		slog.Warn("erro ao ler o resumo da sessão, seguindo sem ele", "erro", err)
		return ""
	}
	if !transbordou {
		return sessao.Resumo
	}

	corte, err := s.Mensagens.IDDeCorte(ctx, sessaoID, tamanhoJanela)
	if err != nil || corte == 0 {
		if err != nil {
			slog.Warn("erro ao calcular o corte da janela, seguindo sem resumir", "erro", err)
		}
		return sessao.Resumo
	}

	var resumidoAte int64
	if sessao.ResumidoAte != nil {
		resumidoAte = *sessao.ResumidoAte
	}

	novas, err := s.Mensagens.ListarNaoResumidas(ctx, sessaoID, corte, resumidoAte)
	if err != nil {
		slog.Warn("erro ao ler as mensagens fora da janela, seguindo sem resumir", "erro", err)
		return sessao.Resumo
	}
	if len(novas) == 0 {
		return sessao.Resumo
	}

	comoMensagens, err := reconstruirHistorico(novas)
	if err != nil {
		slog.Warn("erro ao reconstruir as mensagens a resumir, seguindo sem resumir", "erro", err)
		return sessao.Resumo
	}

	atualizado, err := s.Resumidor.Resumir(ctx, sessao.Resumo, comoMensagens)
	if err != nil {
		slog.Warn("erro ao resumir a conversa, seguindo com a janela truncada", "erro", err)
		return sessao.Resumo
	}

	ultimo := novas[len(novas)-1].ID
	if err := s.Sessoes.AtualizarResumo(ctx, sessaoID, atualizado, ultimo); err != nil {
		slog.Warn("erro ao gravar o resumo, seguindo com ele só neste turno", "erro", err)
	}
	return atualizado
}
```

- [ ] **Passo 5: Montar no `main.go`**

Em `cmd/jamesd/main.go`, onde o `api.Servidor` é montado (junto de `Memoria: memoria.Janela{...}`, linha ~117):

```go
		Resumidor: &memoria.Resumidor{
			Provider:        provider,
			OrcamentoTokens: cfg.OrcamentoResumo,
		},
```

> `provider` nasce em `cmd/jamesd/main.go:52` (`gemini.Novo(...)`) e já alimenta o `Loop` na linha 112. Use a mesma variável — o resumidor usa o mesmo modelo do agente, de propósito.

- [ ] **Passo 6: Rodar tudo**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && go vet ./... && go test ./...
```

Esperado: suíte inteira verde.

- [ ] **Passo 7: Commit**

```bash
git add internal/api/ws.go internal/api/rest.go internal/api/ws_test.go cmd/jamesd/main.go
git commit -m "feat(memoria): resumo rolante entra no contexto quando a janela transborda"
```

---

## Task 19: Aba Memória no shell novo

`web/src/abas/memoria.tsx` foi desenhado para o painel escuro antigo, com `opacity` e classes que somem no fundo novo. Esta task não muda o comportamento — só o encaixe visual.

**Arquivos:**
- Modificar: `web/src/abas/memoria.tsx`

- [ ] **Passo 1: Rodar o teste que já existe, para ter a linha de base**

```bash
npm test --prefix web -- abas/memoria
```

Esperado: PASSA. Anote quantos testes são — eles precisam continuar passando ao fim.

- [ ] **Passo 2: Ajustar as classes**

Em `web/src/abas/memoria.tsx`, troque as classes de cor pelas do tema novo, sem mexer na lógica:

- `opacity-60` / `opacity-40` em texto → `text-app-muted`
- `text-amber-300` (aviso) → mantenha, é sinal de alerta e continua legível
- fundos `bg-white/5` → mantenha
- Acrescente um cabeçalho igual ao das outras abas, com o ícone `Gem` do lucide e o título em `font-serif text-xl text-app-text`, dentro de `border-b border-app-line px-6 py-5`
- O contêiner raiz vira `flex h-full flex-col`, com a lista em `flex-1 overflow-y-auto px-6 py-5`

- [ ] **Passo 3: Rodar o teste e ver continuar passando**

```bash
npm test --prefix web -- abas/memoria
```

Esperado: o mesmo número de testes de antes, todos verdes. Se algum quebrou, foi porque a busca era por classe ou por estrutura — ajuste o teste para buscar por texto ou papel, que é mais robusto.

- [ ] **Passo 4: Commit**

```bash
git add web/src/abas/memoria.tsx web/src/abas/memoria.test.tsx
git commit -m "style(web): aba Memoria encaixada no shell novo"
```

---

## Task 20: Ponta a ponta com Playwright

Prometido na Fase 6 e não entregue. Sem ele, toda verificação de front volta a ser manual.

**Arquivos:**
- Criar: `web/playwright.config.ts`
- Criar: `web/e2e/james.spec.ts`
- Modificar: `web/package.json`

- [ ] **Passo 1: Instalar**

```bash
npm install -D @playwright/test --prefix web
```

```bash
npx --prefix web playwright install chromium
```

- [ ] **Passo 2: Configurar**

Crie `web/playwright.config.ts`:

```ts
import { defineConfig } from "@playwright/test";

// Aponta para o jamesd de verdade, não para o vite dev: a SPA é servida
// embutida no binário (//go:embed all:dist), e é essa versão que precisa
// ser exercitada. O jamesd tem que estar de pé antes de rodar.
export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  use: {
    baseURL: "http://127.0.0.1:8080",
    trace: "retain-on-failure",
  },
});
```

Em `web/package.json`, acrescente aos scripts:

```json
    "e2e": "playwright test"
```

- [ ] **Passo 3: Escrever o teste**

Crie `web/e2e/james.spec.ts`:

```ts
import { expect, test } from "@playwright/test";

// ARMADILHA JÁ DOCUMENTADA NESTE PROJETO: input controlado do React só
// atualiza estado quando o onChange sintético dispara. `fill` e `type` do
// Playwright NÃO o disparam — o campo mostra o texto, o estado do React
// continua vazio, e a conclusão errada é "o app está quebrado". Já custou
// tempo duas vezes aqui.
async function digitarEmInputControlado(seletor: string, texto: string, page: import("@playwright/test").Page) {
  await page.locator(seletor).evaluate((elemento, valor) => {
    const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, "value")!.set!;
    setter.call(elemento, valor);
    elemento.dispatchEvent(new Event("input", { bubbles: true }));
  }, texto);
}

test.describe("James como aplicativo", () => {
  test("abre direto no shell, sem bolinha", async ({ page }) => {
    await page.goto("/");

    await expect(page.getByRole("navigation", { name: "Navegação principal" })).toBeVisible();
    await expect(page.getByLabel("Visualização de áudio do James")).toBeVisible();
  });

  test("troca de aba pela barra lateral", async ({ page }) => {
    await page.goto("/");

    await page.getByRole("button", { name: "Histórico" }).click();
    await expect(page.getByPlaceholder("Buscar conversas…")).toBeVisible();

    await page.getByRole("button", { name: "DashBoard" }).click();
    await expect(page.getByText(/Fase 7B/)).toBeVisible();

    await page.getByRole("button", { name: "Mensagens" }).click();
    await expect(page.getByPlaceholder("Escreva para o James…")).toBeVisible();
  });

  test("envia mensagem e recebe resposta", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("button", { name: "Mensagens" }).click();
    await expect(page.getByText("online")).toBeVisible();

    await digitarEmInputControlado('input[placeholder="Escreva para o James…"]', "diga apenas: pong", page);
    await page.getByLabel("Enviar mensagem").click();

    await expect(page.getByText("diga apenas: pong")).toBeVisible();
    // Chamada real ao Gemini: dá folga de tempo.
    await expect(page.locator("text=/pong/i").last()).toBeVisible({ timeout: 25_000 });
  });

  test("abre sessão nova e o histórico registra as duas", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("button", { name: "Mensagens" }).click();

    await digitarEmInputControlado('input[placeholder="Escreva para o James…"]', "primeira conversa", page);
    await page.getByLabel("Enviar mensagem").click();
    await expect(page.getByText("primeira conversa")).toBeVisible();

    await page.getByRole("button", { name: "Nova sessão" }).click();
    await expect(page.getByText("primeira conversa")).toHaveCount(0);

    await page.getByRole("button", { name: "Histórico" }).click();
    await expect(page.getByText("primeira conversa")).toBeVisible({ timeout: 10_000 });
  });

  test("retoma uma conversa pelo histórico", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("button", { name: "Histórico" }).click();

    const primeira = page.locator("ul li button").first();
    await expect(primeira).toBeVisible({ timeout: 10_000 });
    const titulo = (await primeira.locator("span span").first().textContent()) ?? "";

    await primeira.click();
    await page.getByRole("button", { name: "Mensagens" }).click();

    await expect(page.getByText(titulo.replace(/…$/, "").slice(0, 30), { exact: false })).toBeVisible();
  });
});
```

- [ ] **Passo 4: Subir o sistema e rodar**

Em um terminal:

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && npm run build --prefix web && go run ./cmd/jamesd
```

Em outro:

```bash
npm run e2e --prefix web
```

Esperado: 5 testes passando. Se o de retomada falhar por o banco estar vazio, mande uma mensagem qualquer antes — o teste depende de existir pelo menos uma conversa.

- [ ] **Passo 5: Commit**

```bash
git add web/playwright.config.ts web/e2e web/package.json web/package-lock.json
git commit -m "test(e2e): Playwright cobre shell, abas, envio, sessao nova e retomada"
```

---

## Task 21: Verificação ao vivo e handoff

Nenhum critério de aceitação está fechado até ser exercitado contra o sistema real. Testes com mock já passaram limpo neste projeto enquanto o produto estava quebrado — a Fase 5 descobriu, só ao vivo, que a fórmula de custo de token da doc oficial estava errada e que a varredura de elementos nunca funcionava em app real.

- [ ] **Passo 1: Subir tudo**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && docker compose up -d && npm run build --prefix web && go run ./cmd/jamesd
```

Com o device Python em outro terminal, e o shell Wails aberto.

- [ ] **Passo 2: Percorrer os critérios**

Confira um a um, anotando o que passou e o que não passou:

| | Critério | Como exercitar |
|---|---|---|
| CA1 | Abre no shell, sem bolinha; sidebar colapsa | Olhar e clicar no botão de recolher |
| CA2 | Círculos minimizam, maximizam e fecham; janela redimensionável e não fica sobre as outras; no navegador os círculos somem | Clicar nos três; arrastar a borda; abrir `127.0.0.1:8080` no Chrome |
| CA3 | **Orbe deforma conforme a fala**, sem botão, e volta ao repouso | Pedir uma resposta longa e observar. Deformação que não acompanha a fala = analisador ligado no lugar errado |
| CA4 | Card de aprovação na aba James, com contador na sidebar | Pedir algo destrutivo (`Remove-Item` de arquivo de teste) estando na aba DashBoard |
| CA5 | Trilha mostra nome, estado e duração | Pedir uma busca na web |
| CA6 | Derrubar o `jamesd` troca "online" pelo estado real | `Ctrl+C` no jamesd com a aba Mensagens aberta |
| CA7 | Auto-scroll só quando já está no fim | Rolar para o meio de uma conversa longa e mandar mensagem |
| CA8 | Fechar a janela e reabrir = sessão nova; F5 e queda de rede = mesma | Os três, na ordem |
| CA9 | Nova sessão zera **o servidor** | Clicar, mandar mensagem, e conferir no Postgres que o `sessao_id` é outro |
| CA10 | Conversa longa produz resumo e o James lembra do começo | Conversa longa, depois perguntar sobre algo dito no início |
| CA11 | Falha do resumidor não derruba o turno | Já coberto por teste; confirmar que nada quebrou ao vivo |
| CA12 | Histórico pagina, agrupa e busca | Rolar, buscar, conferir que não renderiza 1600 botões |
| CA13 | Memória lista notas; DashBoard vazio e franco | Abrir as duas |
| CA14 | Suítes verdes | Comandos abaixo |

Para o CA9, no Postgres:

```bash
docker exec -it james-postgres psql -U james -d james -c "SELECT id, titulo, criada_em FROM sessoes ORDER BY criada_em DESC LIMIT 3;"
```

- [ ] **Passo 3: Rodar as três suítes e colar a saída**

```bash
cd "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts" && go vet ./... && go test ./...
```

```bash
npm test --prefix web
```

```bash
npm run e2e --prefix web
```

**Cole a saída real.** Se algo falhou, mostre a falha em vez de suavizar.

- [ ] **Passo 4: Escrever o handoff**

Crie `docs/superpowers/HANDOFF-fase-7a-execucao.md`, no formato dos anteriores:

- Onde está tudo: spec, plano, handoff da fase anterior
- Faixa de commits em `main`
- O que existe agora que não existia
- **O que a verificação ao vivo provou, critério por critério, inclusive o que não passou**
- Achados abertos, não corrigidos
- Correções de fato em documentos anteriores, se houver
- Lição de processo com números: quantas correções o plano acumulou
- Ambiente confirmado
- O que vem depois (Fase 7B: instrumentação e DashBoard)

- [ ] **Passo 5: Atualizar a spec com o resultado**

Acrescente uma seção `12.1 Resultado da verificação ao vivo` à spec, no formato da 10.1 da Fase 6.

- [ ] **Passo 6: Commit**

```bash
git add docs/superpowers/HANDOFF-fase-7a-execucao.md docs/superpowers/specs/2026-08-11-james-fase-7a-aplicativo-design.md
git commit -m "docs: fecha a Fase 7A com a verificacao ao vivo do CA1 ao CA14"
```

---

## Cobertura da spec

| Seção da spec | Tasks |
|---|---|
| §4.1 `web/src` morre/nasce | 2, 3, 4, 5, 13 |
| §4.2 Barra lateral e contador | 2 |
| §4.3 Janela nativa | 6, 7 |
| §4.4 Mudanças no Go | 11, 12, 15, 16, 18 |
| §5.1 Orbe | 8, 9 |
| §5.2 Aprovação, atividade, tela | 3 |
| §6.1 Tela de Mensagens | 4 |
| §6.2 Ciclo de vida | 14 |
| §6.3 `nova_sessao` | 15 |
| §7.1 Migração 0002 | 10 |
| §7.2 Ancoragem no Postgres | 16 |
| §7.3 Fluxo | 18 |
| §7.4 Configuração | 17 |
| §8 Histórico | 12, 13 |
| §9 Design visual | 1 |
| §10 Tratamento de erro | 12 (erro sanitizado), 13 (falha da aba), 18 (degradação do resumo) |
| §11 Testes | todas; Playwright na 20 |
| §12 Critérios de aceitação | 21 |


