import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ALTURA_EXPANDIDA, LARGURA_EXPANDIDA, Orbe, TAMANHO_RECOLHIDO } from "./orbe";

vi.mock("../lib/janela", () => ({
  redimensionarJanela: vi.fn(),
  posicaoJanela: vi.fn().mockResolvedValue(null),
  moverJanela: vi.fn(),
}));

import { redimensionarJanela } from "../lib/janela";

describe("Orbe", () => {
  it("abre pedindo a janela expandida", () => {
    const aoAlternar = vi.fn();
    render(<Orbe aberto={false} aoAlternar={aoAlternar} />);

    fireEvent.click(screen.getByAltText("James"));

    expect(aoAlternar).toHaveBeenCalledWith(true);
    expect(redimensionarJanela).toHaveBeenCalledWith(LARGURA_EXPANDIDA, ALTURA_EXPANDIDA);
  });

  it("fecha pedindo a janela recolhida", () => {
    const aoAlternar = vi.fn();
    render(<Orbe aberto={true} aoAlternar={aoAlternar} />);

    fireEvent.click(screen.getByAltText("James"));

    expect(aoAlternar).toHaveBeenCalledWith(false);
    expect(redimensionarJanela).toHaveBeenCalledWith(TAMANHO_RECOLHIDO, TAMANHO_RECOLHIDO);
  });
});
