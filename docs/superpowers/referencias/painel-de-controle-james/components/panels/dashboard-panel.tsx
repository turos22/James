'use client'

import { Coins, Clock3, MessageSquare, TriangleAlert, TrendingUp, TrendingDown } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

const STATS: {
  label: string
  value: string
  delta: string
  up: boolean
  icon: LucideIcon
}[] = [
  { label: 'Custo em tokens', value: 'R$ 342,80', delta: '+12%', up: true, icon: Coins },
  { label: 'Tempo de uso', value: '48h 12m', delta: '+6%', up: true, icon: Clock3 },
  { label: 'Mensagens enviadas', value: '12.480', delta: '+18%', up: true, icon: MessageSquare },
  { label: 'Total de erros', value: '37', delta: '-24%', up: false, icon: TriangleAlert },
]

const TOKENS = [42, 55, 38, 70, 61, 88, 74]
const DAYS = ['Seg', 'Ter', 'Qua', 'Qui', 'Sex', 'Sáb', 'Dom']

export function DashboardPanel() {
  const max = Math.max(...TOKENS)

  return (
    <div className="h-full overflow-y-auto p-6 lg:p-8">
      <div className="mb-6">
        <h2 className="font-serif text-2xl text-app-text">DashBoard</h2>
        <p className="text-sm text-app-muted">Visão geral dos últimos 7 dias</p>
      </div>

      <div className="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
        {STATS.map((s) => {
          const Icon = s.icon
          return (
            <div
              key={s.label}
              className="rounded-xl border border-app-line bg-app-sidebar p-4"
            >
              <div className="mb-3 flex items-center justify-between">
                <span className="grid size-9 place-items-center rounded-lg bg-app-blue/15 text-app-icon">
                  <Icon className="size-4" />
                </span>
                <span
                  className={`flex items-center gap-0.5 text-xs ${
                    s.up ? 'text-emerald-400' : 'text-emerald-400'
                  }`}
                >
                  {s.delta.startsWith('-') ? (
                    <TrendingDown className="size-3" />
                  ) : (
                    <TrendingUp className="size-3" />
                  )}
                  {s.delta}
                </span>
              </div>
              <p className="font-serif text-2xl text-app-text">{s.value}</p>
              <p className="text-xs text-app-muted">{s.label}</p>
            </div>
          )
        })}
      </div>

      <div className="grid gap-4 lg:grid-cols-3">
        <div className="rounded-xl border border-app-line bg-app-sidebar p-5 lg:col-span-2">
          <p className="mb-4 text-sm font-medium text-app-text">Consumo de tokens por dia</p>
          <div className="flex h-44 items-end gap-3">
            {TOKENS.map((t, i) => (
              <div
                key={i}
                className="flex h-full flex-1 flex-col items-center justify-end gap-2"
              >
                <div
                  className="w-full rounded-t-md bg-app-blue transition-all"
                  style={{ height: `${Math.round((t / max) * 150)}px` }}
                />
                <span className="text-[10px] text-app-muted">{DAYS[i]}</span>
              </div>
            ))}
          </div>
        </div>

        <div className="rounded-xl border border-app-line bg-app-sidebar p-5">
          <p className="mb-4 text-sm font-medium text-app-text">Erros por tipo</p>
          <ul className="space-y-3">
            {[
              { name: 'Timeout', value: 62 },
              { name: 'Rate limit', value: 24 },
              { name: 'Parsing', value: 14 },
            ].map((e) => (
              <li key={e.name}>
                <div className="mb-1 flex justify-between text-xs">
                  <span className="text-app-text">{e.name}</span>
                  <span className="text-app-muted">{e.value}%</span>
                </div>
                <div className="h-2 overflow-hidden rounded-full bg-white/5">
                  <div
                    className="h-full rounded-full bg-app-icon"
                    style={{ width: `${e.value}%` }}
                  />
                </div>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </div>
  )
}
