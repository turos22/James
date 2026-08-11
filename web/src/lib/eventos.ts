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
  // Obrigatório, não opcional: o backend (internal/api/eventos.go,
  // payloadFerramentaConcluida) sempre serializa o campo — nunca omite a
  // chave — e usa string vazia pra "esta ferramenta não produziu saída",
  // exatamente como já faz com `erro`. Opcional sugeriria que a chave pode
  // faltar no JSON, o que nunca acontece e forçaria todo consumidor a um
  // `?? ""` desnecessário.
  saida: string;
}

export interface DadosAprovacaoRequerida {
  // O id vem duplicado dentro de `dados` de propósito: o cliente WS só
  // repassa `dados` aos manipuladores, então o id do envelope não chega
  // sozinho ao handler. Ver o comentário em internal/api/ws.go.
  id: string;
  ferramenta: string;
  argumentos: Record<string, unknown>;
}

// DadosSessaoRetomada carregava só a CONTAGEM de mensagens (`mensagens:
// number`). Achado da verificação ao vivo da Fase 6: o Gemini lembrava do
// histórico direitinho (reconstruído do Postgres pelo servidor), mas a tela
// ficava em branco — nada além do número chegava ao front. Agora o próprio
// servidor manda o conteúdo, no formato que a interface exibe (ver
// internal/api/eventos.go, payloadSessaoRetomada e mensagemDoHistorico): a
// mesma reconstrução que alimenta o contexto do modelo alimenta a tela, sem
// segunda consulta ao backend.
export interface DadosSessaoRetomada {
  sessao_id: string;
  mensagens: { papel: string; texto: string }[];
}

// DadosTranscricao é o texto que o device entendeu numa entrada falada. Só o
// caminho de voz emite (internal/api/dispositivo.go, tratarEntradaDeVoz): no
// caminho de texto o próprio cliente que digitou já desenhou o balão.
export interface DadosTranscricao {
  texto: string;
}

export const EVENTOS_CONHECIDOS = [
  "resposta_final",
  "erro",
  "audio_pronto",
  "ferramenta_iniciada",
  "ferramenta_concluida",
  "aprovacao_requerida",
  "sessao_retomada",
  "transcricao",
] as const;

export type TipoEvento = (typeof EVENTOS_CONHECIDOS)[number];

export function ehEventoConhecido(tipo: string): tipo is TipoEvento {
  return (EVENTOS_CONHECIDOS as readonly string[]).includes(tipo);
}
