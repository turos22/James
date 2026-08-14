import { expect, test, type Page } from "@playwright/test";

// ARMADILHA JÁ DOCUMENTADA NESTE PROJETO: input controlado do React só
// atualiza estado quando o onChange sintético dispara. `fill` e `type` do
// Playwright NÃO o disparam — o campo mostra o texto, o estado do React
// continua vazio, e a conclusão errada é "o app está quebrado". Já custou
// tempo duas vezes aqui.
async function digitarEmInputControlado(seletor: string, texto: string, page: Page) {
  await page.locator(seletor).evaluate((elemento, valor) => {
    const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, "value")!.set!;
    setter.call(elemento, valor);
    elemento.dispatchEvent(new Event("input", { bubbles: true }));
  }, texto);
}

// O Postgres desta suíte é o banco de verdade do jamesd (não um container
// descartável por rodada), e acumula sessão de execuções anteriores e de
// outras suítes (ex.: internal/api/rest_test.go). Dois efeitos colaterais
// documentados por experiência real rodando isto:
//   1. getByRole(..., { name }) faz correspondência por SUBSTRING por
//      padrão — um item de histórico antigo com título tipo "sessão pra
//      mensagens via REST 15:32" já colidiu com getByRole("button", { name:
//      "Mensagens" }) tentando clicar no botão da barra lateral. Por isso
//      todo clique de navegação lateral abaixo é escopado ao <nav> — a
//      barra lateral nunca compartilha árvore com o conteúdo do histórico.
//   2. Texto de mensagem fixo ("primeira conversa") colide com a MESMA
//      sessão criada numa rodada anterior da própria suíte, e
//      getByText(...) estoura "strict mode violation" com 2+ matches. Por
//      isso o texto de teste leva uma marca única por execução.
function navegacao(page: Page) {
  return page.getByRole("navigation", { name: "Navegação principal" });
}

const marca = `${Date.now()}`;

test.describe("James como aplicativo", () => {
  test("abre direto no shell, sem bolinha", async ({ page }) => {
    await page.goto("/");

    await expect(navegacao(page)).toBeVisible();
    await expect(page.getByLabel("Visualização de áudio do James")).toBeVisible();
  });

  test("troca de aba pela barra lateral", async ({ page }) => {
    await page.goto("/");

    await navegacao(page).getByRole("button", { name: "Histórico" }).click();
    await expect(page.getByPlaceholder("Buscar conversas…")).toBeVisible();

    await navegacao(page).getByRole("button", { name: "DashBoard" }).click();
    await expect(page.getByText(/Fase 7B/)).toBeVisible();

    await navegacao(page).getByRole("button", { name: "Mensagens" }).click();
    await expect(page.getByPlaceholder("Escreva para o James…")).toBeVisible();
  });

  test("envia mensagem e recebe resposta", async ({ page }) => {
    await page.goto("/");
    await navegacao(page).getByRole("button", { name: "Mensagens" }).click();
    await expect(page.getByText("online")).toBeVisible();

    const pergunta = `diga apenas: pong ${marca}`;
    await digitarEmInputControlado('input[placeholder="Escreva para o James…"]', pergunta, page);
    await page.getByLabel("Enviar mensagem").click();

    await expect(page.getByText(pergunta)).toBeVisible();
    // Chamada real ao Gemini: dá folga de tempo.
    // Mira especificamente na bolha do ASSISTENTE (wrapper "justify-start").
    // A mensagem do próprio usuário ("diga apenas: pong ...") também contém
    // a substring "pong" — um locator genérico text=/pong/i casaria com a
    // bolha do usuário e passaria de imediato, sem nunca esperar a resposta
    // real do Gemini. Já aconteceu aqui: o teste passava em ~200ms.
    await expect(page.locator(".justify-start", { hasText: /pong/i }).last()).toBeVisible({ timeout: 25_000 });
  });

  test("abre sessão nova e o histórico registra as duas", async ({ page }) => {
    await page.goto("/");
    await navegacao(page).getByRole("button", { name: "Mensagens" }).click();

    const textoPrimeira = `primeira conversa ${marca}`;
    await digitarEmInputControlado('input[placeholder="Escreva para o James…"]', textoPrimeira, page);
    await page.getByLabel("Enviar mensagem").click();
    await expect(page.getByText(textoPrimeira)).toBeVisible();

    await page.getByRole("button", { name: "Nova sessão" }).click();
    await expect(page.getByText(textoPrimeira)).toHaveCount(0);

    await navegacao(page).getByRole("button", { name: "Histórico" }).click();
    await expect(page.getByText(textoPrimeira)).toBeVisible({ timeout: 10_000 });
  });

  test("retoma uma conversa pelo histórico", async ({ page }) => {
    await page.goto("/");
    await navegacao(page).getByRole("button", { name: "Histórico" }).click();

    // Escopado em "main": a barra lateral TAMBÉM é <nav><ul><li><button>,
    // e vem antes do <main> no DOM. "ul li button" sem escopo casava com o
    // primeiro botão da navegação ("James"), não com um item do histórico —
    // já aconteceu aqui e travava esperando um "span span" que nunca existe
    // na barra lateral.
    const primeira = page.locator("main ul li button").first();
    await expect(primeira).toBeVisible({ timeout: 10_000 });
    const titulo = (await primeira.locator("span span").first().textContent()) ?? "";

    await primeira.click();
    await navegacao(page).getByRole("button", { name: "Mensagens" }).click();

    await expect(page.getByText(titulo.replace(/…$/, "").slice(0, 30), { exact: false })).toBeVisible();
  });
});
