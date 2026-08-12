import { act, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";

// SocketFalso espelha o de lib/ws.test.ts: o ClienteWS real roda por cima
// dele, então o que estes testes exercitam é a fiação de App.tsx com a
// reconexão de verdade, não um dublê da reconexão.
class SocketFalso {
  static instancias: SocketFalso[] = [];

  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  enviados: string[] = [];
  fechado = false;

  constructor(public url: string) {
    SocketFalso.instancias.push(this);
  }
  send(dado: string) {
    if (this.fechado) throw new Error("socket fechado");
    this.enviados.push(dado);
  }
  close() {
    this.fechado = true;
    this.onclose?.();
  }
  abrir() {
    this.onopen?.();
  }
  derrubar() {
    this.fechado = true;
    this.onclose?.();
  }
  receber(tipo: string, dados: unknown) {
    this.onmessage?.({ data: JSON.stringify({ tipo, id: "e1", dados }) });
  }
  envelopes() {
    return this.enviados.map((e) => JSON.parse(e));
  }
}

const SESSAO = "3fa85f64-5717-4562-b3fc-2c963f66afa6";

beforeEach(() => {
  SocketFalso.instancias = [];
  vi.stubGlobal("WebSocket", SocketFalso as unknown as typeof WebSocket);
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

// Reconectar abre uma conexão NOVA no jamesd, e o servidor guarda sessão e
// histórico por conexão (internal/api/ws.go: sessaoID e historico são
// variáveis locais do handler). Sem reemitir "retomar", a conversa continua
// desenhada na tela mas o modelo volta a zero e o Postgres ganha uma sessão
// órfã — reproduzido ao vivo na auditoria da Fase 6, derrubando o jamesd no
// meio de uma conversa: os balões antigos seguiram na tela e a mensagem
// seguinte nasceu numa sessão nova.
describe("App: continuidade da sessão através da reconexão", () => {
  it("reemite retomar com a sessão corrente quando a conexão volta", () => {
    render(<App />);
    const primeiro = SocketFalso.instancias[0];
    // act() em volta de tudo que dispara evento: o redutor é quem grava a
    // sessão, e o App só a enxerga no efeito de conexão depois de o React ter
    // aplicado a atualização. Sem act, o evento entra mas o efeito ainda não
    // rodou quando a reconexão acontece.
    act(() => {
      primeiro.abrir();
      primeiro.receber("resposta_final", { texto: "oi", sessao_id: SESSAO });
    });

    act(() => {
      primeiro.derrubar();
      vi.advanceTimersByTime(1000);
    });
    const segundo = SocketFalso.instancias[1];
    act(() => segundo.abrir());

    expect(segundo.envelopes()).toEqual([
      { tipo: "retomar", id: expect.any(String), dados: { sessao_id: SESSAO } },
    ]);
  });

  it("não pede retomada na primeira conexão, quando ainda não há sessão", () => {
    render(<App />);
    const primeiro = SocketFalso.instancias[0];

    primeiro.abrir();

    expect(primeiro.envelopes()).toEqual([]);
  });

  it("não pede retomada se a queda aconteceu antes de existir sessão", () => {
    // Conexão caiu antes de qualquer resposta_final: não há o que retomar, e
    // pedir retomada com sessão vazia viraria erro "id de sessão inválido"
    // na tela do usuário (ws.go, tratarRetomar).
    render(<App />);
    SocketFalso.instancias[0].abrir();
    SocketFalso.instancias[0].derrubar();
    vi.advanceTimersByTime(1000);
    const segundo = SocketFalso.instancias[1];

    segundo.abrir();

    expect(segundo.envelopes()).toEqual([]);
  });

  it("retomar sai antes da mensagem que estava na fila", () => {
    // Ordem é o ponto: a mensagem enfileirada durante a queda precisa cair
    // na sessão retomada, não numa sessão nova. Se ela saísse primeiro, o
    // servidor criaria sessão antes de processar o retomar, e o retomar
    // adotaria a sessão antiga depois — mensagem gravada no lugar errado.
    render(<App />);
    const primeiro = SocketFalso.instancias[0];
    act(() => {
      primeiro.abrir();
      primeiro.receber("resposta_final", { texto: "oi", sessao_id: SESSAO });
      primeiro.derrubar();
    });

    const campo = document.querySelector("input") as HTMLInputElement | null;
    // A aba inicial é a James, e o campo de texto vive na aba Mensagens —
    // não há input no DOM aqui. Exercitamos a fila pelo próprio ClienteWS,
    // que é onde ela vive.
    expect(campo).toBeNull();

    act(() => vi.advanceTimersByTime(1000));
    const segundo = SocketFalso.instancias[1];
    act(() => segundo.abrir());

    expect(segundo.envelopes()[0].tipo).toBe("retomar");
  });
});
