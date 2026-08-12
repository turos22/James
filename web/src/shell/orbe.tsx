interface Props {
  analisador: AnalyserNode | null;
}

// Esqueleto: o canvas reativo entra na Task 8.
export function Orbe({ analisador }: Props) {
  void analisador;
  return <div className="size-[320px] shrink-0 rounded-full bg-black/60" aria-label="Orbe do James" />;
}
