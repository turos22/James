interface Props {
  ferramenta: string;
  argumentos: Record<string, unknown>;
  onDecidir: (aprovado: boolean) => void;
}

// Card mínimo funcional (decisão P4) — sem redesenho, só o necessário pro
// critério de aceitação #6: perfil Restrito pedindo ação perigosa vira
// algo que o Arthur consegue aprovar ou negar no desktop.
export function CardAprovacao({ ferramenta, argumentos, onDecidir }: Props) {
  return (
    <div className="card-aprovacao">
      <p>
        O James quer executar <strong>{ferramenta}</strong>
      </p>
      <pre>{JSON.stringify(argumentos, null, 2)}</pre>
      <div className="card-aprovacao-botoes">
        <button onClick={() => onDecidir(true)}>Aprovar</button>
        <button onClick={() => onDecidir(false)}>Negar</button>
      </div>
    </div>
  );
}
