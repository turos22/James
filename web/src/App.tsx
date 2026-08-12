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
  // Espelho de estado.sessaoAtual só para leitura dentro do efeito de
  // conexão, que roda uma vez e não enxerga estado novo pelo closure. É
  // derivado, não uma segunda fonte de verdade: quem decide qual é a sessão
  // continua sendo o redutor.
  const sessaoAtualRef = useRef<string | null>(null);
  const jaConectouRef = useRef(false);

  useEffect(() => {
    sessaoAtualRef.current = estado.sessaoAtual;
  }, [estado.sessaoAtual]);

  useEffect(() => {
    const cliente = new ClienteWS(urlWS());
    clienteRef.current = cliente;

    cliente.aoMudarEstado((novoEstado, n) => {
      setConexao(novoEstado);
      setTentativa(n);

      if (novoEstado !== "conectado") return;

      // Reconexão abre uma conexão NOVA no jamesd, e sessão e histórico
      // vivem por conexão lá (ws.go: variáveis locais do handler). Sem
      // reemitir "retomar", a tela segue mostrando a conversa antiga
      // enquanto o modelo volta a zero e o turno seguinte grava numa sessão
      // órfã — sem erro nenhum, que é o que torna isso perigoso.
      //
      // Sai daqui e não de um efeito: ClienteWS publica "conectado" dentro
      // do onopen ANTES de descarregar a fila, então este envio entra na
      // frente da mensagem que esperou a queda. É a ordem que importa — a
      // mensagem enfileirada precisa cair na sessão retomada, não criar
      // outra.
      if (jaConectouRef.current && sessaoAtualRef.current) {
        cliente.enviar("retomar", crypto.randomUUID(), { sessao_id: sessaoAtualRef.current });
      }
      jaConectouRef.current = true;
    });

    cliente.on("resposta_final", (dados) => despachar({ tipo: "resposta_final", dados }));
    cliente.on("erro", (dados) => despachar({ tipo: "erro", dados }));
    cliente.on("ferramenta_iniciada", (dados) => despachar({ tipo: "ferramenta_iniciada", dados }));
    cliente.on("ferramenta_concluida", (dados) => {
      despachar({ tipo: "ferramenta_concluida", dados });
      if (dados.nome === "ver_tela" && dados.saida) setSaidaVerTela(dados.saida);
    });
    cliente.on("transcricao", (dados) => despachar({ tipo: "transcricao", dados }));
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
      <Orbe analisador={null} />

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
