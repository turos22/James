import { afterEach, describe, expect, it, vi } from "vitest";
import { criarSessao, listarMensagens, listarSessoes } from "./api";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("listarSessoes", () => {
  it("faz GET /api/sessoes e devolve o JSON", async () => {
    const fakeFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => [{ ID: "1", Titulo: "sessão" }],
    });
    vi.stubGlobal("fetch", fakeFetch);

    const sessoes = await listarSessoes();

    expect(fakeFetch).toHaveBeenCalledWith("/api/sessoes");
    expect(sessoes).toEqual([{ ID: "1", Titulo: "sessão" }]);
  });

  it("lança erro quando a resposta não é ok", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 500 }));
    await expect(listarSessoes()).rejects.toThrow();
  });
});

describe("criarSessao", () => {
  it("faz POST /api/sessoes com o título", async () => {
    const fakeFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ ID: "1", Titulo: "nova" }),
    });
    vi.stubGlobal("fetch", fakeFetch);

    await criarSessao("nova");

    const [url, opcoes] = fakeFetch.mock.calls[0];
    expect(url).toBe("/api/sessoes");
    expect(opcoes.method).toBe("POST");
    expect(JSON.parse(opcoes.body)).toEqual({ titulo: "nova" });
  });
});

describe("listarMensagens", () => {
  it("faz GET /api/sessoes/{id}/mensagens", async () => {
    const fakeFetch = vi.fn().mockResolvedValue({ ok: true, json: async () => [] });
    vi.stubGlobal("fetch", fakeFetch);

    await listarMensagens("abc-123");

    expect(fakeFetch).toHaveBeenCalledWith("/api/sessoes/abc-123/mensagens");
  });
});
