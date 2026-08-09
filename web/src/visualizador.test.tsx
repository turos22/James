import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { estadoInicial } from "./estado/turno";
import { escolherConteudo, Visualizador } from "./visualizador";

describe("escolherConteudo", () => {
  it("aprovação pendente vence tudo", () => {
    const estado = {
      ...estadoInicial,
      aprovacao: { id: "a1", ferramenta: "executar_comando_sistema", argumentos: {} },
      ferramentas: [{ nome: "ver_tela", estado: "concluida" as const, iniciadaEm: 0 }],
    };

    expect(escolherConteudo(estado, "saida qualquer")).toBe("aprovacao");
  });

  it("sem nada relevante, fica vazio", () => {
    expect(escolherConteudo(estadoInicial, null)).toBe("vazio");
  });

  it("última ferramenta é ver_tela e há captura: mostra a tela", () => {
    const estado = {
      ...estadoInicial,
      ferramentas: [{ nome: "ver_tela", estado: "concluida" as const, iniciadaEm: 0 }],
    };

    expect(escolherConteudo(estado, "superficie: DISPLAY1")).toBe("tela");
  });

  it("houve ver_tela antes, mas a última ferramenta é outra: não congela na tela", () => {
    // Este é o teste que prova a correção: saidaVerTela é a última captura
    // da SESSÃO inteira e persiste depois que o turno já foi para outra
    // ferramenta. Sob a regra antiga (saidaVerTela manda sozinho), este
    // caso voltaria "tela" — errado, porque a ferramenta mais recente do
    // turno é executar_comando_sistema, não ver_tela.
    const estado = {
      ...estadoInicial,
      ferramentas: [
        { nome: "ver_tela", estado: "concluida" as const, iniciadaEm: 0 },
        { nome: "executar_comando_sistema", estado: "concluida" as const, iniciadaEm: 1 },
      ],
    };

    expect(escolherConteudo(estado, "superficie: DISPLAY1")).toBe("ferramenta");
  });
});

describe("Visualizador", () => {
  it("mostra o pedido de aprovação com os dois botões", () => {
    const estado = {
      ...estadoInicial,
      aprovacao: { id: "a1", ferramenta: "executar_comando_sistema", argumentos: { comando: "del x" } },
    };
    const aoDecidir = vi.fn();

    render(<Visualizador estado={estado} saidaVerTela={null} aoDecidir={aoDecidir} />);

    expect(screen.getByText(/executar_comando_sistema/)).toBeTruthy();
    screen.getByRole("button", { name: "Aprovar" }).click();
    expect(aoDecidir).toHaveBeenCalledWith(true);
  });
});
