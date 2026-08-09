import type { EstadoTurno } from "../estado/turno";

export function Atividade({ estado }: { estado: EstadoTurno }) {
  if (estado.ferramentas.length === 0) {
    return <p className="p-4 text-sm opacity-60">Nenhuma ferramenta neste turno.</p>;
  }

  return (
    <ul className="h-full overflow-y-auto p-3 font-mono text-xs">
      {estado.ferramentas.map((f, i) => (
        <li key={i} className="mb-1">
          <span className="mr-2">{f.estado === "voando" ? "⚡" : f.estado === "erro" ? "✗" : "✓"}</span>
          {f.nome}
          {f.erro && <span className="ml-2 text-amber-300">{f.erro}</span>}
        </li>
      ))}
    </ul>
  );
}
