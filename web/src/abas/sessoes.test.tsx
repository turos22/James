import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Sessoes } from "./sessoes";

vi.mock("../lib/api", () => ({
  listarSessoes: vi.fn(),
}));

import { listarSessoes } from "../lib/api";

const SESSOES = [
  { ID: "11111111-1111-1111-1111-111111111111", Titulo: "abre a calculadora", CriadaEm: "2026-08-08T22:00:00Z", AtualizadaEm: "2026-08-08T22:05:00Z" },
  { ID: "22222222-2222-2222-2222-222222222222", Titulo: "pesquisa sobre Go", CriadaEm: "2026-08-07T10:00:00Z", AtualizadaEm: "2026-08-07T10:30:00Z" },
];

describe("Aba Sessões", () => {
  it("lista as sessões do banco", async () => {
    vi.mocked(listarSessoes).mockResolvedValue(SESSOES);

    render(<Sessoes aoRetomar={vi.fn()} sessaoAtual={null} />);

    await waitFor(() => expect(screen.getByText("abre a calculadora")).toBeTruthy());
    expect(screen.getByText("pesquisa sobre Go")).toBeTruthy();
  });

  it("retoma a sessão clicada", async () => {
    vi.mocked(listarSessoes).mockResolvedValue(SESSOES);
    const aoRetomar = vi.fn();

    render(<Sessoes aoRetomar={aoRetomar} sessaoAtual={null} />);

    await waitFor(() => screen.getByText("abre a calculadora"));
    fireEvent.click(screen.getByText("abre a calculadora"));

    expect(aoRetomar).toHaveBeenCalledWith("11111111-1111-1111-1111-111111111111");
  });

  it("marca a sessão atual", async () => {
    vi.mocked(listarSessoes).mockResolvedValue(SESSOES);

    render(<Sessoes aoRetomar={vi.fn()} sessaoAtual="22222222-2222-2222-2222-222222222222" />);

    await waitFor(() => screen.getByText("pesquisa sobre Go"));
    expect(screen.getByText("pesquisa sobre Go").closest("button")!.getAttribute("aria-current")).toBe("true");
  });
});
