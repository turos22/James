import { useCallback, useEffect, useRef, useState } from "react";
import { Clock, Hourglass, Search } from "lucide-react";
import { listarSessoes, type Sessao } from "../lib/api";

const POR_PAGINA = 50;

interface Props {
  sessaoAtual: string | null;
  aoRetomar: (id: string) => void;
}

// rotuloDoDia agrupa como o design de referência: Hoje, Ontem, depois data.
export function rotuloDoDia(iso: string, agora = new Date()): string {
  const data = new Date(iso);
  const dia = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  const diferenca = (dia(agora) - dia(data)) / 86_400_000;
  if (diferenca === 0) return "Hoje";
  if (diferenca === 1) return "Ontem";
  return data.toLocaleDateString("pt-BR", { day: "2-digit", month: "short" });
}

// O título é a primeira mensagem inteira do usuário (ws.go: Criar(ctx, texto)),
// sem truncar na origem. O corte é aqui, na exibição. Array.from (não
// slice/length direto) porque títulos vêm de mensagem livre do usuário e
// podem ter emoji fora do BMP (2 code units UTF-16 cada) — slice ingênuo
// partiria um desses ao meio e renderizaria par substituto solto.
export function preview(titulo: string): string {
  const limpo = titulo.trim().replace(/\s+/g, " ");
  const caracteres = Array.from(limpo);
  return caracteres.length > 80 ? `${caracteres.slice(0, 80).join("")}…` : limpo || "(sem título)";
}

export function AbaHistorico({ sessaoAtual, aoRetomar }: Props) {
  const [sessoes, setSessoes] = useState<Sessao[]>([]);
  const [busca, setBusca] = useState("");
  const [carregando, setCarregando] = useState(true);
  const [acabou, setAcabou] = useState(false);
  const [falha, setFalha] = useState("");
  // Descarta resposta desatualizada quando busca e paginação correm ao
  // mesmo tempo e a rede devolve fora de ordem: cada carregar() incrementa
  // a geração, e só a resposta da geração mais recente atualiza o estado.
  // Sem isso, uma paginação atrasada (termo velho) que resolve DEPOIS de
  // uma busca nova grudaria resultado do termo velho em cima da lista do
  // termo novo — sem erro nenhum visível pro usuário.
  const geracaoRef = useRef(0);

  const carregar = useCallback(
    async (termo: string, cursor?: string) => {
      const geracao = ++geracaoRef.current;
      setCarregando(true);
      setFalha("");
      try {
        const pagina = await listarSessoes({ limite: POR_PAGINA, busca: termo || undefined, antes: cursor });
        if (geracao !== geracaoRef.current) return;
        setSessoes((anteriores) => (cursor ? [...anteriores, ...pagina] : pagina));
        setAcabou(pagina.length < POR_PAGINA);
      } catch (e) {
        if (geracao !== geracaoRef.current) return;
        setFalha((e as Error).message);
      } finally {
        if (geracao === geracaoRef.current) setCarregando(false);
      }
    },
    [],
  );

  // Debounce para a busca não disparar uma requisição por tecla.
  useEffect(() => {
    const timer = setTimeout(() => {
      void carregar(busca);
    }, 250);
    return () => clearTimeout(timer);
  }, [busca, carregar]);

  const carregarMais = () => {
    const ultima = sessoes[sessoes.length - 1];
    if (ultima) void carregar(busca, ultima.AtualizadaEm);
  };

  const dias: string[] = [];
  for (const s of sessoes) {
    const dia = rotuloDoDia(s.AtualizadaEm);
    if (!dias.includes(dia)) dias.push(dia);
  }

  return (
    <div className="flex h-full flex-col">
      <header className="space-y-4 border-b border-app-line px-6 py-5">
        <div className="flex items-center gap-2">
          <Hourglass className="size-5 text-app-icon" />
          <h2 className="font-serif text-xl text-app-text">Histórico</h2>
        </div>
        <div className="flex items-center gap-2 rounded-lg bg-white/5 px-3 py-2">
          <Search className="size-4 text-app-muted" />
          <input
            value={busca}
            onChange={(e) => setBusca(e.target.value)}
            placeholder="Buscar conversas…"
            className="flex-1 bg-transparent text-sm text-app-text placeholder:text-app-muted focus:outline-none"
          />
        </div>
      </header>

      <div className="flex-1 overflow-y-auto px-6 py-5">
        {falha && (
          <div className="space-y-2">
            <p role="status" className="text-sm text-amber-300">
              {falha}
            </p>
            <button
              type="button"
              onClick={() => void carregar(busca)}
              className="rounded-lg bg-white/5 px-3 py-1.5 text-xs text-app-text hover:bg-app-hover"
            >
              Tentar de novo
            </button>
          </div>
        )}

        {dias.map((dia) => (
          <div key={dia} className="mb-6">
            <p className="mb-2 text-xs font-medium uppercase tracking-wide text-app-muted">{dia}</p>
            <ul className="space-y-1">
              {sessoes
                .filter((s) => rotuloDoDia(s.AtualizadaEm) === dia)
                .map((s) => (
                  <li key={s.ID}>
                    <button
                      type="button"
                      aria-current={s.ID === sessaoAtual ? "true" : undefined}
                      onClick={() => aoRetomar(s.ID)}
                      className={`flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left transition-colors hover:bg-app-hover ${
                        s.ID === sessaoAtual ? "bg-app-hover" : ""
                      }`}
                    >
                      <span className="grid size-8 shrink-0 place-items-center rounded-md bg-white/5 text-app-muted">
                        <Clock className="size-4" />
                      </span>
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-sm text-app-text">{preview(s.Titulo)}</span>
                        <span className="text-xs text-app-muted">
                          {new Date(s.AtualizadaEm).toLocaleTimeString("pt-BR", {
                            hour: "2-digit",
                            minute: "2-digit",
                          })}
                          {/* A coluna de tokens do design chega na Fase 7B:
                              nada mede token hoje, e número inventado é pior
                              que coluna ausente (D12). */}
                        </span>
                      </span>
                    </button>
                  </li>
                ))}
            </ul>
          </div>
        ))}

        {!carregando && !falha && sessoes.length === 0 && (
          <p className="mt-10 text-center text-sm text-app-muted">Nenhuma conversa encontrada.</p>
        )}

        {!acabou && sessoes.length > 0 && (
          <button
            type="button"
            onClick={carregarMais}
            disabled={carregando}
            className="mx-auto block rounded-lg bg-white/5 px-4 py-2 text-xs text-app-text hover:bg-app-hover disabled:opacity-40"
          >
            Carregar mais
          </button>
        )}
      </div>
    </div>
  );
}
