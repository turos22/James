import { useState } from "react";
import { BarraLateral, type IdAba } from "./barraLateral";
import { ControlesJanela } from "./controlesJanela";

interface Props {
  aprovacoesPendentes: number;
  // Cada aba é montada pelo App, que é quem tem o ClienteWS e o redutor.
  // O shell só decide qual mostrar — não conhece estado de turno.
  paineis: Record<IdAba, React.ReactNode>;
}

export function Aplicativo({ aprovacoesPendentes, paineis }: Props) {
  const [ativa, setAtiva] = useState<IdAba>("james");

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-app-bg text-app-text">
      <div className="relative">
        <ControlesJanela />
        <BarraLateral ativa={ativa} aoSelecionar={setAtiva} aprovacoesPendentes={aprovacoesPendentes} />
      </div>
      <main className="min-w-0 flex-1 overflow-hidden bg-app-panel">{paineis[ativa]}</main>
    </div>
  );
}
