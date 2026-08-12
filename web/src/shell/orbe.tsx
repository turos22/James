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
      let dados: Uint8Array<ArrayBuffer> | null = null;
      if (analisadorAtual) {
        dados = new Uint8Array(new ArrayBuffer(analisadorAtual.frequencyBinCount));
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
    <canvas
      ref={canvasRef}
      style={{ width: TAMANHO, height: TAMANHO }}
      className="max-w-full shrink-0"
      aria-label="Visualização de áudio do James"
    />
  );
}
