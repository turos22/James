import { afterEach, describe, expect, it, vi } from "vitest";
import { moverJanela, posicaoJanela } from "./janela";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("posicaoJanela", () => {
  it("devolve a posição decodificada do JSON", async () => {
    const fakeFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ x: 10, y: 20 }),
    });
    vi.stubGlobal("fetch", fakeFetch);

    const posicao = await posicaoJanela();

    expect(posicao).toEqual({ x: 10, y: 20 });
  });

  it("devolve null quando a resposta não é ok", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false }));
    expect(await posicaoJanela()).toBeNull();
  });

  it("devolve null quando o shell não está rodando (fetch falha)", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("ECONNREFUSED")));
    expect(await posicaoJanela()).toBeNull();
  });
});

describe("moverJanela", () => {
  it("faz POST /posicao com x e y arredondados na query", () => {
    const fakeFetch = vi.fn().mockResolvedValue({ ok: true });
    vi.stubGlobal("fetch", fakeFetch);

    moverJanela(12.7, 8.2);

    const [url, opcoes] = fakeFetch.mock.calls[0];
    expect(url).toBe("http://127.0.0.1:17313/posicao?x=13&y=8");
    expect(opcoes.method).toBe("POST");
  });

  it("não lança quando o shell não está rodando (fetch falha)", () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("ECONNREFUSED")));
    expect(() => moverJanela(0, 0)).not.toThrow();
  });
});
