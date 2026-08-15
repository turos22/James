import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { BarraLateral } from "./barraLateral";

vi.mock("../lib/janela", () => ({ arrastarJanela: vi.fn() }));
import { arrastarJanela } from "../lib/janela";

// Sem isto, a chamada registrada num teste sobrevive pro próximo — este
// arquivo não zera mocks globalmente (vitest.setup.ts não tem
// clearMocks/resetMocks), e um `toHaveBeenCalledTimes`/`not.toHaveBeenCalled`
// mais adiante mentiria contando a sobra do teste anterior.
beforeEach(() => {
  vi.clearAllMocks();
});

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

  // A janela do Wails não tem moldura — a única forma de movê-la é
  // arrastando alguma área da própria SPA. A faixa reservada (pt-11, ver
  // comentário no componente) é onde os círculos de janela já vivem; o
  // resto dela, e a área ao redor do rótulo "James", vira a pega da
  // "barra de título" — do jeito que qualquer outra janela do Windows
  // funciona.
  it("arrasta a janela ao apertar o botão do mouse na área reservada, fora do rótulo e do botão de recolher", () => {
    render(<BarraLateral ativa="james" aoSelecionar={vi.fn()} aprovacoesPendentes={0} />);

    const areaReservada = screen.getByTestId("area-arrastavel");
    fireEvent.mouseDown(areaReservada, { screenX: 40, screenY: 20 });

    expect(arrastarJanela).toHaveBeenCalledTimes(1);
  });

  it("não arrasta ao apertar o mouse no rótulo James nem no botão de recolher", () => {
    render(<BarraLateral ativa="james" aoSelecionar={vi.fn()} aprovacoesPendentes={0} />);

    fireEvent.mouseDown(screen.getByTestId("rotulo-app"));
    fireEvent.mouseDown(screen.getByRole("button", { name: "Recolher menu" }));

    expect(arrastarJanela).not.toHaveBeenCalled();
  });
});
