import { beforeEach, describe, expect, it, vi } from "vitest";
import { ClienteWS } from "./ws";

class FakeWebSocket {
  static instancias: FakeWebSocket[] = [];
  enviados: string[] = [];
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
    cliente.enviar("mensagem", "1", { texto: "oi" });

    const socket = FakeWebSocket.instancias[0];
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
