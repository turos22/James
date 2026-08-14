import { useEffect, useState } from "react";
import { Gem } from "lucide-react";
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
  if (!dados) return <p className="p-4 text-sm text-app-muted">Carregando...</p>;

  return (
    <div className="flex h-full flex-col">
      <header className="border-b border-app-line px-6 py-5">
        <div className="flex items-center gap-2">
          <Gem className="size-5 text-app-icon" />
          <h2 className="font-serif text-xl text-app-text">Memória</h2>
        </div>
      </header>
      <div className="flex-1 overflow-y-auto px-6 py-5">
        {dados.aviso && (
          <p role="status" className="mb-3 rounded bg-amber-500/20 px-3 py-1 text-xs">
            {dados.aviso}
          </p>
        )}
        {dados.notas.length === 0 && !dados.aviso && (
          <p className="text-sm text-app-muted">Nenhuma nota guardada ainda.</p>
        )}
        {dados.notas.map((nota) => (
          <section key={nota.assunto} className="mb-4">
            <h3 className="mb-1 text-xs uppercase tracking-wide text-app-muted">{nota.assunto}</h3>
            <pre className="whitespace-pre-wrap text-sm">{nota.conteudo}</pre>
          </section>
        ))}
      </div>
    </div>
  );
}
