import { useEffect, useReducer, useRef, useState } from "react";
import { Atividade } from "./abas/atividade";
import { Conversa } from "./abas/conversa";
import { Memoria } from "./abas/memoria";
import { Sessoes } from "./abas/sessoes";
import { Tela } from "./abas/tela";
import { estadoInicial, reduzir } from "./estado/turno";
import { ClienteWS, type EstadoConexao } from "./lib/ws";
import { Orbe } from "./shell/orbe";
import { Trilha, type IdAba } from "./trilha";
import { Visualizador } from "./visualizador";

function urlWS(): string {
  const protocolo = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocolo}//${window.location.host}/ws`;
}

export default function App() {
  const [aberto, setAberto] = useState(false);
  const [aba, setAba] = useState<IdAba>("conversa");
  const [conexao, setConexao] = useState<EstadoConexao>("conectando");
  const [tentativa, setTentativa] = useState(0);
  const [saidaVerTela, setSaidaVerTela] = useState<string | null>(null);
  const [estado, despachar] = useReducer(reduzir, estadoInicial);
  const clienteRef = useRef<ClienteWS | null>(null);

  useEffect(() => {
    const cliente = new ClienteWS(urlWS());
    clienteRef.current = cliente;

    cliente.aoMudarEstado((novoEstado, n) => {
      setConexao(novoEstado);
      setTentativa(n);
    });

    cliente.on("resposta_final", (dados) => despachar({ tipo: "resposta_final", dados }));
    cliente.on("erro", (dados) => despachar({ tipo: "erro", dados }));
    cliente.on("ferramenta_iniciada", (dados) => despachar({ tipo: "ferramenta_iniciada", dados }));
    cliente.on("ferramenta_concluida", (dados) => despachar({ tipo: "ferramenta_concluida", dados }));
    cliente.on("aprovacao_requerida", (dados) => despachar({ tipo: "aprovacao_requerida", dados }));
    cliente.on("sessao_retomada", (dados) => despachar({ tipo: "sessao_retomada", dados }));

    cliente.on("audio_pronto", (dados: { id: string }) => {
      const excluirAudio = () => {
        fetch(`/api/audio/${dados.id}`, { method: "DELETE" }).catch(() => {});
      };
      const audio = new Audio(`/api/audio/${dados.id}`);
      audio.addEventListener("ended", excluirAudio);
      // Autoplay pode ser bloqueado sem gesto recente do usuário — se falhar,
      // o áudio só não toca, mas o arquivo ainda precisa sair do disco.
      audio.play().catch(excluirAudio);
    });

    return () => cliente.fechar();
  }, []);

  const enviar = (texto: string) => {
    despachar({ tipo: "envio_local", texto });
    clienteRef.current?.enviar("mensagem", crypto.randomUUID(), { texto });
  };

  const decidir = (aprovado: boolean) => {
    if (!estado.aprovacao) return;
    clienteRef.current?.enviar(aprovado ? "aprovar" : "negar", estado.aprovacao.id, {});
    despachar({ tipo: "aprovacao_decidida" });
  };

  const retomar = (id: string) => {
    clienteRef.current?.enviar("retomar", crypto.randomUUID(), { sessao_id: id });
    setAba("conversa");
  };

  return (
    <div className="h-screen w-screen text-white">
      <Orbe aberto={aberto} aoAlternar={setAberto} />

      {aberto && (
        <div className="flex h-full flex-col-reverse rounded-xl bg-black/70 backdrop-blur md:flex-row">
          <Trilha ativa={aba} aoTrocar={setAba} />

          <main className="min-w-0 flex-1 p-2">
            {aba === "conversa" && (
              <Conversa estado={estado} conexao={conexao} tentativa={tentativa} aoEnviar={enviar} />
            )}
            {aba === "tela" && <Tela saida={saidaVerTela} />}
            {aba === "atividade" && <Atividade estado={estado} />}
            {aba === "memoria" && <Memoria />}
            {aba === "sessoes" && <Sessoes sessaoAtual={estado.sessaoAtual} aoRetomar={retomar} />}
          </main>

          <aside className="min-w-0 flex-1 border-l border-white/10">
            <Visualizador estado={estado} saidaVerTela={saidaVerTela} aoDecidir={decidir} />
          </aside>
        </div>
      )}
    </div>
  );
}
