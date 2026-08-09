import { useState } from "react";
import type { EstadoConexao } from "../lib/ws";
import type { EstadoTurno } from "../estado/turno";
import { separarSegmentos } from "../lib/markdown";

interface Props {
  estado: EstadoTurno;
  conexao: EstadoConexao;
  aoEnviar: (texto: string) => void;
  // Vem do segundo argumento de ClienteWS.aoMudarEstado. Sem isso a faixa
  // diz "Reconectando" para sempre, e o usuário não distingue uma queda de
  // um segundo de um backend fora do ar há dez minutos.
  tentativa?: number;
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
        )
      )}
    </>
  );
}

// "fechado" é encerramento deliberado (ClienteWS.fechar()), não queda de
// rede: não existe timer de reconexão rodando por trás. Mostrar "Conectando"
// ou "Reconectando" nesse caso prometeria uma tentativa automática que não
// vai acontecer. Por isso tem texto próprio, sem número de tentativa.
function textoFaixa(conexao: EstadoConexao, tentativa: number | undefined): string {
  if (conexao === "fechado") return "Desconectado.";
  if (conexao === "conectando") return "Conectando...";
  const sufixo = tentativa && tentativa > 1 ? ` (tentativa ${tentativa})` : "";
  return `Reconectando ao James...${sufixo}`;
}

export function Conversa({ estado, conexao, aoEnviar, tentativa }: Props) {
  const [rascunho, setRascunho] = useState("");
  const bloqueado = conexao !== "conectado";

  const enviar = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key !== "Enter" || rascunho.trim() === "") return;
    aoEnviar(rascunho);
    setRascunho("");
  };

  return (
    <div className="flex h-full flex-col gap-2">
      {bloqueado && (
        <div role="status" className="rounded bg-amber-500/20 px-3 py-1 text-xs">
          {textoFaixa(conexao, tentativa)}
        </div>
      )}

      <div className="flex-1 overflow-y-auto pr-1">
        {estado.mensagens.map((m, i) => (
          <div
            key={i}
            className={`mb-2 max-w-[85%] rounded-lg px-3 py-2 text-sm ${
              m.papel === "usuario" ? "ml-auto bg-white/10" : "bg-white/5"
            }`}
          >
            <div className="mb-0.5 text-[10px] opacity-50">{m.papel === "usuario" ? "Arthur" : "James"}</div>
            <Corpo texto={m.texto} />
          </div>
        ))}

        {estado.ferramentas.map((f, i) => (
          <div key={i} className="mb-1 font-mono text-[10px] opacity-60">
            {f.estado === "voando" ? "⚡" : f.estado === "erro" ? "✗" : "✓"} {f.nome}
            {f.erro ? ` — ${f.erro}` : ""}
          </div>
        ))}
      </div>

      <input
        className="w-full rounded border border-white/15 bg-transparent px-3 py-2 text-sm outline-none disabled:opacity-40"
        placeholder="Comando de voz ou texto..."
        value={rascunho}
        disabled={bloqueado}
        onChange={(e) => setRascunho(e.target.value)}
        onKeyDown={enviar}
        autoFocus
      />
    </div>
  );
}
