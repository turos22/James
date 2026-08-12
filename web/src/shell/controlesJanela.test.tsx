import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ControlesJanela } from "./controlesJanela";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("Controles de janela", () => {
  // Fora do shell (navegador, celular) a porta 17313 não existe. Desenhar
  // três círculos que não fazem nada seria pior que não desenhar nada.
  it("não desenha nada quando o shell não responde", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("recusado")));

    const { container } = render(<ControlesJanela />);

    await waitFor(() => expect(container.querySelector("button")).toBeNull());
  });

  it("desenha os três controles quando o shell responde", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true }));

    render(<ControlesJanela />);

    await waitFor(() => expect(screen.getByRole("button", { name: "Fechar" })).toBeTruthy());
    expect(screen.getByRole("button", { name: "Minimizar" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Maximizar" })).toBeTruthy();
  });

  it("chama o endpoint certo em cada círculo", async () => {
    const fetchFalso = vi.fn().mockResolvedValue({ ok: true });
    vi.stubGlobal("fetch", fetchFalso);

    render(<ControlesJanela />);
    await waitFor(() => screen.getByRole("button", { name: "Minimizar" }));

    fireEvent.click(screen.getByRole("button", { name: "Minimizar" }));
    expect(fetchFalso).toHaveBeenCalledWith("http://127.0.0.1:17313/minimizar", { method: "POST" });

    fireEvent.click(screen.getByRole("button", { name: "Maximizar" }));
    expect(fetchFalso).toHaveBeenCalledWith("http://127.0.0.1:17313/maximizar", { method: "POST" });

    fireEvent.click(screen.getByRole("button", { name: "Fechar" }));
    expect(fetchFalso).toHaveBeenCalledWith("http://127.0.0.1:17313/fechar", { method: "POST" });
  });
});
