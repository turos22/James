// Ponte entre o áudio do TTS e o orbe. Três armadilhas de falha silenciosa
// vivem aqui, e só aqui:
//
// 1. createMediaElementSource só pode ser chamado UMA VEZ por elemento. Por
//    isso o nó nasce junto com o Audio, nesta função, e nunca depois.
// 2. Roteado pelo grafo de áudio, o som só sai pelo destination. Esquecer
//    esse connect deixa o orbe animando em silêncio absoluto — e o sintoma
//    parece "o TTS parou de funcionar".
// 3. AudioContext nasce suspended sem gesto do usuário. Um contexto único
//    no app, com resume() aqui e no primeiro clique (ver despertarAudio).

let contextoUnico: AudioContext | null = null;

export function contextoDeAudio(): AudioContext {
  if (!contextoUnico) {
    const Construtor =
      window.AudioContext ?? (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
    contextoUnico = new Construtor();
  }
  return contextoUnico;
}

// despertarAudio é chamado no primeiro clique em qualquer lugar do app.
// Sem gesto do usuário o navegador mantém o contexto suspenso, e nem o
// áudio toca nem o analisador produz dado.
export function despertarAudio(): void {
  const ctx = contextoDeAudio();
  if (ctx.state === "suspended") void ctx.resume();
}

export interface AudioAnalisado {
  audio: HTMLAudioElement;
  analisador: AnalyserNode;
}

export function criarAudioAnalisado(url: string, ctx: AudioContext = contextoDeAudio()): AudioAnalisado {
  if (ctx.state === "suspended") void ctx.resume();

  const audio = new Audio(url);
  // Same-origin: o jamesd serve /api/audio e a SPA. Sem isso o
  // createMediaElementSource devolveria silêncio por CORS.
  audio.crossOrigin = "anonymous";

  const fonte = ctx.createMediaElementSource(audio);
  const analisador = ctx.createAnalyser();
  analisador.fftSize = 128;

  fonte.connect(analisador);
  analisador.connect(ctx.destination);

  return { audio, analisador };
}
