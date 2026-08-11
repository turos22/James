'use client'

import { Clock, Hourglass, Search } from 'lucide-react'
import { useState } from 'react'

type Entry = {
  id: number
  time: string
  day: string
  preview: string
  tokens: number
}

const HISTORY: Entry[] = [
  { id: 1, time: '14:32', day: 'Hoje', preview: 'Resumo das reuniões de ontem', tokens: 1240 },
  { id: 2, time: '11:05', day: 'Hoje', preview: 'Rascunho de e-mail para a Vega', tokens: 860 },
  { id: 3, time: '09:47', day: 'Hoje', preview: 'Ideias para o fluxo de checkout', tokens: 2130 },
  { id: 4, time: '18:20', day: 'Ontem', preview: 'Análise de métricas do Q2', tokens: 3410 },
  { id: 5, time: '16:02', day: 'Ontem', preview: 'Tradução de documento técnico', tokens: 1580 },
  { id: 6, time: '10:15', day: 'Ontem', preview: 'Planejamento da sprint', tokens: 990 },
  { id: 7, time: '15:40', day: '12 mar', preview: 'Debug de erro no deploy', tokens: 2670 },
]

export function HistoryPanel() {
  const [query, setQuery] = useState('')
  const filtered = HISTORY.filter((e) =>
    e.preview.toLowerCase().includes(query.toLowerCase()),
  )
  const days = Array.from(new Set(filtered.map((e) => e.day)))

  return (
    <div className="flex h-full flex-col">
      <header className="space-y-4 border-b border-app-line px-6 py-5">
        <div className="flex items-center gap-2">
          <Hourglass className="size-5 text-app-icon" />
          <h2 className="font-serif text-xl text-app-text">Histórico</h2>
        </div>
        <div className="flex items-center gap-2 rounded-lg bg-white/5 px-3 py-2">
          <Search className="size-4 text-app-muted" />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Buscar conversas…"
            className="flex-1 bg-transparent text-sm text-app-text placeholder:text-app-muted focus:outline-none"
          />
        </div>
      </header>

      <div className="flex-1 overflow-y-auto px-6 py-5">
        {days.map((day) => (
          <div key={day} className="mb-6">
            <p className="mb-2 text-xs font-medium uppercase tracking-wide text-app-muted">
              {day}
            </p>
            <ul className="space-y-1">
              {filtered
                .filter((e) => e.day === day)
                .map((e) => (
                  <li key={e.id}>
                    <button
                      type="button"
                      className="flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left transition-colors hover:bg-app-hover"
                    >
                      <span className="grid size-8 shrink-0 place-items-center rounded-md bg-white/5 text-app-muted">
                        <Clock className="size-4" />
                      </span>
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-sm text-app-text">
                          {e.preview}
                        </span>
                        <span className="text-xs text-app-muted">
                          {e.time} · {e.tokens.toLocaleString('pt-BR')} tokens
                        </span>
                      </span>
                    </button>
                  </li>
                ))}
            </ul>
          </div>
        ))}
        {filtered.length === 0 && (
          <p className="mt-10 text-center text-sm text-app-muted">
            Nenhuma conversa encontrada.
          </p>
        )}
      </div>
    </div>
  )
}
