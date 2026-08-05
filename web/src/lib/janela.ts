// Controle da janela nativa do shell (resize e arraste), via o servidor
// de controle local que shell/controle.go expõe em loopback — nunca
// window.runtime do Wails, que não é injetado na página que o jamesd
// serve (ver o comentário no topo de shell/controle.go pro porquê).
// Fora do shell (navegador, celular), a porta não existe: todo fetch
// falha e vira no-op silencioso — mesmo comportamento de antes.
const ENDERECO_CONTROLE_SHELL = "http://127.0.0.1:17313";

// Usa /redimensionar (não um /tamanho ingênuo): o shell ancora o resize
// no canto onde a bolinha já está, em vez de crescer a partir do canto
// superior-esquerdo — senão, com a bolinha perto da borda da tela (ex.:
// perto da barra de tarefas), o chat abriria pra fora da área visível.
export async function redimensionarJanela(largura: number, altura: number): Promise<void> {
  try {
    await fetch(`${ENDERECO_CONTROLE_SHELL}/redimensionar?largura=${largura}&altura=${altura}`, { method: "POST" });
  } catch {
    // sem shell rodando — ignora
  }
}

export async function posicaoJanela(): Promise<{ x: number; y: number } | null> {
  try {
    const resp = await fetch(`${ENDERECO_CONTROLE_SHELL}/posicao`);
    if (!resp.ok) return null;
    return await resp.json();
  } catch {
    return null;
  }
}

export function moverJanela(x: number, y: number): void {
  fetch(`${ENDERECO_CONTROLE_SHELL}/posicao?x=${Math.round(x)}&y=${Math.round(y)}`, { method: "POST" }).catch(() => {});
}
