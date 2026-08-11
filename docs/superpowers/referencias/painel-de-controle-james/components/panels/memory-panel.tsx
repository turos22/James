'use client'

import { useState } from 'react'
import { FileText, Folder, Hash, Link2, Gem } from 'lucide-react'
import { cn } from '@/lib/utils'

type Note = {
  id: string
  title: string
  tags: string[]
  links: string[]
  body: string
}

const NOTES: Note[] = [
  {
    id: 'vega',
    title: 'Cliente Vega',
    tags: ['cliente', 'ativo'],
    links: ['Checkout', 'Reunião Q3'],
    body: 'A Vega aprovou o protótipo. Precisa de ajustes no [[Checkout]] até sexta.\n\nContato principal: Marina Alves.',
  },
  {
    id: 'checkout',
    title: 'Checkout',
    tags: ['produto', 'fluxo'],
    links: ['Cliente Vega'],
    body: 'Refatorar o fluxo em 3 etapas. Reduzir campos do formulário e adicionar carteira digital.',
  },
  {
    id: 'q3',
    title: 'Reunião Q3',
    tags: ['planejamento'],
    links: ['Cliente Vega', 'Checkout'],
    body: 'Metas do trimestre: lançar v2, dobrar retenção e fechar 4 contas enterprise.',
  },
]

export function MemoryPanel() {
  const [activeId, setActiveId] = useState(NOTES[0].id)
  const note = NOTES.find((n) => n.id === activeId)!

  return (
    <div className="grid h-full grid-cols-[180px_1fr_180px] max-lg:grid-cols-[160px_1fr]">
      {/* explorer */}
      <aside className="border-r border-app-line px-3 py-5">
        <div className="mb-3 flex items-center gap-2 px-1 text-app-muted">
          <Folder className="size-4" />
          <span className="text-xs font-medium uppercase tracking-wide">Cofre</span>
        </div>
        <ul className="space-y-0.5">
          {NOTES.map((n) => (
            <li key={n.id}>
              <button
                type="button"
                onClick={() => setActiveId(n.id)}
                className={cn(
                  'flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition-colors',
                  n.id === activeId
                    ? 'bg-app-blue/20 text-app-text'
                    : 'text-app-muted hover:bg-app-hover hover:text-app-text',
                )}
              >
                <FileText className="size-3.5 shrink-0" />
                <span className="truncate">{n.title}</span>
              </button>
            </li>
          ))}
        </ul>
      </aside>

      {/* editor */}
      <section className="overflow-y-auto px-8 py-6">
        <div className="mb-4 flex items-center gap-2">
          <Gem className="size-5 text-app-icon" />
          <h2 className="font-serif text-2xl text-app-text">{note.title}</h2>
        </div>
        <div className="mb-5 flex flex-wrap gap-2">
          {note.tags.map((t) => (
            <span
              key={t}
              className="flex items-center gap-1 rounded-full bg-white/5 px-2.5 py-0.5 text-xs text-app-icon"
            >
              <Hash className="size-3" />
              {t}
            </span>
          ))}
        </div>
        <div className="space-y-3 text-sm leading-relaxed text-app-text/90">
          {note.body.split('\n').map((line, i) =>
            line ? (
              <p key={i}>
                {line.split(/(\[\[.*?\]\])/g).map((part, j) =>
                  part.startsWith('[[') ? (
                    <span key={j} className="text-app-icon underline decoration-app-icon/40">
                      {part.replace(/\[\[|\]\]/g, '')}
                    </span>
                  ) : (
                    <span key={j}>{part}</span>
                  ),
                )}
              </p>
            ) : (
              <div key={i} className="h-2" />
            ),
          )}
        </div>
      </section>

      {/* backlinks / graph */}
      <aside className="border-l border-app-line px-4 py-5 max-lg:hidden">
        <div className="mb-3 flex items-center gap-2 text-app-muted">
          <Link2 className="size-4" />
          <span className="text-xs font-medium uppercase tracking-wide">Conexões</span>
        </div>
        <ul className="mb-6 space-y-1.5">
          {note.links.map((l) => (
            <li key={l}>
              <button
                type="button"
                onClick={() => {
                  const t = NOTES.find((n) => n.title === l)
                  if (t) setActiveId(t.id)
                }}
                className="w-full rounded-md bg-white/5 px-2.5 py-1.5 text-left text-xs text-app-text/80 transition-colors hover:bg-app-hover"
              >
                {l}
              </button>
            </li>
          ))}
        </ul>
        <MiniGraph active={note.title} />
      </aside>
    </div>
  )
}

function MiniGraph({ active }: { active: string }) {
  const nodes = [
    { x: 75, y: 30 },
    { x: 30, y: 90 },
    { x: 120, y: 100 },
  ]
  return (
    <div>
      <p className="mb-2 text-xs text-app-muted">Grafo</p>
      <svg viewBox="0 0 150 140" className="w-full" aria-hidden="true">
        {nodes.map((n, i) =>
          nodes.slice(i + 1).map((m, j) => (
            <line
              key={`${i}-${j}`}
              x1={n.x}
              y1={n.y}
              x2={m.x}
              y2={m.y}
              stroke="oklch(0.58 0.22 266 / 0.4)"
              strokeWidth="1"
            />
          )),
        )}
        {nodes.map((n, i) => (
          <circle
            key={i}
            cx={n.x}
            cy={n.y}
            r={i === 0 ? 9 : 6}
            fill={i === 0 ? 'oklch(0.52 0.24 267)' : 'oklch(0.58 0.22 266 / 0.5)'}
          />
        ))}
      </svg>
      <p className="mt-1 text-center text-[10px] text-app-muted">{active}</p>
    </div>
  )
}
