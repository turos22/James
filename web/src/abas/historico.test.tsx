import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AbaHistorico, preview } from "./historico";

vi.mock("../lib/api", () => ({
  listarSessoes: vi.fn(),
}));

import { listarSessoes } from "../lib/api";

// Sem isto, mock.calls acumula entre os `it`s (o projeto não liga
// clearMocks/resetMocks em nenhum lugar — nem vite.config.ts, nem
// vitest.setup.ts) e o teste "carrega mais páginas pelo cursor" indexa
// mock.calls[1] esperando que seja a 2ª chamada DESTE teste, quando na
// prática é a 2ª chamada do ARQUIVO inteiro. Furo achado ao rodar a suíte
// completa vs. isolada (`-t cursor`): isolado passava, no arquivo inteiro
// falhava 5/5 vezes de forma determinística — não era flake de timing.
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
