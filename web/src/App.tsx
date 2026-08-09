import { useEffect, useRef, useState } from "react";
import "./App.css";
import { CardAprovacao } from "./components/CardAprovacao";
import { separarSegmentos } from "./lib/markdown";
import { ClienteWS } from "./lib/ws";
import { Orbe } from "./shell/orbe";

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

  const decidirAprovacao = (aprovado: boolean) => {
    if (!pedidoAprovacao || !clienteRef.current) return;
    clienteRef.current.enviar(aprovado ? "aprovar" : "negar", pedidoAprovacao.id, {});
    setPedidoAprovacao(null);
  };

  return (
    <div className="container">
      <Orbe aberto={isOpen} aoAlternar={setIsOpen} />

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
