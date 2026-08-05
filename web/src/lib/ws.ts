export interface Envelope {
  tipo: string;
  id: string;
  dados: unknown;
}

type Manipulador = (dados: any) => void;

export class ClienteWS {
  private socket: WebSocket;
  private manipuladores = new Map<string, Manipulador[]>();

  constructor(url: string) {
    this.socket = new WebSocket(url);
    this.socket.onmessage = (evento: MessageEvent) => {
      const envelope: Envelope = JSON.parse(evento.data);
      const lista = this.manipuladores.get(envelope.tipo) ?? [];
      for (const fn of lista) fn(envelope.dados);
    };
  }

  on(tipo: string, manipulador: Manipulador) {
    const lista = this.manipuladores.get(tipo) ?? [];
    lista.push(manipulador);
    this.manipuladores.set(tipo, lista);
  }

  enviar(tipo: string, id: string, dados: unknown) {
    this.socket.send(JSON.stringify({ tipo, id, dados }));
  }

  fechar() {
    this.socket.close();
  }
}
