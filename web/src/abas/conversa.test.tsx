import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Conversa } from "./conversa";
import { estadoInicial } from "../estado/turno";

const estadoComConversa = {
  ...estadoInicial,
  mensagens: [
    { papel: "usuario" as const, texto: "abre a calculadora" },
    { papel: "james" as const, texto: "Feito." },
  ],
};

describe("Aba Conversa", () => {
  it("renderiza as mensagens dos dois lados", () => {
    render(<Conversa estado={estadoComConversa} conexao="conectado" aoEnviar={vi.fn()} />);

    expect(screen.getByText("abre a calculadora")).toBeTruthy();
    expect(screen.getByText("Feito.")).toBeTruthy();
  });

  it("envia com Enter e limpa o campo", () => {
    const aoEnviar = vi.fn();
    render(<Conversa estado={estadoInicial} conexao="conectado" aoEnviar={aoEnviar} />);

    const campo = screen.getByPlaceholderText("Comando de voz ou texto...") as HTMLInputElement;
    fireEvent.change(campo, { target: { value: "oi" } });
    fireEvent.keyDown(campo, { key: "Enter" });

    expect(aoEnviar).toHaveBeenCalledWith("oi");
    expect(campo.value).toBe("");
  });

  it("não envia texto em branco", () => {
    const aoEnviar = vi.fn();
    render(<Conversa estado={estadoInicial} conexao="conectado" aoEnviar={aoEnviar} />);

    const campo = screen.getByPlaceholderText("Comando de voz ou texto...");
    fireEvent.change(campo, { target: { value: "   " } });
    fireEvent.keyDown(campo, { key: "Enter" });

    expect(aoEnviar).not.toHaveBeenCalled();
  });

  it("mostra a faixa de reconexão e bloqueia o envio", () => {
    const aoEnviar = vi.fn();
    render(<Conversa estado={estadoInicial} conexao="reconectando" aoEnviar={aoEnviar} />);

    expect(screen.getByRole("status").textContent).toContain("Reconectando");

    const campo = screen.getByPlaceholderText("Comando de voz ou texto...");
    expect((campo as HTMLInputElement).disabled).toBe(true);
  });

  it("mostra ferramenta em voo durante o turno", () => {
    const estado = {
      ...estadoInicial,
      emAndamento: true,
      ferramentas: [{ nome: "ver_tela", estado: "voando" as const, iniciadaEm: Date.now() }],
    };
    render(<Conversa estado={estado} conexao="conectado" aoEnviar={vi.fn()} />);

    expect(screen.getByText(/ver_tela/)).toBeTruthy();
  });

  it("mostra o número da tentativa quando maior que 1", () => {
    render(<Conversa estado={estadoInicial} conexao="reconectando" aoEnviar={vi.fn()} tentativa={4} />);

    expect(screen.getByRole("status").textContent).toContain("tentativa 4");
  });

  it("não polui a faixa quando a tentativa é 1 ou está ausente", () => {
    const { rerender } = render(
      <Conversa estado={estadoInicial} conexao="reconectando" aoEnviar={vi.fn()} tentativa={1} />
    );
    expect(screen.getByRole("status").textContent).not.toContain("tentativa");

    rerender(<Conversa estado={estadoInicial} conexao="reconectando" aoEnviar={vi.fn()} />);
    expect(screen.getByRole("status").textContent).not.toContain("tentativa");
  });

  it("mostra a faixa de encerrado sem prometer reconexão automática", () => {
    const aoEnviar = vi.fn();
    render(<Conversa estado={estadoInicial} conexao="fechado" aoEnviar={aoEnviar} />);

    const faixa = screen.getByRole("status").textContent ?? "";
    expect(faixa).not.toContain("Reconectando");
    expect(faixa).not.toContain("Conectando");

    const campo = screen.getByPlaceholderText("Comando de voz ou texto...");
    expect((campo as HTMLInputElement).disabled).toBe(true);
  });
});
