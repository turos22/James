import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AbaMensagens } from "./mensagens";
import { estadoInicial, type EstadoTurno } from "../estado/turno";

function comMensagens(): EstadoTurno {
  return {
    ...estadoInicial,
    sessaoAtual: "11111111-1111-1111-1111-111111111111",
    mensagens: [
      { papel: "usuario", texto: "que horas são?" },
      { papel: "james", texto: "são 14h32." },
    ],
  };
}

describe("Aba Mensagens", () => {
  it("desenha os balões da conversa", () => {
    render(
      <AbaMensagens
        estado={comMensagens()}
        conexao="conectado"
        tentativa={0}
        aoEnviar={vi.fn()}
        aoNovaSessao={vi.fn()}
      />,
    );

    expect(screen.getByText("que horas são?")).toBeTruthy();
    expect(screen.getByText("são 14h32.")).toBeTruthy();
  });

  // O "online" verde fixo do design de referência mentiria durante uma queda.
  it("mostra o estado real da conexão", () => {
    const { rerender } = render(
      <AbaMensagens estado={comMensagens()} conexao="conectado" tentativa={0} aoEnviar={vi.fn()} aoNovaSessao={vi.fn()} />,
    );
    expect(screen.getByText("online")).toBeTruthy();

    rerender(
      <AbaMensagens estado={comMensagens()} conexao="reconectando" tentativa={3} aoEnviar={vi.fn()} aoNovaSessao={vi.fn()} />,
    );
    expect(screen.getByText("Reconectando… (tentativa 3)")).toBeTruthy();

    rerender(
      <AbaMensagens estado={comMensagens()} conexao="fechado" tentativa={0} aoEnviar={vi.fn()} aoNovaSessao={vi.fn()} />,
    );
    expect(screen.getByText("desconectado")).toBeTruthy();
  });

  it("trava o campo quando a conexão não está de pé", () => {
    render(
      <AbaMensagens estado={comMensagens()} conexao="reconectando" tentativa={1} aoEnviar={vi.fn()} aoNovaSessao={vi.fn()} />,
    );

    expect((screen.getByPlaceholderText("Escreva para o James…") as HTMLInputElement).disabled).toBe(true);
  });

  it("envia com Enter e limpa o campo", () => {
    const aoEnviar = vi.fn();
    render(
      <AbaMensagens estado={comMensagens()} conexao="conectado" tentativa={0} aoEnviar={aoEnviar} aoNovaSessao={vi.fn()} />,
    );

    const campo = screen.getByPlaceholderText("Escreva para o James…") as HTMLInputElement;
    fireEvent.change(campo, { target: { value: "oi" } });
    fireEvent.keyDown(campo, { key: "Enter" });

    expect(aoEnviar).toHaveBeenCalledWith("oi");
    expect(campo.value).toBe("");
  });

  it("não envia mensagem em branco", () => {
    const aoEnviar = vi.fn();
    render(
      <AbaMensagens estado={comMensagens()} conexao="conectado" tentativa={0} aoEnviar={aoEnviar} aoNovaSessao={vi.fn()} />,
    );

    const campo = screen.getByPlaceholderText("Escreva para o James…");
    fireEvent.change(campo, { target: { value: "   " } });
    fireEvent.keyDown(campo, { key: "Enter" });

    expect(aoEnviar).not.toHaveBeenCalled();
  });

  it("pede sessão nova pelo botão", () => {
    const aoNovaSessao = vi.fn();
    render(
      <AbaMensagens estado={comMensagens()} conexao="conectado" tentativa={0} aoEnviar={vi.fn()} aoNovaSessao={aoNovaSessao} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Nova sessão" }));

    expect(aoNovaSessao).toHaveBeenCalled();
  });

  // O mockup de referência (painel-de-controle-james/messages-panel.tsx:86)
  // já tinha essa guarda; a primeira versão deste componente a perdeu na
  // tradução. Sem ela, confirmar um candidato de IME (japonês/chinês/
  // coreano) com Enter dispara o envio com texto parcial de composição.
  it("não envia durante composição de IME", () => {
    const aoEnviar = vi.fn();
    render(
      <AbaMensagens estado={comMensagens()} conexao="conectado" tentativa={0} aoEnviar={aoEnviar} aoNovaSessao={vi.fn()} />,
    );

    const campo = screen.getByPlaceholderText("Escreva para o James…") as HTMLInputElement;
    fireEvent.change(campo, { target: { value: "候補" } });
    fireEvent.keyDown(campo, { key: "Enter", isComposing: true });

    expect(aoEnviar).not.toHaveBeenCalled();
  });

  // conversa.tsx, memoria.tsx e sessoes.tsx já usam role="status" no mesmo
  // tipo de aviso — sem ele, quem usa leitor de tela só percebe a queda de
  // conexão quando o campo já está desabilitado.
  it("o indicador de conexão é um role=status", () => {
    render(
      <AbaMensagens estado={comMensagens()} conexao="reconectando" tentativa={2} aoEnviar={vi.fn()} aoNovaSessao={vi.fn()} />,
    );

    expect(screen.getByRole("status").textContent).toBe("Reconectando… (tentativa 2)");
  });

  // jsdom não faz layout de verdade: scrollHeight/clientHeight/scrollTop
  // precisam ser simulados na mão para exercitar as duas metades do
  // auto-scroll condicional (gruda vs. não gruda).
  it("rola pro fim quando chega mensagem nova e o usuário estava no fim", () => {
    const estado = comMensagens();
    const { container, rerender } = render(
      <AbaMensagens estado={estado} conexao="conectado" tentativa={0} aoEnviar={vi.fn()} aoNovaSessao={vi.fn()} />,
    );
    const lista = container.querySelector(".overflow-y-auto") as HTMLDivElement;
    Object.defineProperty(lista, "scrollHeight", { configurable: true, value: 1000 });
    Object.defineProperty(lista, "clientHeight", { configurable: true, value: 300 });
    Object.defineProperty(lista, "scrollTop", { configurable: true, writable: true, value: 0 });

    const novoEstado = { ...estado, mensagens: [...estado.mensagens, { papel: "james" as const, texto: "mais uma" }] };
    rerender(
      <AbaMensagens estado={novoEstado} conexao="conectado" tentativa={0} aoEnviar={vi.fn()} aoNovaSessao={vi.fn()} />,
    );

    expect(lista.scrollTop).toBe(1000);
  });

  it("não rola quando o usuário tinha subido na conversa", () => {
    const estado = comMensagens();
    const { container, rerender } = render(
      <AbaMensagens estado={estado} conexao="conectado" tentativa={0} aoEnviar={vi.fn()} aoNovaSessao={vi.fn()} />,
    );
    const lista = container.querySelector(".overflow-y-auto") as HTMLDivElement;
    Object.defineProperty(lista, "scrollHeight", { configurable: true, value: 1000 });
    Object.defineProperty(lista, "clientHeight", { configurable: true, value: 300 });
    // scrollTop=100 com scrollHeight=1000 e clientHeight=300 deixa 600px até
    // o fim — bem acima do limiar de 40px que conta como "grudado".
    Object.defineProperty(lista, "scrollTop", { configurable: true, writable: true, value: 100 });
    fireEvent.scroll(lista);

    const novoEstado = { ...estado, mensagens: [...estado.mensagens, { papel: "james" as const, texto: "mais uma" }] };
    rerender(
      <AbaMensagens estado={novoEstado} conexao="conectado" tentativa={0} aoEnviar={vi.fn()} aoNovaSessao={vi.fn()} />,
    );

    expect(lista.scrollTop).toBe(100);
  });
});
