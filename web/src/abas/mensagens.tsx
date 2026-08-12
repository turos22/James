import { useEffect, useRef, useState } from "react";
import { Plus, Send, Sparkle } from "lucide-react";
import type { EstadoTurno } from "../estado/turno";
import type { EstadoConexao } from "../lib/ws";
import { separarSegmentos } from "../lib/markdown";

interface Props {
  estado: EstadoTurno;
  conexao: EstadoConexao;
  tentativa: number;
  aoEnviar: (texto: string) => void;
  aoNovaSessao: () => void;
}

// "fechado" é encerramento deliberado (ClienteWS.fechar()), não queda de
// rede: não existe timer de reconexão rodando por trás. Dizer "Reconectando"
// ali prometeria uma tentativa automática que não vai acontecer.
function textoConexao(conexao: EstadoConexao, tentativa: number): string {
  if (conexao === "conectado") return "online";
  if (conexao === "fechado") return "desconectado";
  if (conexao === "conectando") return "conectando…";
  const sufixo = tentativa > 1 ? ` (tentativa ${tentativa})` : "";
  return `Reconectando…${sufixo}`;
}

function Corpo({ texto }: { texto: string }) {
  return (
    <>
      {separarSegmentos(texto).map((segmento, i) =>
        segmento.tipo === "texto" ? (
          <span key={i}>{segmento.conteudo}</span>
        ) : (
          <pre key={i} className="my-1 overflow-x-auto rounded bg-black/40 p-2 text-xs">
            <code className={segmento.linguagem}>{segmento.conteudo}</code>
          </pre>
        ),
      )}
    </>
  );
}

export function AbaMensagens({ estado, conexao, tentativa, aoEnviar, aoNovaSessao }: Props) {
  const [rascunho, setRascunho] = useState("");
  const listaRef = useRef<HTMLDivElement>(null);
  // Auto-scroll condicional: rolar sempre arrancaria o usuário do meio da
  // conversa toda vez que uma resposta chegasse. Só acompanha quem já estava
  // no fim.
  const grudadoNoFim = useRef(true);
  const bloqueado = conexao !== "conectado";

  useEffect(() => {
    const lista = listaRef.current;
    if (!lista || !grudadoNoFim.current) return;
    lista.scrollTop = lista.scrollHeight;
  }, [estado.mensagens]);

  const aoRolar = () => {
    const lista = listaRef.current;
    if (!lista) return;
    const distanciaDoFim = lista.scrollHeight - lista.scrollTop - lista.clientHeight;
    grudadoNoFim.current = distanciaDoFim < 40;
  };

  const enviar = () => {
    const texto = rascunho.trim();
    if (!texto) return;
    aoEnviar(texto);
    setRascunho("");
    grudadoNoFim.current = true;
  };

  return (
    <div className="flex h-full flex-col">
      <header className="flex items-center gap-3 border-b border-app-line px-6 py-4">
        <span className="grid size-9 shrink-0 place-items-center rounded-full bg-app-blue text-app-blue-foreground">
          <Sparkle className="size-4" />
        </span>
        <div className="min-w-0 flex-1">
          <p className="font-serif text-app-text">James</p>
          <p className={`text-xs ${conexao === "conectado" ? "text-emerald-400" : "text-amber-400"}`}>
            {textoConexao(conexao, tentativa)}
          </p>
        </div>
        <button
          type="button"
          onClick={aoNovaSessao}
          aria-label="Nova sessão"
          className="flex items-center gap-1.5 rounded-lg bg-white/5 px-3 py-1.5 text-xs text-app-text transition-colors hover:bg-app-hover"
        >
          <Plus className="size-3.5" />
          Nova sessão
        </button>
      </header>

      <div ref={listaRef} onScroll={aoRolar} className="flex-1 space-y-4 overflow-y-auto px-6 py-6">
        {estado.mensagens.map((m, i) => (
          <div key={i} className={`flex ${m.papel === "usuario" ? "justify-end" : "justify-start"}`}>
            <div
              className={`max-w-[75%] rounded-2xl px-4 py-2.5 text-left text-sm leading-relaxed ${
                m.papel === "usuario"
                  ? "rounded-br-sm bg-app-blue text-app-blue-foreground"
                  : "rounded-bl-sm bg-white/5 text-app-text"
              }`}
            >
              <Corpo texto={m.texto} />
            </div>
          </div>
        ))}
      </div>

      <div className="border-t border-app-line p-4">
        <div className="flex items-center gap-2 rounded-xl bg-white/5 px-3 py-2">
          <input
            value={rascunho}
            disabled={bloqueado}
            onChange={(e) => setRascunho(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                enviar();
              }
            }}
            placeholder="Escreva para o James…"
            className="flex-1 bg-transparent text-sm text-app-text placeholder:text-app-muted focus:outline-none disabled:opacity-40"
            autoFocus
          />
          <button
            type="button"
            onClick={enviar}
            disabled={bloqueado}
            aria-label="Enviar mensagem"
            className="grid size-8 place-items-center rounded-lg bg-app-blue text-app-blue-foreground transition-transform hover:scale-105 active:scale-95 disabled:opacity-40"
          >
            <Send className="size-4" />
          </button>
        </div>
      </div>
    </div>
  );
}
