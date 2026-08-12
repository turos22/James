// Controle da janela nativa do shell, via o servidor de controle local que
// shell/controle.go expõe em loopback — nunca window.runtime do Wails, que
// não é injetado na página que o jamesd serve (ver o comentário no topo de
// shell/controle.go pro porquê).
// Fora do shell (navegador, celular), a porta não existe: todo fetch falha
// e vira no-op silencioso.
const ENDERECO_CONTROLE_SHELL = "http://127.0.0.1:17313";

// redimensionarJanela saiu na Fase 7A: existia para a bolinha crescer ao
// abrir o chat, e a bolinha morreu. Maximizar passa por alternarMaximizado.

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

// shellPresente sonda o servidor de controle. É como a SPA descobre que
// está rodando dentro do Wails — não por user-agent, que mentiria em
// qualquer WebView.
export async function shellPresente(): Promise<boolean> {
  try {
    const resp = await fetch(`${ENDERECO_CONTROLE_SHELL}/posicao`);
    return resp.ok;
  } catch {
    return false;
  }
}

export function minimizarJanela(): void {
  fetch(`${ENDERECO_CONTROLE_SHELL}/minimizar`, { method: "POST" }).catch(() => {});
}

export function alternarMaximizado(): void {
  fetch(`${ENDERECO_CONTROLE_SHELL}/maximizar`, { method: "POST" }).catch(() => {});
}

export function fecharJanela(): void {
  fetch(`${ENDERECO_CONTROLE_SHELL}/fechar`, { method: "POST" }).catch(() => {});
}
