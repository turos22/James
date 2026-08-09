import { useEffect, useState } from "react";
import { listarSessoes, type Sessao } from "../lib/api";

interface Props {
  sessaoAtual: string | null;
  aoRetomar: (id: string) => void;
}

export function Sessoes({ sessaoAtual, aoRetomar }: Props) {
  const [sessoes, setSessoes] = useState<Sessao[] | null>(null);
  const [falha, setFalha] = useState("");

  useEffect(() => {
    listarSessoes()
      .then(setSessoes)
      .catch((e: Error) => setFalha(e.message));
  }, []);

  if (falha) return <p role="status" className="p-4 text-sm text-amber-300">{falha}</p>;
  if (!sessoes) return <p className="p-4 text-sm opacity-60">Carregando...</p>;
  if (sessoes.length === 0) return <p className="p-4 text-sm opacity-60">Nenhuma conversa guardada.</p>;

  return (
    <ul className="h-full overflow-y-auto p-2">
      {sessoes.map((s) => (
        <li key={s.ID}>
          <button
            aria-current={s.ID === sessaoAtual}
            onClick={() => aoRetomar(s.ID)}
            className={`mb-1 w-full rounded px-3 py-2 text-left text-sm hover:bg-white/10 ${
              s.ID === sessaoAtual ? "bg-white/10" : ""
            }`}
          >
            <div className="truncate">{s.Titulo}</div>
            <div className="text-[10px] opacity-50">{new Date(s.AtualizadaEm).toLocaleString("pt-BR")}</div>
          </button>
        </li>
      ))}
    </ul>
  );
}
