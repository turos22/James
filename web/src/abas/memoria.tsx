import { useEffect, useState } from "react";
import { lerMemoria, type RespostaMemoria } from "../lib/api";

export function Memoria() {
  const [dados, setDados] = useState<RespostaMemoria | null>(null);
  const [falha, setFalha] = useState<string>("");

  // Relê ao montar, não a cada turno: a memória muda por escrita do modelo,
  // não pela conversa. Abrir a aba de novo é o gesto de atualizar.
  useEffect(() => {
    lerMemoria()
      .then(setDados)
      .catch((e: Error) => setFalha(e.message));
  }, []);

  if (falha) {
    return (
      <p role="status" className="p-4 text-sm text-amber-300">
        Não consegui ler a memória: {falha}
      </p>
    );
  }
  if (!dados) return <p className="p-4 text-sm opacity-60">Carregando...</p>;

  return (
    <div className="h-full overflow-y-auto p-3">
      {dados.aviso && (
        <p role="status" className="mb-3 rounded bg-amber-500/20 px-3 py-1 text-xs">
          {dados.aviso}
        </p>
      )}
      {dados.notas.length === 0 && !dados.aviso && (
        <p className="text-sm opacity-60">Nenhuma nota guardada ainda.</p>
      )}
      {dados.notas.map((nota) => (
        <section key={nota.assunto} className="mb-4">
          <h3 className="mb-1 text-xs uppercase tracking-wide opacity-60">{nota.assunto}</h3>
          <pre className="whitespace-pre-wrap text-sm">{nota.conteudo}</pre>
        </section>
      ))}
    </div>
  );
}
