import { afterEach, describe, expect, it, vi } from "vitest";
import { arrastarJanela, calcularNovaPosicaoArrasto, moverJanela, posicaoJanela } from "./janela";

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

describe("calcularNovaPosicaoArrasto", () => {
  it("soma o deslocamento do mouse à posição original da janela", () => {
    const nova = calcularNovaPosicaoArrasto({ x: 100, y: 200 }, { x: 50, y: 50 }, { x: 80, y: 70 });
    expect(nova).toEqual({ x: 130, y: 220 });
  });

  it("mouse voltando à origem devolve a posição original", () => {
    const nova = calcularNovaPosicaoArrasto({ x: 300, y: 150 }, { x: 10, y: 10 }, { x: 10, y: 10 });
    expect(nova).toEqual({ x: 300, y: 150 });
  });

  it("deslocamento negativo (mouse foi pra esquerda/cima) também funciona", () => {
    const nova = calcularNovaPosicaoArrasto({ x: 100, y: 100 }, { x: 200, y: 200 }, { x: 150, y: 170 });
    expect(nova).toEqual({ x: 50, y: 70 });
  });
});

describe("arrastarJanela", () => {
  it("segue o mouse: busca a posição atual e chama moverJanela a cada frame pendente", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((url: string) => {
        if (url.endsWith("/posicao")) return Promise.resolve({ ok: true, json: async () => ({ x: 100, y: 200 }) });
        return Promise.resolve({ ok: true });
      }),
    );
    let executarQuadro: FrameRequestCallback = () => {};
    vi.stubGlobal(
      "requestAnimationFrame",
      vi.fn((cb: FrameRequestCallback) => {
        executarQuadro = cb;
        return 1;
      }),
    );
    vi.stubGlobal("cancelAnimationFrame", vi.fn());

    arrastarJanela({ screenX: 50, screenY: 50 } as MouseEvent);
    // posicaoJanela() empilha dois awaits (fetch + json) antes do .then()
    // em arrastarJanela rodar — um setTimeout(0) esvazia a fila de
    // microtasks inteira, o que um par de Promise.resolve() não garante.
    await new Promise((r) => setTimeout(r, 0));

    window.dispatchEvent(new MouseEvent("mousemove", { screenX: 80, screenY: 70 }));
    executarQuadro(0);

    const chamadaMover = (fetch as ReturnType<typeof vi.fn>).mock.calls.find(([url]) => url.includes("?x="));
    expect(chamadaMover?.[0]).toBe("http://127.0.0.1:17313/posicao?x=130&y=220");

    window.dispatchEvent(new MouseEvent("mouseup"));
  });

  it("solta o mouse encerra os listeners sem lançar", () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => ({ x: 0, y: 0 }) }));
    expect(() => {
      arrastarJanela({ screenX: 0, screenY: 0 } as MouseEvent);
      window.dispatchEvent(new MouseEvent("mouseup"));
      window.dispatchEvent(new MouseEvent("mousemove", { screenX: 10, screenY: 10 }));
    }).not.toThrow();
  });
});
