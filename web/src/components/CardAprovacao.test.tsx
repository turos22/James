import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { CardAprovacao } from "./CardAprovacao";

describe("CardAprovacao", () => {
  it("mostra o nome da ferramenta pedida", () => {
    render(<CardAprovacao ferramenta="manipular_interface" argumentos={{}} onDecidir={() => {}} />);
    expect(screen.getByText(/manipular_interface/)).toBeTruthy();
  });

  it("chama onDecidir(true) ao clicar em Aprovar", () => {
    const onDecidir = vi.fn();
    render(<CardAprovacao ferramenta="executar_comando_sistema" argumentos={{ comando: "ls" }} onDecidir={onDecidir} />);

    fireEvent.click(screen.getByText("Aprovar"));
    expect(onDecidir).toHaveBeenCalledWith(true);
  });

  it("chama onDecidir(false) ao clicar em Negar", () => {
    const onDecidir = vi.fn();
    render(<CardAprovacao ferramenta="x" argumentos={{}} onDecidir={onDecidir} />);

    fireEvent.click(screen.getByText("Negar"));
    expect(onDecidir).toHaveBeenCalledWith(false);
  });
});
