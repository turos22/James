import { useEffect, useRef, useState } from "react";
import "./App.css";
import qfLogo from "./assets/images/QF.png";
import { CardAprovacao } from "./components/CardAprovacao";
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

// window.runtime é injetado pelo Wails só quando a página roda dentro do
// shell desktop — no navegador do celular ele não existe, e a chamada vira
// no-op. Substitui os bindings gerados de wailsjs/runtime (spec 4.9).
function alternarJanelaNativa(expandir: boolean) {
  const runtime = (window as any).runtime;
  if (runtime?.WindowSetSize) {
    runtime.WindowSetSize(expandir ? 800 : 100, expandir ? 900 : 100);
  }
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
    const novoEstado = !isOpen;
    setIsOpen(novoEstado);
    alternarJanelaNativa(novoEstado);
  };

  const decidirAprovacao = (aprovado: boolean) => {
    if (!pedidoAprovacao || !clienteRef.current) return;
    clienteRef.current.enviar(aprovado ? "aprovar" : "negar", pedidoAprovacao.id, {});
    setPedidoAprovacao(null);
  };

  return (
    <div className="container">
      <div className="drag-handle" data-tauri-drag-region onClick={handleToggleChat}>
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
