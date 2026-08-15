import { useState } from "react";
import {
  Gem,
  Hourglass,
  LayoutDashboard,
  MessageCircle,
  PanelLeft,
  Sparkle,
  type LucideIcon,
} from "lucide-react";
import { arrastarJanela } from "../lib/janela";

export type IdAba = "james" | "mensagens" | "memoria" | "historico" | "dashboard";

interface ItemNav {
  id: IdAba;
  rotulo: string;
  Icone: LucideIcon;
}

const ITENS: ItemNav[] = [
  { id: "james", rotulo: "James", Icone: Sparkle },
  { id: "mensagens", rotulo: "Mensagens", Icone: MessageCircle },
  { id: "memoria", rotulo: "Memória", Icone: Gem },
  { id: "historico", rotulo: "Histórico", Icone: Hourglass },
  { id: "dashboard", rotulo: "DashBoard", Icone: LayoutDashboard },
];

interface Props {
  ativa: IdAba;
  aoSelecionar: (id: IdAba) => void;
  aprovacoesPendentes: number;
}

// nomeAcessivel é o que o leitor de tela anuncia ao focar o botão. Hoje o
// estado do turno guarda no máximo uma aprovação por vez, mas a prop é um
// número livre — pluralizar custa uma linha e evita "2 aprovação pendente"
// se isso mudar.
function nomeAcessivel(rotulo: string, pendentes: number): string {
  if (pendentes <= 0) return rotulo;
  const plural = pendentes === 1 ? "aprovação pendente" : "aprovações pendentes";
  return `${rotulo}, ${pendentes} ${plural}`;
}

export function BarraLateral({ ativa, aoSelecionar, aprovacoesPendentes }: Props) {
  const [expandida, setExpandida] = useState(true);

  // A janela do Wails é frameless — sem isto, ela só se move pelo atalho
  // do Windows (Win+seta) ou nem isso. O padding reservado (pt-11) e o
  // resto desta faixa viram a "barra de título": arrasta como qualquer
  // janela normal, exceto em cima de algo clicável (checado por
  // e.target !== e.currentTarget — só dispara quando o clique caiu no
  // próprio contêiner, não num filho como o botão de recolher).
  const aoApertarNaAreaReservada = (e: React.MouseEvent) => {
    if (e.target !== e.currentTarget) return;
    arrastarJanela(e);
  };

  return (
    <nav
      aria-label="Navegação principal"
      className={`flex h-full shrink-0 flex-col border-r border-app-line bg-app-sidebar transition-[width] duration-300 ease-out ${
        expandida ? "w-52" : "w-16"
      }`}
    >
      {/* pt-11 abre espaço para os controles de janela, que são posicionados
          por cima desta área pelo aplicativo.tsx */}
      <div
        data-testid="area-arrastavel"
        onMouseDown={aoApertarNaAreaReservada}
        className={`flex px-3 pt-11 pb-4 ${
          expandida ? "items-center justify-between gap-2" : "flex-col items-center gap-3"
        }`}
      >
        {expandida && (
          <span data-testid="rotulo-app" className="font-serif text-sm text-app-icon">
            James
          </span>
        )}
        <button
          type="button"
          onClick={() => setExpandida((v) => !v)}
          aria-label={expandida ? "Recolher menu" : "Expandir menu"}
          className="grid size-8 place-items-center rounded-md text-app-muted transition-colors hover:bg-app-hover hover:text-app-text"
        >
          <PanelLeft className="size-4" />
        </button>
      </div>

      <ul className="flex flex-1 flex-col gap-1 px-2">
        {ITENS.map(({ id, rotulo, Icone }) => {
          const estaAtiva = id === ativa;
          const pendentes = id === "james" ? aprovacoesPendentes : 0;
          return (
            <li key={id}>
              <button
                type="button"
                onClick={() => aoSelecionar(id)}
                aria-current={estaAtiva ? "page" : undefined}
                // A contagem entra AQUI, no nome do próprio botão, e não num
                // aria-label do badge: aria-label no botão substitui todo o
                // conteúdo descendente no cálculo do nome acessível, então
                // rótulo posto no <span> de dentro nunca seria anunciado.
                aria-label={nomeAcessivel(rotulo, pendentes)}
                // title é a pista para quem ENXERGA e não reconhece o ícone
                // com a barra recolhida — ali o rótulo textual não existe.
                // Não interfere no nome acessível, que o aria-label já fixa.
                title={rotulo}
                className={`flex w-full items-center rounded-lg text-left transition-colors ${
                  expandida ? "gap-3 px-3 py-2" : "justify-center px-0 py-2"
                } ${estaAtiva ? "bg-app-blue text-app-blue-foreground" : "text-app-icon hover:bg-app-hover"}`}
              >
                <span className="relative grid size-5 shrink-0 place-items-center">
                  <Icone className="size-5" />
                  {pendentes > 0 && (
                    // Puramente decorativo: o texto equivalente já está no
                    // nome do botão.
                    <span
                      aria-hidden="true"
                      className="absolute -right-1.5 -top-1.5 grid size-4 place-items-center rounded-full bg-amber-500 text-[10px] font-semibold text-black"
                    >
                      {pendentes}
                    </span>
                  )}
                </span>
                {expandida && <span className="font-serif text-sm">{rotulo}</span>}
              </button>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
