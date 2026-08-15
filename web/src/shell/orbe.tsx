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

// Marca do James (o "J" do design de referência, ver
// docs/superpowers/referencias/painel-de-controle-james/public/icon.svg) —
// só os traços, sem o retângulo de fundo do ícone original, porque o fundo
// aqui já é o próprio orbe. Fixa por cima do canvas, não desenhada nele: não
// precisa reagir ao áudio, e desenhar por cima do canvas a cada quadro só
// custaria CPU à toa.
function LogoJames() {
  return (
    <svg
      data-testid="logo-james"
      aria-hidden="true"
      viewBox="0 0 180 180"
      fill="currentColor"
      className="pointer-events-none absolute left-1/2 top-1/2 size-14 -translate-x-1/2 -translate-y-1/2 text-white/80"
    >
      <path d="M101.141 53H136.632C151.023 53 162.689 64.6662 162.689 79.0573V112.904H148.112V79.0573C148.112 78.7105 148.098 78.3662 148.072 78.0251L112.581 112.898C112.701 112.902 112.821 112.904 112.941 112.904H148.112V126.672H112.941C98.5504 126.672 86.5638 114.891 86.5638 100.5V66.7434H101.141V100.5C101.141 101.15 101.191 101.792 101.289 102.422L137.56 66.7816C137.255 66.7563 136.945 66.7434 136.632 66.7434H101.141V53Z" />
      <path d="M65.2926 124.136L14 66.7372H34.6355L64.7495 100.436V66.7372H80.1365V118.47C80.1365 126.278 70.4953 129.958 65.2926 124.136Z" />
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
