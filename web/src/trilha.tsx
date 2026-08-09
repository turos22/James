export type IdAba = "conversa" | "tela" | "atividade" | "memoria" | "sessoes";

export const ABAS: { id: IdAba; rotulo: string; icone: string }[] = [
  { id: "conversa", rotulo: "Conversa", icone: "💬" },
  { id: "tela", rotulo: "Tela", icone: "🖥" },
  { id: "atividade", rotulo: "Atividade", icone: "🔧" },
  { id: "memoria", rotulo: "Memória", icone: "🧠" },
  { id: "sessoes", rotulo: "Sessões", icone: "🕘" },
];

interface Props {
  ativa: IdAba;
  aoTrocar: (id: IdAba) => void;
}

// Trilha vertical no desktop, barra horizontal embaixo no celular — o
// colapso é só a direção do flex, sem componente separado (CA8).
export function Trilha({ ativa, aoTrocar }: Props) {
  return (
    <nav
      role="tablist"
      aria-label="Seções do James"
      className="flex flex-row justify-around border-t border-white/10 md:flex-col md:justify-start md:gap-1 md:border-t-0 md:border-r md:p-2"
    >
      {ABAS.map((aba) => (
        <button
          key={aba.id}
          role="tab"
          aria-selected={ativa === aba.id}
          onClick={() => aoTrocar(aba.id)}
          className={`flex flex-col items-center gap-0.5 rounded px-3 py-2 text-[10px] transition-opacity ${
            ativa === aba.id ? "opacity-100" : "opacity-50 hover:opacity-80"
          }`}
        >
          <span aria-hidden="true" className="text-base">
            {aba.icone}
          </span>
          {aba.rotulo}
        </button>
      ))}
    </nav>
  );
}
