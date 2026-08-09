import { useEffect, useRef } from "react";
import qfLogo from "../assets/images/QF.png";
import { moverJanela, posicaoJanela, redimensionarJanela } from "../lib/janela";

// Distância mínima (em px de tela) pra um mousedown+mousemove contar como
// arraste em vez de clique — sem isso, qualquer tremor de mão ao clicar
// na bolinha abriria/fecharia o chat sem querer.
const LIMIAR_ARRASTE_PX = 3;

// Controla o arraste da bolinha flutuante (mover a janela nativa
// inteira, já que a bolinha em si é fixa dentro da janela via CSS) e
// distingue de um clique simples pra abrir/fechar o chat. Sem shell
// rodando (navegador, celular), posicaoJanela() devolve null e o
// arraste nunca começa — só o clique de abrir/fechar continua ativo.
function useArrastarJanela() {
  // mouseApertado é síncrono (true/false exatos entre mousedown e mouseup
  // reais) — arrastando só passa a valer depois que a posição de origem
  // chega (await), o que pode acontecer DEPOIS do mouseup num clique
  // rápido. Sem mouseApertado como guarda, um clique rápido (mousedown +
  // mouseup antes da resposta de posicaoJanela() chegar) deixava
  // arrastando.current preso em true pra sempre — daí qualquer movimento
  // de mouse seguinte (nem perto da bolinha, só indo até o campo de
  // texto) movia a janela sem o usuário estar arrastando nada. Isso
  // desalinhava a posição da janela em relação ao que o resize ancorado
  // espera, e o sintoma era o chat não voltar pro mesmo lugar ao fechar.
  const mouseApertado = useRef(false);
  const arrastando = useRef(false);
  const houveArraste = useRef(false);
  const origemJanela = useRef({ x: 0, y: 0 });
  const origemMouse = useRef({ x: 0, y: 0 });

  useEffect(() => {
    const aoMover = (e: MouseEvent) => {
      if (!arrastando.current) return;
      const dx = e.screenX - origemMouse.current.x;
      const dy = e.screenY - origemMouse.current.y;
      // Só move a janela de verdade depois de passar do limiar — um
      // tremor de 1px no clique não pode deslocar a janela sozinho.
      if (Math.abs(dx) > LIMIAR_ARRASTE_PX || Math.abs(dy) > LIMIAR_ARRASTE_PX) {
        houveArraste.current = true;
        moverJanela(origemJanela.current.x + dx, origemJanela.current.y + dy);
      }
    };
    const aoSoltar = () => {
      mouseApertado.current = false;
      arrastando.current = false;
    };
    window.addEventListener("mousemove", aoMover);
    window.addEventListener("mouseup", aoSoltar);
    return () => {
      window.removeEventListener("mousemove", aoMover);
      window.removeEventListener("mouseup", aoSoltar);
    };
  }, []);

  const aoIniciarArraste = async (e: React.MouseEvent) => {
    houveArraste.current = false;
    mouseApertado.current = true;
    const mouseXInicial = e.screenX;
    const mouseYInicial = e.screenY;
    const posicaoAtual = await posicaoJanela();
    // O botão já pode ter sido solto enquanto esperava a resposta — sem
    // essa checagem, arrastando.current ficaria true com o mouse já
    // solto, e o próximo mousemove (de qualquer clique futuro) moveria
    // a janela sem o usuário estar arrastando nada.
    if (!posicaoAtual || !mouseApertado.current) return;
    origemJanela.current = posicaoAtual;
    origemMouse.current = { x: mouseXInicial, y: mouseYInicial };
    arrastando.current = true;
  };

  return { aoIniciarArraste, houveArraste };
}

export const TAMANHO_RECOLHIDO = 100;
export const LARGURA_EXPANDIDA = 1100;
export const ALTURA_EXPANDIDA = 900;

interface Props {
  aberto: boolean;
  aoAlternar: (aberto: boolean) => void;
}

export function Orbe({ aberto, aoAlternar }: Props) {
  const { aoIniciarArraste, houveArraste } = useArrastarJanela();

  const alternar = () => {
    // Se acabou de arrastar a janela, esse clique é o mouseup do arraste,
    // não uma intenção de abrir/fechar o chat — ignora.
    if (houveArraste.current) return;
    const novoEstado = !aberto;
    aoAlternar(novoEstado);
    redimensionarJanela(
      novoEstado ? LARGURA_EXPANDIDA : TAMANHO_RECOLHIDO,
      novoEstado ? ALTURA_EXPANDIDA : TAMANHO_RECOLHIDO
    );
  };

  return (
    <div className="drag-handle" onMouseDown={aoIniciarArraste} onClick={alternar}>
      <div className="sphere-visual">
        <img src={qfLogo} className="img_qf" alt="James" />
      </div>
    </div>
  );
}
