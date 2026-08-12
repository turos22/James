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
