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

  // Pedido do Arthur (2026-08-15): o orbe ficar "mais bonitinho" com a marca
  // do James no centro.
  it("mostra a logo do James centralizada, sem atrapalhar a acessibilidade do canvas", () => {
    render(<Orbe analisador={null} />);

    const logo = screen.getByTestId("logo-james");
    // Decorativa: o rótulo de verdade do orbe continua sendo o aria-label
    // do canvas ("Visualização de áudio do James"), não a logo por cima.
    expect(logo.getAttribute("aria-hidden")).toBe("true");
    // Não pode interceptar clique/gesto — o canvas segue sendo o alvo.
    // getAttribute, não .className: em SVG, className é SVGAnimatedString,
    // não uma string comum — .toContain nele mente (compara um objeto).
    expect(logo.getAttribute("class")).toContain("pointer-events-none");
    expect(screen.getByLabelText("Visualização de áudio do James")).toBeTruthy();
  });

  // A logo veio de C:\Users\Arthur\Downloads\logo_james.png, traçada por
  // análise de imagem (contornos reais via OpenCV, não path desenhado à
  // mão) em 5 regiões de cor: amarelo, azul, verde, vermelho e o "JJ" cinza.
  // Essa contagem é a rede de segurança contra perder uma faixa de cor sem
  // querer numa edição futura.
  it("a logo tem as cinco regiões de cor extraídas do arquivo original", () => {
    render(<Orbe analisador={null} />);

    const logo = screen.getByTestId("logo-james");
    expect(logo.querySelectorAll("path").length).toBe(5);
  });
});
