import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ClienteWS } from "./ws";

class FakeWebSocket {
  static instancias: FakeWebSocket[] = [];
  enviados: string[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  constructor(public url: string) {
    FakeWebSocket.instancias.push(this);
  }
  send(dados: string) {
    this.enviados.push(dados);
  }
  close() {}
}

beforeEach(() => {
  FakeWebSocket.instancias = [];
  vi.stubGlobal("WebSocket", FakeWebSocket);
});

describe("ClienteWS", () => {
  it("enviar codifica o envelope como JSON", () => {
    const cliente = new ClienteWS("ws://localhost/ws");
    const socket = FakeWebSocket.instancias[0];
    // Desde a Task 8 o cliente enfileira o envio até a conexão abrir (é o
    // próprio contrato testado em "ClienteWS resiliente"), então o mock
    // precisa simular a abertura antes de verificar o que foi enviado.
    socket.onopen?.();
    cliente.enviar("mensagem", "1", { texto: "oi" });

    expect(JSON.parse(socket.enviados[0])).toEqual({ tipo: "mensagem", id: "1", dados: { texto: "oi" } });
  });

  it("on despacha o manipulador certo pelo tipo do envelope", () => {
    const cliente = new ClienteWS("ws://localhost/ws");
    const recebidos: unknown[] = [];
    cliente.on("resposta_final", (dados) => recebidos.push(dados));

    const socket = FakeWebSocket.instancias[0];
    socket.onmessage?.({ data: JSON.stringify({ tipo: "resposta_final", id: "1", dados: { texto: "oi" } }) });

    expect(recebidos).toEqual([{ texto: "oi" }]);
  });

  it("não despacha manipulador de outro tipo", () => {
    const cliente = new ClienteWS("ws://localhost/ws");
    const recebidos: unknown[] = [];
    cliente.on("resposta_final", (dados) => recebidos.push(dados));

    const socket = FakeWebSocket.instancias[0];
    socket.onmessage?.({ data: JSON.stringify({ tipo: "erro", id: "1", dados: { mensagem: "falhou" } }) });

    expect(recebidos).toEqual([]);
  });
});

class SocketFalso {
  static ultimo: SocketFalso | null = null;
  static abertos = 0;

  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  enviados: string[] = [];
  fechado = false;

  constructor(public url: string) {
    SocketFalso.ultimo = this;
    SocketFalso.abertos++;
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
}

describe("ClienteWS resiliente", () => {
  beforeEach(() => {
    SocketFalso.ultimo = null;
    SocketFalso.abertos = 0;
    vi.stubGlobal("WebSocket", SocketFalso as unknown as typeof WebSocket);
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("enfileira envio feito antes de abrir e descarrega ao abrir", () => {
    const cliente = new ClienteWS("ws://teste/ws");
    cliente.enviar("mensagem", "id1", { texto: "oi" });

    expect(SocketFalso.ultimo!.enviados).toHaveLength(0);

    SocketFalso.ultimo!.abrir();

    expect(SocketFalso.ultimo!.enviados).toHaveLength(1);
    expect(JSON.parse(SocketFalso.ultimo!.enviados[0])).toEqual({
      tipo: "mensagem",
      id: "id1",
      dados: { texto: "oi" },
    });
  });

  it("reconecta com backoff depois de uma queda", () => {
    const cliente = new ClienteWS("ws://teste/ws");
    SocketFalso.ultimo!.abrir();
    expect(SocketFalso.abertos).toBe(1);

    SocketFalso.ultimo!.close();
    expect(SocketFalso.abertos).toBe(1);

    vi.advanceTimersByTime(1000);
    expect(SocketFalso.abertos).toBe(2);

    cliente.fechar();
  });

  // Não estava nos cinco testes pedidos pelo plano, mas o backoff exponencial
  // com teto é comportamento central da task (evitar martelar o servidor numa
  // queda prolongada) e só é exercitado uma vez pelo teste acima (1s). Sem
  // isso, um bug que fizesse a espera não dobrar — ou nunca saturar — passaria
  // batido.
  it("dobra a espera a cada tentativa e satura em 15s", () => {
    const cliente = new ClienteWS("ws://teste/ws");
    SocketFalso.ultimo!.abrir();
    expect(SocketFalso.abertos).toBe(1);

    const esperasEsperadas = [1000, 2000, 4000, 8000, 15000, 15000];
    let abertosEsperado = 1;

    for (const espera of esperasEsperadas) {
      SocketFalso.ultimo!.close();
      // Um instante antes do prazo: ainda não reconectou.
      vi.advanceTimersByTime(espera - 1);
      expect(SocketFalso.abertos).toBe(abertosEsperado);

      vi.advanceTimersByTime(1);
      abertosEsperado++;
      expect(SocketFalso.abertos).toBe(abertosEsperado);
    }

    cliente.fechar();
  });

  it("publica o estado da conexão", () => {
    const estados: string[] = [];
    const cliente = new ClienteWS("ws://teste/ws");
    cliente.aoMudarEstado((e) => estados.push(e));

    SocketFalso.ultimo!.abrir();
    SocketFalso.ultimo!.close();

    expect(estados).toContain("conectado");
    expect(estados).toContain("reconectando");

    cliente.fechar();
  });

  // Também não pedido pelo plano. O teste "enfileira envio feito antes de
  // abrir" só cobre a mensagem feita antes da primeira conexão — mas o bug
  // relatado na Fase 5 (resposta completa no backend, tela muda) é mais
  // provável de acontecer com uma queda no MEIO de uma sessão em uso, não
  // no arranque. Sem este teste, um bug que só descarregasse a fila do
  // "estado inicial" e não da fila acumulada numa queda passaria batido.
  it("mensagem enfileirada durante uma queda sai só depois da reconexão", () => {
    const cliente = new ClienteWS("ws://teste/ws");
    SocketFalso.ultimo!.abrir();

    SocketFalso.ultimo!.close();
    cliente.enviar("mensagem", "id2", { texto: "durante a queda" });

    expect(SocketFalso.abertos).toBe(1);

    vi.advanceTimersByTime(1000);
    expect(SocketFalso.abertos).toBe(2);
    expect(SocketFalso.ultimo!.enviados).toHaveLength(0);

    SocketFalso.ultimo!.abrir();

    expect(SocketFalso.ultimo!.enviados).toHaveLength(1);
    expect(JSON.parse(SocketFalso.ultimo!.enviados[0])).toEqual({
      tipo: "mensagem",
      id: "id2",
      dados: { texto: "durante a queda" },
    });

    cliente.fechar();
  });

  it("fechar de propósito não reconecta", () => {
    const cliente = new ClienteWS("ws://teste/ws");
    SocketFalso.ultimo!.abrir();
    cliente.fechar();

    vi.advanceTimersByTime(30000);

    expect(SocketFalso.abertos).toBe(1);
  });

  it("ignora evento desconhecido sem lançar", () => {
    const cliente = new ClienteWS("ws://teste/ws");
    SocketFalso.ultimo!.abrir();

    expect(() =>
      SocketFalso.ultimo!.onmessage!({ data: JSON.stringify({ tipo: "inventado", id: "x", dados: {} }) })
    ).not.toThrow();

    cliente.fechar();
  });
});
