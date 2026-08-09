import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ABAS, Trilha } from "./trilha";

describe("Trilha", () => {
  it("mostra as cinco abas", () => {
    render(<Trilha ativa="conversa" aoTrocar={vi.fn()} />);

    expect(ABAS).toHaveLength(5);
    for (const aba of ABAS) {
      expect(screen.getByRole("tab", { name: aba.rotulo })).toBeTruthy();
    }
  });

  it("marca a aba ativa e troca ao clicar", () => {
    const aoTrocar = vi.fn();
    render(<Trilha ativa="conversa" aoTrocar={aoTrocar} />);

    expect(screen.getByRole("tab", { name: "Conversa" }).getAttribute("aria-selected")).toBe("true");

    fireEvent.click(screen.getByRole("tab", { name: "Sessões" }));

    expect(aoTrocar).toHaveBeenCalledWith("sessoes");
  });
});
