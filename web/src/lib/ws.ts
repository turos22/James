import { ehEventoConhecido } from "./eventos";

export interface Envelope {
  tipo: string;
  id: string;
  dados: unknown;
}

export type EstadoConexao = "conectando" | "conectado" | "reconectando" | "fechado";

type Manipulador = (dados: any) => void;
type ObservadorEstado = (estado: EstadoConexao, tentativa: number) => void;

const ESPERA_INICIAL_MS = 1000;
const ESPERA_MAXIMA_MS = 15000;

// ClienteWS existia com 35 linhas e sem onclose, onerror ou reconexão: o
// socket caía e a página ficava muda, sem nada na tela. Era o candidato
// direto para as respostas que completavam no backend e nunca apareciam.
export class ClienteWS {
  private socket: WebSocket | null = null;
  private manipuladores = new Map<string, Manipulador[]>();
  private observadores: ObservadorEstado[] = [];
  private fila: Envelope[] = [];
  private tentativa = 0;
  private encerrado = false;
  private timer: ReturnType<typeof setTimeout> | null = null;
  // Estado rastreado à mão em vez de ler socket.readyState/WebSocket.OPEN:
  // o fake de teste não tem readyState nem a constante estática OPEN, então
  // a comparação com o global real daria sempre true no teste (undefined ===
  // undefined) e a fila nunca seria usada. Rastrear internamente também
  // deixa o comportamento idêntico em teste e produção.
  private aberto = false;

  constructor(private url: string) {
    this.conectar();
  }

  private conectar() {
    this.aberto = false;
    this.publicar(this.tentativa === 0 ? "conectando" : "reconectando");
    const socket = new WebSocket(this.url);
    this.socket = socket;

    socket.onopen = () => {
      this.tentativa = 0;
      // Precisa virar true antes de descarregarFila(): descarregarFila chama
      // escrever(), e escrever() só envia de verdade quando `aberto` é true.
      this.aberto = true;
      this.publicar("conectado");
      this.descarregarFila();
    };

    socket.onmessage = (evento: MessageEvent) => {
      let envelope: Envelope;
      try {
        envelope = JSON.parse(evento.data);
      } catch {
        console.warn("envelope ilegível do jamesd", evento.data);
        return;
      }
      // Evento desconhecido nunca derruba a tela: o backend pode ganhar um
      // evento novo antes de a SPA ser atualizada.
      if (!ehEventoConhecido(envelope.tipo)) {
        console.warn("evento desconhecido, ignorado:", envelope.tipo);
        return;
      }
      for (const fn of this.manipuladores.get(envelope.tipo) ?? []) fn(envelope.dados);
    };

    // onerror sempre é seguido de onclose. A reconexão é agendada só lá,
    // para não haver dois caminhos agendando o mesmo timer.
    socket.onerror = () => {};

    socket.onclose = () => {
      this.aberto = false;
      if (this.encerrado) return;
      this.agendarReconexao();
    };
  }

  private agendarReconexao() {
    this.tentativa++;
    const espera = Math.min(ESPERA_INICIAL_MS * 2 ** (this.tentativa - 1), ESPERA_MAXIMA_MS);
    this.publicar("reconectando");
    this.timer = setTimeout(() => this.conectar(), espera);
  }

  private descarregarFila() {
    const pendentes = this.fila;
    this.fila = [];
    for (const envelope of pendentes) this.escrever(envelope);
  }

  private escrever(envelope: Envelope) {
    if (this.aberto && this.socket) {
      this.socket.send(JSON.stringify(envelope));
      return;
    }
    this.fila.push(envelope);
  }

  private publicar(estado: EstadoConexao) {
    for (const fn of this.observadores) fn(estado, this.tentativa);
  }

  on(tipo: string, manipulador: Manipulador) {
    const lista = this.manipuladores.get(tipo) ?? [];
    lista.push(manipulador);
    this.manipuladores.set(tipo, lista);
  }

  aoMudarEstado(observador: ObservadorEstado) {
    this.observadores.push(observador);
  }

  // enviar nunca lança: mensagem feita com o socket fechado espera a
  // reconexão em vez de virar exceção não tratada, que era o que acontecia
  // antes e deixava o campo de texto aparentemente sem efeito.
  enviar(tipo: string, id: string, dados: unknown) {
    this.escrever({ tipo, id, dados });
  }

  fechar() {
    this.encerrado = true;
    if (this.timer) clearTimeout(this.timer);
    this.publicar("fechado");
    this.socket?.close();
  }
}
