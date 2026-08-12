import { LayoutDashboard } from "lucide-react";

export function AbaDashboard() {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-3 p-8 text-center">
      <LayoutDashboard className="size-8 text-app-icon" />
      <h2 className="font-serif text-xl text-app-text">DashBoard</h2>
      <p className="max-w-md text-sm text-app-muted">
        Custo, uso, duração e erros ainda não são medidos. A instrumentação chega na Fase 7B — até lá esta
        aba fica vazia de propósito, em vez de mostrar número inventado.
      </p>
    </div>
  );
}
