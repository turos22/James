import { useEffect, useRef, useState } from "react";
import "./App.css";
import qfLogo from "./assets/images/QF.png";
import { CardAprovacao } from "./components/CardAprovacao";
import { moverJanela, posicaoJanela, redimensionarJanela } from "./lib/janela";
import { separarSegmentos } from "./lib/markdown";
import { ClienteWS } from "./lib/ws";

interface Message {
  role: "user" | "James";
  content: string;
}

interface PedidoAprovacao {
  id: string;
  ferramenta: string;
  argumentos: Record<string, unknown>;
}

function urlWS(): string {
  const protocolo = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocolo}//${window.location.host}/ws`;
}

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

function App() {
  const [isOpen, setIsOpen] = useState(false);
  const [inputValue, setInputValue] = useState("");
  const [messages, setMessages] = useState<Message[]>([
    { role: "James", content: "Sistemas online. Como posso ajudar, Arthur?" },
  ]);
  const [isTyping, setIsTyping] = useState(false);
  const [pedidoAprovacao, setPedidoAprovacao] = useState<PedidoAprovacao | null>(null);
  const clienteRef = useRef<ClienteWS | null>(null);
  const { aoIniciarArraste, houveArraste } = useArrastarJanela();

  useEffect(() => {
    const cliente = new ClienteWS(urlWS());
    clienteRef.current = cliente;

    cliente.on("resposta_final", (dados: { texto: string }) => {
      setMessages((prev) => [...prev, { role: "James", content: dados.texto }]);
      setIsTyping(false);
    });

    cliente.on("aprovacao_requerida", (dados: any) => {
      setPedidoAprovacao({ id: dados.id ?? "", ferramenta: dados.ferramenta, argumentos: dados.argumentos ?? {} });
    });

    cliente.on("erro", (dados: { mensagem: string }) => {
      setMessages((prev) => [...prev, { role: "James", content: `Erro: ${dados.mensagem}` }]);
      setIsTyping(false);
    });

    cliente.on("audio_pronto", (dados: { id: string }) => {
      const excluirAudio = () => {
        fetch(`/api/audio/${dados.id}`, { method: "DELETE" }).catch(() => {});
      };
      const audio = new Audio(`/api/audio/${dados.id}`);
      audio.addEventListener("ended", excluirAudio);
      // Autoplay pode ser bloqueado pelo navegador sem gesto do usuário
      // recente — se falhar, o áudio só não toca (chat continua normal
      // por texto), mas o arquivo ainda precisa ser limpo do disco, já
      // que "ended" nunca vai disparar nesse caso.
      audio.play().catch(excluirAudio);
    });

    return () => cliente.fechar();
  }, []);

  const renderMessageContent = (content: string) =>
    separarSegmentos(content).map((segmento, indice) =>
      segmento.tipo === "texto" ? (
        <span key={indice}>{segmento.conteudo}</span>
      ) : (
        <pre className="code-block" key={indice}>
          <code className={segmento.linguagem}>{segmento.conteudo}</code>
        </pre>
      )
    );

  const processarMensagem = (texto: string) => {
    if (texto.trim() === "" || !clienteRef.current) return;
    setMessages((prev) => [...prev, { role: "user", content: texto }]);
    setIsTyping(true);
    clienteRef.current.enviar("mensagem", crypto.randomUUID(), { texto });
  };

  const handleSendMessage = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter" && inputValue.trim() !== "") {
      const textoDigitado = inputValue;
      setInputValue("");
      processarMensagem(textoDigitado);
    }
  };

  const handleToggleChat = () => {
    // Se acabou de arrastar a janela, esse clique é o mouseup do arraste,
    // não uma intenção de abrir/fechar o chat — ignora.
    if (houveArraste.current) return;
    const novoEstado = !isOpen;
    setIsOpen(novoEstado);
    redimensionarJanela(novoEstado ? 800 : 100, novoEstado ? 900 : 100);
  };

  const decidirAprovacao = (aprovado: boolean) => {
    if (!pedidoAprovacao || !clienteRef.current) return;
    clienteRef.current.enviar(aprovado ? "aprovar" : "negar", pedidoAprovacao.id, {});
    setPedidoAprovacao(null);
  };

  return (
    <div className="container">
      <div className="drag-handle" onMouseDown={aoIniciarArraste} onClick={handleToggleChat}>
        <div className="sphere-visual">
          <img src={qfLogo} className="img_qf" alt="James" />
        </div>
      </div>

      {isOpen && (
        <div className="chat-window">
          {pedidoAprovacao && (
            <CardAprovacao
              ferramenta={pedidoAprovacao.ferramenta}
              argumentos={pedidoAprovacao.argumentos}
              onDecidir={decidirAprovacao}
            />
          )}
          <div className="messages-container">
            {messages.map((msg, i) => (
              <div key={i} className={msg.role === "user" ? "msg-user" : "msg-jarvis"}>
                <small style={{ fontSize: "10px", opacity: 0.5, marginBottom: "2px" }}>
                  {msg.role === "user" ? "Arthur" : "James"}
                </small>
                {renderMessageContent(msg.content)}
              </div>
            ))}
            {isTyping && (
              <div className="msg-jarvis typing-dots">
                <span></span>
                <span></span>
                <span></span>
              </div>
            )}
          </div>

          <input
            className="input-area"
            placeholder="Comando de voz ou texto..."
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            onKeyDown={handleSendMessage}
            autoFocus
          />
        </div>
      )}
    </div>
  );
}

export default App;
