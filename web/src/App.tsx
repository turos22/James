import { useEffect, useReducer, useRef, useState } from "react";
import { AbaDashboard } from "./abas/dashboard";
import { AbaHistorico } from "./abas/historico";
import { AbaJames } from "./abas/james";
import { AbaMensagens } from "./abas/mensagens";
import { Memoria } from "./abas/memoria";
import { estadoInicial, reduzir } from "./estado/turno";
import { criarAudioAnalisado, despertarAudio, tocarAudio } from "./lib/audio";
import { guardarSessao, limparSessao, sessaoGuardada } from "./lib/sessao";
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
  const [vozBloqueada, setVozBloqueada] = useState(false);
  const [estado, despachar] = useReducer(reduzir, estadoInicial);
  const clienteRef = useRef<ClienteWS | null>(null);
  // Espelho de estado.sessaoAtual só para leitura dentro do efeito de
  // conexão, que roda uma vez e não enxerga estado novo pelo closure. É
  // derivado, não uma segunda fonte de verdade: quem decide qual é a sessão
  // continua sendo o redutor.
  const sessaoAtualRef = useRef<string | null>(sessaoGuardada());

  // Só espelha/persiste quando há sessão de verdade. No mount,
  // estado.sessaoAtual começa null (estadoInicial) — sem esta guarda, esse
  // null sobrescreveria (antes mesmo da conexão WS abrir) o valor que
  // sessaoAtualRef acabou de carregar do sessionStorage num F5, e a sessão
  // persistida nunca seria retomada. novaSessao() já limpa o ref e o
  // storage diretamente quando o usuário pede sessão nova — este efeito não
  // precisa (e não deve) fazer isso de novo.
  useEffect(() => {
    if (estado.sessaoAtual) {
      sessaoAtualRef.current = estado.sessaoAtual;
      guardarSessao(estado.sessaoAtual);
    }
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
      //
      // Retoma tanto na reconexão (sessão viva no redutor) quanto na
      // primeira conexão depois de um F5 (sessão veio do sessionStorage).
      // O que NÃO retoma é janela nova: ali o sessionStorage está vazio, e
      // a sessão nasce no servidor no primeiro turno.
      if (sessaoAtualRef.current) {
        cliente.enviar("retomar", crypto.randomUUID(), { sessao_id: sessaoAtualRef.current });
      }
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
      // Autoplay pode ser bloqueado sem gesto recente do usuário (medido no
      // WebKitGTK do shell no Fedora). Se falhar, o arquivo ainda precisa sair
      // do disco, e a tela precisa dizer por que o James ficou mudo.
      void tocarAudio(audio).then((tocou) => {
        if (tocou) return;
        setVozBloqueada(true);
        encerrar();
      });
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

  const novaSessao = () => {
    limparSessao();
    sessaoAtualRef.current = null;
    // Limpar só no cliente deixaria o servidor na conversa velha:
    // sessaoID e historico são variáveis locais do handler em ws.go.
    clienteRef.current?.enviar("nova_sessao", crypto.randomUUID(), {});
  };

  const retomar = (id: string) => {
    clienteRef.current?.enviar("retomar", crypto.randomUUID(), { sessao_id: id });
  };

  return (
    <div
      className="contents"
      onClick={() => {
        despertarAudio();
        setVozBloqueada(false);
      }}
    >
      {vozBloqueada && (
        <div role="alert" className="fixed inset-x-0 top-0 z-50 bg-amber-500 px-4 py-2 text-center text-sm text-black">
          O navegador bloqueou a voz do James. Clique em qualquer lugar da janela para liberar.
        </div>
      )}
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
