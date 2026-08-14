import { useEffect, useReducer, useRef, useState } from "react";
import { AbaDashboard } from "./abas/dashboard";
import { AbaHistorico } from "./abas/historico";
import { AbaJames } from "./abas/james";
import { AbaMensagens } from "./abas/mensagens";
import { Memoria } from "./abas/memoria";
import { estadoInicial, reduzir } from "./estado/turno";
import { criarAudioAnalisado, despertarAudio } from "./lib/audio";
import { ClienteWS, type EstadoConexao } from "./lib/ws";
import { Aplicativo } from "./shell/aplicativo";

function urlWS(): string {
  const protocolo = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocolo}//${window.location.host}/ws`;
}

export default function App() {
  const [conexao, setConexao] = useState<EstadoConexao>("conectando");
  const [tentativa, setTentativa] = useState(0);
  const [saidaVerTela, setSaidaVerTela] = useState<string | null>(null);
  const [analisador, setAnalisador] = useState<AnalyserNode | null>(null);
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
      // frente da mensagem que esperou a queda. É a ordem que importa.
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

      let audio: HTMLAudioElement;
      let analisadorDoAudio: AnalyserNode | null = null;
      try {
        const analisado = criarAudioAnalisado(`/api/audio/${dados.id}`);
        audio = analisado.audio;
        analisadorDoAudio = analisado.analisador;
      } catch (erro) {
        // Web Audio indisponível (WebView antigo, política restritiva): o
        // áudio ainda deve tocar, só o orbe fica em repouso. console.warn
        // (não silencioso) segue o padrão de ws.ts: um catch mudo aqui
        // esconderia um bug de programação real atrás do mesmo rótulo de
        // "navegador não suporta".
        console.warn("Web Audio indisponível, orbe fica em repouso:", erro);
        audio = new Audio(`/api/audio/${dados.id}`);
      }

      setAnalisador(analisadorDoAudio);
      const encerrar = () => {
        setAnalisador(null);
        excluirAudio();
      };
      audio.addEventListener("ended", encerrar);
      // Autoplay pode ser bloqueado sem gesto recente do usuário — se falhar,
      // o áudio só não toca, mas o arquivo ainda precisa sair do disco.
      audio.play().catch(encerrar);
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

  // Provisório: a Task 14 liga isto ao sessionStorage e ao comando
  // nova_sessao. Deixar sem efeito aqui manteria o botão mentindo.
  const novaSessao = () => {};

  const retomar = (id: string) => {
    clienteRef.current?.enviar("retomar", crypto.randomUUID(), { sessao_id: id });
  };

  return (
    <div className="contents" onClick={despertarAudio}>
      <Aplicativo
        aprovacoesPendentes={estado.aprovacao ? 1 : 0}
        paineis={{
          james: <AbaJames estado={estado} saidaVerTela={saidaVerTela} aoDecidir={decidir} analisador={analisador} />,
          mensagens: (
            <AbaMensagens
              estado={estado}
              conexao={conexao}
              tentativa={tentativa}
              aoEnviar={enviar}
              aoNovaSessao={novaSessao}
            />
          ),
          memoria: <Memoria />,
          historico: <AbaHistorico sessaoAtual={estado.sessaoAtual} aoRetomar={retomar} />,
          dashboard: <AbaDashboard />,
        }}
      />
    </div>
  );
}
