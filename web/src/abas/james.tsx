import type { EstadoTurno, FerramentaNoTurno } from "../estado/turno";
import { interpretarSaidaVerTela } from "../lib/verTela";
import { Orbe } from "../shell/orbe";

interface Props {
  estado: EstadoTurno;
  saidaVerTela: string | null;
  aoDecidir: (aprovado: boolean) => void;
  // O analisador vem de fora para o componente ser testável sem áudio real
  // (§5.1 da spec). Null = orbe em respiração de repouso.
  analisador: AnalyserNode | null;
}

export type ConteudoPainel = "aprovacao" | "ferramenta" | "tela" | "repouso";

// Precedência: aprovação pendente vence tudo, porque é a única coisa que
// bloqueia o turno esperando o humano. Depois vale a recência — a leitura de
// tela só aparece enquanto ver_tela for a ferramenta mais recente.
export function escolherConteudo(estado: EstadoTurno, saidaVerTela: string | null): ConteudoPainel {
  if (estado.aprovacao) return "aprovacao";

  const ultima = estado.ferramentas[estado.ferramentas.length - 1];
  if (!ultima) return "repouso";
  if (ultima.nome === "ver_tela" && saidaVerTela) return "tela";
  return "ferramenta";
}

// duracaoLegivel formata o tempo desde iniciadaEm. Ferramenta ainda voando
// mostra o tempo corrido até agora; concluída mostra o total.
function duracaoLegivel(f: FerramentaNoTurno): string {
  const segundos = (Date.now() - f.iniciadaEm) / 1000;
  return `${segundos.toFixed(1).replace(".", ",")} s`;
}

export function AbaJames({ estado, saidaVerTela, aoDecidir, analisador }: Props) {
  const conteudo = escolherConteudo(estado, saidaVerTela);

  return (
    <div className="flex h-full flex-col items-center gap-6 overflow-y-auto p-8">
      <Orbe analisador={analisador} />

      <div className="w-full max-w-xl">
        {conteudo === "aprovacao" && estado.aprovacao && (
          <div className="rounded-xl border border-amber-500/40 bg-amber-500/10 p-4">
            <p className="mb-2 text-sm text-app-text">
              O James quer executar <strong className="font-serif">{estado.aprovacao.ferramenta}</strong>
            </p>
            <pre className="mb-3 overflow-x-auto rounded bg-black/40 p-2 text-xs text-app-muted">
              {JSON.stringify(estado.aprovacao.argumentos, null, 2)}
            </pre>
            <div className="flex gap-2">
              <button
                type="button"
                onClick={() => aoDecidir(true)}
                className="rounded-lg bg-emerald-600 px-4 py-1.5 text-sm text-white transition-transform hover:scale-[1.03] active:scale-95"
              >
                Aprovar
              </button>
              <button
                type="button"
                onClick={() => aoDecidir(false)}
                className="rounded-lg bg-red-600 px-4 py-1.5 text-sm text-white transition-transform hover:scale-[1.03] active:scale-95"
              >
                Negar
              </button>
            </div>
          </div>
        )}

        {conteudo === "tela" && saidaVerTela && <LeituraDeTela saida={saidaVerTela} />}

        {conteudo === "ferramenta" && <Atividade ferramentas={estado.ferramentas} />}

        {conteudo === "repouso" && (
          <p className="text-center text-sm text-app-muted">Em repouso. Fale ou escreva para o James.</p>
        )}
      </div>
    </div>
  );
}

function Atividade({ ferramentas }: { ferramentas: FerramentaNoTurno[] }) {
  return (
    <ul className="space-y-1.5">
      {ferramentas.map((f, i) => (
        <li key={i} className="flex items-center gap-3 rounded-lg bg-white/5 px-3 py-2 text-sm">
          <span aria-hidden="true">{f.estado === "voando" ? "⚡" : f.estado === "erro" ? "✗" : "✓"}</span>
          <span className="flex-1 font-mono text-xs text-app-text">{f.nome}</span>
          <span className="text-xs text-app-muted">{duracaoLegivel(f)}</span>
          {f.erro && <span className="text-xs text-amber-300">{f.erro}</span>}
        </li>
      ))}
    </ul>
  );
}

function LeituraDeTela({ saida }: { saida: string }) {
  const { superficie, elementos } = interpretarSaidaVerTela(saida);
  return (
    <div className="space-y-2">
      {superficie && <p className="text-xs text-app-muted">{superficie}</p>}
      <ul className="max-h-64 overflow-y-auto rounded-lg bg-white/5 p-3 font-mono text-[11px] leading-relaxed text-app-text">
        {elementos.map((e, i) => (
          <li key={i}>{e}</li>
        ))}
      </ul>
    </div>
  );
}
