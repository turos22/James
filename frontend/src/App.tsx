import { useState, useEffect } from "react";
import "./App.css";
import { AlternarJanela, PerguntaAi } from "../wailsjs/go/main/App";
// Importe a função de escuta de eventos do runtime do Wails
import { EventsOn, EventsOff } from "../wailsjs/runtime/runtime";
import qfLogo from "./assets/images/QF.png";
import { separarSegmentos } from "./lib/markdown";

interface Message {
  role: "user" | "Athena";
  content: string;
}

function App() {
  const [isOpen, setIsOpen] = useState(false);
  const [inputValue, setInputValue] = useState("");
  const [messages, setMessages] = useState<Message[]>([
    { role: "Athena", content: "Sistemas online. Como posso ajudar, Arthur?" }
  ]);
  const [isTyping, setIsTyping] = useState(false);

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

  const processarMensagem = async (texto: string) => {
    if (texto.trim() === "") return;

    setMessages(prev => [...prev, { role: "user", content: texto }]);
    setIsTyping(true);

    const precisaVerTela = texto.toLowerCase().includes("tela");

    const resposta = await PerguntaAi(texto, precisaVerTela);

    setMessages(prev => [...prev, { role: "Athena", content: resposta }]);
    setIsTyping(false);
  };

  useEffect(() => {
    EventsOn("voz_transcricao", (textoTranscrevido: string) => {
      processarMensagem(textoTranscrevido);
    });

    return () => {
      EventsOff("voz_transcricao");
    };
  }, []);

  const handleSendMessage = async (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter" && inputValue.trim() !== "") {
      const textoDigitado = inputValue;
      setInputValue("");
      await processarMensagem(textoDigitado);
    }
  };

  const handleToggleChat = async () => {
    const novoEstado = !isOpen;
    setIsOpen(novoEstado);
    AlternarJanela(novoEstado);
  };

  return (
    <div className="container">
      {/* Floating action button (logo) - always visible */}
      <div className="drag-handle" data-tauri-drag-region onClick={handleToggleChat}>
        <div className="sphere-visual">
          <img src={qfLogo} className="img_qf" alt="Athena" />
        </div>
      </div>

      {/* Chat window - only visible when open */}
      {isOpen && (
        <div className="chat-window">
          <div className="messages-container">
            {messages.map((msg, i) => (
              <div key={i} className={msg.role === 'user' ? 'msg-user' : 'msg-jarvis'}>
                 <small style={{ fontSize: '10px', opacity: 0.5, marginBottom: '2px' }}>
                  {msg.role === 'user' ? 'Arthur' : 'Athena'}
                </small>
                {renderMessageContent(msg.content)}
              </div>
            ))}
            {isTyping && (
                <div className="msg-jarvis typing-dots">
                  <span></span><span></span><span></span>
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