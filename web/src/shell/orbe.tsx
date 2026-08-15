import { useEffect, useRef } from "react";

const TAMANHO = 320;
const RAIO_REPOUSO = 96;
const GANHO_RAIO = 34;
const PONTOS_BORDA = 96;

// energiaDaFaixa devolve 0..1 para uma fatia do espectro. inicio e fim são
// frações (0 = primeiro bin, 1 = último), o que deixa a chamada legível:
// graves são energiaDaFaixa(dados, 0, 0.25).
export function energiaDaFaixa(dados: Uint8Array, inicio: number, fim: number): number {
  const de = Math.floor(inicio * dados.length);
  const ate = Math.max(de + 1, Math.floor(fim * dados.length));
  let soma = 0;
  for (let i = de; i < ate; i++) soma += dados[i];
  return soma / (ate - de) / 255;
}

// raioDoOrbe traduz energia em raio. Arredonda para o teste poder afirmar
// valor exato — meio pixel não muda nada na tela.
export function raioDoOrbe(energia: number): number {
  return Math.round(RAIO_REPOUSO + energia * GANHO_RAIO);
}

interface Props {
  // Null = sem áudio tocando: o orbe respira devagar em vez de ficar parado.
  analisador: AnalyserNode | null;
}

// Marca do James, colorida — enviada pelo Arthur em
// C:\Users\Arthur\Downloads\logo_james.png. Os cinco path abaixo não foram
// desenhados à mão: vieram de contornar cada região de cor do PNG por
// análise de imagem (OpenCV: máscara por cor → connectedComponents →
// findContours → approxPolyDP), pra não arriscar inventar uma forma
// parecida. viewBox bate com o tamanho do PNG original (441×566) de
// propósito — os pontos já estão nesse espaço, sem precisar reescalar.
// Fixa por cima do canvas, não desenhada nele: não precisa reagir ao áudio,
// e redesenhar por cima do canvas a cada quadro custaria CPU à toa.
function LogoJames() {
  return (
    <svg
      data-testid="logo-james"
      aria-hidden="true"
      viewBox="0 0 441 566"
      className="pointer-events-none absolute left-1/2 top-1/2 h-14 w-11 -translate-x-1/2 -translate-y-1/2 opacity-90"
    >
      <path fill="#F0B60A" d="M156,42 L156,84 L159,87 L192,87 L193,72 L196,69 L370,68 L370,38 L158,37 Z" />
      <path fill="#4283EB" d="M337,73 L337,277 L369,277 L371,275 L370,73 Z" />
      <path
        fill="#379E4F"
        d="M370,280 L337,280 L337,346 L336,347 L336,359 L335,360 L335,365 L334,366 L334,373 L331,379 L331,383 L329,386 L328,391 L324,397 L323,402 L321,403 L321,405 L318,408 L318,410 L315,413 L309,422 L304,426 L304,427 L297,434 L296,436 L295,436 L292,439 L291,439 L290,441 L284,445 L284,449 L304,469 L304,470 L308,473 L309,473 L315,467 L316,467 L322,461 L322,460 L325,458 L325,457 L333,449 L333,448 L340,440 L341,437 L347,430 L347,428 L350,425 L354,416 L356,414 L356,412 L359,407 L359,405 L361,402 L361,400 L364,393 L364,390 L365,389 L366,383 L367,382 L367,377 L368,376 L369,365 L370,364 L370,351 L371,350 L371,281 Z"
      />
      <path
        fill="#E23F2E"
        d="M52,464 L55,469 L58,470 L64,475 L68,476 L72,480 L85,488 L104,497 L112,502 L115,502 L124,507 L132,509 L134,511 L137,511 L140,513 L143,513 L154,517 L163,518 L169,520 L174,520 L175,521 L185,521 L186,522 L219,521 L247,514 L252,511 L261,508 L269,503 L271,503 L283,495 L287,491 L289,491 L305,477 L305,475 L280,450 L260,466 L244,474 L242,476 L234,478 L228,481 L224,481 L220,483 L214,483 L208,485 L188,485 L187,484 L172,483 L141,474 L134,470 L125,467 L101,454 L83,442 L81,442 L74,436 L72,436 L69,439 L58,455 L57,458 Z"
      />
      <path
        fill="#D0D1D0"
        d="M298,109 L155,109 L155,141 L198,141 L201,144 L200,346 L192,369 L171,390 L148,398 L129,397 L92,382 L80,412 L119,428 L160,429 L194,414 L218,389 L233,350 L233,143 L239,141 L265,142 L266,333 L263,348 L250,377 L231,401 L202,430 L178,445 L207,443 L239,430 L257,417 L277,395 L292,366 L298,339 Z"
      />
    </svg>
  );
}

export function Orbe({ analisador }: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const analisadorRef = useRef<AnalyserNode | null>(analisador);

  // O analisador troca a cada áudio novo. Guardar em ref (e não fechar sobre
  // a prop) evita cancelar e recriar o requestAnimationFrame a cada resposta
  // do James — o laço de desenho nasce uma vez e vive enquanto a aba viver.
  useEffect(() => {
    analisadorRef.current = analisador;
  }, [analisador]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    // jsdom não implementa canvas 2d. Sair aqui mantém o componente
    // renderizável em teste sem a lib `canvas` instalada.
    if (!ctx) return;

    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    canvas.width = TAMANHO * dpr;
    canvas.height = TAMANHO * dpr;
    ctx.scale(dpr, dpr);

    const centro = TAMANHO / 2;
    let fase = 0;
    let quadro = 0;

    const desenhar = () => {
      fase += 0.03;

      const analisadorAtual = analisadorRef.current;
      // A ANOTAÇÃO precisa ser Uint8Array<ArrayBuffer>, não Uint8Array puro:
      // TS 5.9 com o lib.dom.d.ts atual tipa
      // AnalyserNode.getByteFrequencyData exigindo esse genérico exato, e o
      // ALIAS "Uint8Array" sem parâmetro widena para Uint8Array<ArrayBufferLike>
      // (supertipo, não atribuível) — mesmo que o valor atribuído já seja
      // Uint8Array<ArrayBuffer>. `new Uint8Array(n)` sozinho já devolve
      // Uint8Array<ArrayBuffer> (ver a sobrecarga `new (length: number)` em
      // lib.es5.d.ts); não precisa de `new ArrayBuffer(n)` por baixo —
      // testado isoladamente antes de escrever este comentário. Achado
      // rodando o build de verdade: o texto anterior deste plano não
      // compilava.
      let dados: Uint8Array<ArrayBuffer> | null = null;
      if (analisadorAtual) {
        dados = new Uint8Array(analisadorAtual.frequencyBinCount);
        analisadorAtual.getByteFrequencyData(dados);
      }

      // Graves controlam o tamanho; agudos, a ondulação fina (§5.1). Sem
      // áudio, respiração senoidal lenta.
      const graves = dados ? energiaDaFaixa(dados, 0, 0.25) : 0.12 + (Math.sin(fase) * 0.5 + 0.5) * 0.1;
      const agudos = dados ? energiaDaFaixa(dados, 0.5, 1) : 0;
      const raioBase = raioDoOrbe(graves);

      ctx.clearRect(0, 0, TAMANHO, TAMANHO);
      ctx.beginPath();
      for (let i = 0; i <= PONTOS_BORDA; i++) {
        const angulo = (i / PONTOS_BORDA) * Math.PI * 2;
        const ondulacao =
          Math.sin(angulo * 3 + fase * 1.6) * (4 + graves * 14) +
          Math.sin(angulo * 6 - fase * 1.1) * (3 + agudos * 10) +
          Math.cos(angulo * 2 + fase) * 3;
        const raio = raioBase + ondulacao;
        const x = centro + Math.cos(angulo) * raio;
        const y = centro + Math.sin(angulo) * raio;
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      }
      ctx.closePath();

      ctx.save();
      ctx.shadowColor = "rgba(0,0,0,0.85)";
      ctx.shadowBlur = 50 + graves * 60;
      const gradiente = ctx.createRadialGradient(centro, centro - 20, 20, centro, centro, raioBase + 20);
      gradiente.addColorStop(0, "#1a1a1f");
      gradiente.addColorStop(0.7, "#0a0a0c");
      gradiente.addColorStop(1, "#000000");
      ctx.fillStyle = gradiente;
      ctx.fill();
      ctx.restore();

      ctx.save();
      ctx.lineWidth = 2;
      ctx.strokeStyle = `rgba(120,140,255,${0.15 + graves * 0.4})`;
      ctx.shadowColor = "rgba(90,110,255,0.6)";
      ctx.shadowBlur = 18 + graves * 30;
      ctx.stroke();
      ctx.restore();

      quadro = requestAnimationFrame(desenhar);
    };

    desenhar();
    return () => cancelAnimationFrame(quadro);
  }, []);

  return (
    <div className="relative inline-block max-w-full shrink-0">
      <canvas
        ref={canvasRef}
        style={{ width: TAMANHO, height: TAMANHO }}
        className="block max-w-full"
        aria-label="Visualização de áudio do James"
      />
      <LogoJames />
    </div>
  );
}
