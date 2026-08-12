import { useEffect, useState } from "react";
import { alternarMaximizado, fecharJanela, minimizarJanela, shellPresente } from "../lib/janela";

export function ControlesJanela() {
  const [dentroDoShell, setDentroDoShell] = useState(false);

  useEffect(() => {
    let vivo = true;
    shellPresente().then((presente) => {
      if (vivo) setDentroDoShell(presente);
    });
    return () => {
      vivo = false;
    };
  }, []);

  if (!dentroDoShell) return null;

  return (
    <div className="absolute left-4 top-4 z-10 flex gap-2">
      <button type="button" aria-label="Fechar" onClick={fecharJanela} className="size-3 rounded-full bg-[#ff5f57]" />
      <button type="button" aria-label="Minimizar" onClick={minimizarJanela} className="size-3 rounded-full bg-[#febc2e]" />
      <button
        type="button"
        aria-label="Maximizar"
        onClick={alternarMaximizado}
        className="size-3 rounded-full bg-[#28c840]"
      />
    </div>
  );
}
