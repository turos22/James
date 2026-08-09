// Tipos dos eventos que o jamesd emite pelo WebSocket. Um lugar só: antes
// da Fase 6 cada handler inventava sua própria forma inline em App.tsx, e
// três dos sete eventos simplesmente não tinham tratamento nenhum.

export interface DadosRespostaFinal {
  texto: string;
  // A sessão vem no próprio resposta_final desde a Task 5. Sem isso, o
  // cliente só descobriria em que conversa está depois de pedir `retomar`
  // — numa conversa nova, o id nasce no servidor e nunca chegaria aqui, e a
  // aba Sessões não teria como destacar a conversa em andamento. Confirmado
  // ao vivo em ws.go:121-124 e dispositivo.go:134-137: os dois caminhos de
  // emissão mandam texto e sessao_id juntos.
  sessao_id: string;
}

export interface DadosErro {
  mensagem: string;
}

export interface DadosAudioPronto {
  id: string;
}

export interface DadosFerramentaIniciada {
  nome: string;
}

export interface DadosFerramentaConcluida {
  nome: string;
  erro: string;
}

export interface DadosAprovacaoRequerida {
  // O id vem duplicado dentro de `dados` de propósito: o cliente WS só
  // repassa `dados` aos manipuladores, então o id do envelope não chega
  // sozinho ao handler. Ver o comentário em internal/api/ws.go.
  id: string;
  ferramenta: string;
  argumentos: Record<string, unknown>;
}

export interface DadosSessaoRetomada {
  sessao_id: string;
  mensagens: number;
}

export const EVENTOS_CONHECIDOS = [
  "resposta_final",
  "erro",
  "audio_pronto",
  "ferramenta_iniciada",
  "ferramenta_concluida",
  "aprovacao_requerida",
  "sessao_retomada",
] as const;

export type TipoEvento = (typeof EVENTOS_CONHECIDOS)[number];

export function ehEventoConhecido(tipo: string): tipo is TipoEvento {
  return (EVENTOS_CONHECIDOS as readonly string[]).includes(tipo);
}
