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
  });

  // O contador existe porque, estando em outra aba, você não veria o James
  // travado esperando um sim — e o timeout de 300s corre igual (§4.2).
  it("mostra o contador de aprovação pendente na aba James", () => {
    render(<BarraLateral ativa="mensagens" aoSelecionar={vi.fn()} aprovacoesPendentes={1} />);

    expect(screen.getByLabelText("1 aprovação pendente")).toBeTruthy();
  });

  it("não mostra contador quando não há aprovação", () => {
    render(<BarraLateral ativa="mensagens" aoSelecionar={vi.fn()} aprovacoesPendentes={0} />);

    expect(screen.queryByLabelText(/aprovaç(ão|ões) pendente/)).toBeNull();
  });
});
