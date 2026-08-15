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

// calcularNovaPosicaoArrasto é a matemática pura do arraste: soma o
// deslocamento do mouse (em coordenadas de TELA, não da página — é a
// janela que se move, então clientX/Y não servem de referência) ao canto
// onde a janela estava quando o botão desceu.
export function calcularNovaPosicaoArrasto(
  origemJanela: { x: number; y: number },
  origemMouse: { x: number; y: number },
  mouseAtual: { x: number; y: number },
): { x: number; y: number } {
  return {
    x: origemJanela.x + (mouseAtual.x - origemMouse.x),
    y: origemJanela.y + (mouseAtual.y - origemMouse.y),
  };
}

// arrastarJanela inicia o arraste a partir de um mousedown na área de
// chrome (ver barraLateral.tsx). O Wails 2.11 instalado aqui não expõe
// WindowStartDrag (checado no pkg/runtime da versão vendorizada) — por
// isso o arraste é feito na mão, reaproveitando moverJanela/posicaoJanela
// que já existiam pro resize: escuta mousemove enquanto o botão estiver
// apertado e reposiciona a cada frame.
export function arrastarJanela(evento: Pick<MouseEvent, "screenX" | "screenY">): void {
  const origemMouse = { x: evento.screenX, y: evento.screenY };
  let origemJanela: { x: number; y: number } | null = null;
  let quadroPendente = 0;
  let posicaoPendente: { x: number; y: number } | null = null;

  posicaoJanela().then((pos) => {
    origemJanela = pos;
  });

  const aoMover = (e: MouseEvent) => {
    if (!origemJanela) return;
    posicaoPendente = calcularNovaPosicaoArrasto(origemJanela, origemMouse, { x: e.screenX, y: e.screenY });
    // rAF em vez de mandar fetch a cada mousemove bruto: o evento dispara
    // muito mais rápido do que a rede/o SO conseguem processar reposição,
    // e enfileirar um fetch por evento faz a janela "arrastar atrasado".
    if (!quadroPendente) {
      quadroPendente = requestAnimationFrame(() => {
        quadroPendente = 0;
        if (posicaoPendente) moverJanela(posicaoPendente.x, posicaoPendente.y);
      });
    }
  };

  const aoSoltar = () => {
    window.removeEventListener("mousemove", aoMover);
    window.removeEventListener("mouseup", aoSoltar);
    if (quadroPendente) cancelAnimationFrame(quadroPendente);
  };

  window.addEventListener("mousemove", aoMover);
  window.addEventListener("mouseup", aoSoltar);
}
