import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Memoria } from "./memoria";

vi.mock("../lib/api", () => ({
  lerMemoria: vi.fn(),
}));

import { lerMemoria } from "../lib/api";

describe("Aba Memória", () => {
  it("lista as notas por assunto", async () => {
    vi.mocked(lerMemoria).mockResolvedValue({
      notas: [{ assunto: "projetos", conteudo: "James está na Fase 6", modificado: "2026-08-09T12:00:00Z" }],
      aviso: "",
    });

    render(<Memoria />);

    await waitFor(() => expect(screen.getByText("projetos")).toBeTruthy());
    expect(screen.getByText(/Fase 6/)).toBeTruthy();
  });

  it("mostra o aviso quando o vault falhou", async () => {
    vi.mocked(lerMemoria).mockResolvedValue({ notas: [], aviso: "não consegui ler a memória" });

    render(<Memoria />);

    await waitFor(() => expect(screen.getByRole("status").textContent).toContain("não consegui ler a memória"));
  });

  it("explica o vault vazio sem parecer erro", async () => {
    vi.mocked(lerMemoria).mockResolvedValue({ notas: [], aviso: "" });

    render(<Memoria />);

    await waitFor(() => expect(screen.getByText(/Nenhuma nota/)).toBeTruthy());
  });
});
