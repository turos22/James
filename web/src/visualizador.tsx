import type { EstadoTurno } from "./estado/turno";
import { Tela } from "./abas/tela";

export type ConteudoVisualizador = "aprovacao" | "tela" | "ferramenta" | "vazio";

interface Props {
  estado: EstadoTurno;
  saidaVerTela: string | null;
  aoDecidir: (aprovado: boolean) => void;
}

// Precedência: aprovação pendente vence tudo, porque é a única coisa que
// bloqueia o turno esperando o humano. Depois vale a recência — a captura
// de tela só aparece enquanto ver_tela for a ferramenta mais recente. Sem
// isso, saidaVerTela (que guarda a última captura da sessão inteira, não
// do turno) congelaria o painel na primeira captura, e nenhuma ferramenta
// posterior apareceria.
export function escolherConteudo(estado: EstadoTurno, saidaVerTela: string | null): ConteudoVisualizador {
  if (estado.aprovacao) return "aprovacao";

  const ultima = estado.ferramentas[estado.ferramentas.length - 1];
  if (ultima?.nome === "ver_tela" && saidaVerTela) return "tela";
  if (ultima) return "ferramenta";
  return "vazio";
}

export function Visualizador({ estado, saidaVerTela, aoDecidir }: Props) {
  const conteudo = escolherConteudo(estado, saidaVerTela);

  if (conteudo === "aprovacao" && estado.aprovacao) {
    return (
      <div className="p-4">
        <p className="mb-2 text-sm">
          O James quer executar <strong>{estado.aprovacao.ferramenta}</strong>
        </p>
        <pre className="mb-3 overflow-x-auto rounded bg-black/40 p-2 text-xs">
          {JSON.stringify(estado.aprovacao.argumentos, null, 2)}
        </pre>
        <div className="flex gap-2">
          <button onClick={() => aoDecidir(true)} className="rounded bg-emerald-600/70 px-3 py-1 text-sm">
            Aprovar
          </button>
          <button onClick={() => aoDecidir(false)} className="rounded bg-red-600/70 px-3 py-1 text-sm">
            Negar
          </button>
        </div>
      </div>
    );
  }

  if (conteudo === "tela") return <Tela saida={saidaVerTela} />;

  if (conteudo === "ferramenta") {
    const ultima = estado.ferramentas[estado.ferramentas.length - 1];
    return (
      <div className="p-4 font-mono text-xs">
        <div className="mb-1 opacity-60">última ferramenta</div>
        <div>{ultima.nome}</div>
        {ultima.erro && <div className="mt-1 text-amber-300">{ultima.erro}</div>}
      </div>
    );
  }

  return <p className="p-4 text-sm opacity-40">Nada para mostrar ainda.</p>;
}
