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
});
