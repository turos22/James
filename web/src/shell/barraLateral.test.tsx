import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { BarraLateral } from "./barraLateral";

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
});
