'use client'

import { PanelLeft } from 'lucide-react'
import { NAV_ITEMS, type NavId } from '@/lib/nav'
import { cn } from '@/lib/utils'

type SidebarProps = {
  active: NavId
  onSelect: (id: NavId) => void
  expanded: boolean
  onToggle: () => void
}

export function Sidebar({ active, onSelect, expanded, onToggle }: SidebarProps) {
  return (
    <nav
      aria-label="Navegação principal"
      className={cn(
        'flex h-full flex-col border-r border-app-line bg-app-sidebar transition-[width] duration-300 ease-out',
        expanded ? 'w-52' : 'w-16',
      )}
    >
      <div
        className={cn(
          'flex px-3 pt-11 pb-4',
          expanded
            ? 'items-center justify-between gap-2'
            : 'flex-col items-center gap-3',
        )}
      >
        <span className="flex items-center gap-2 overflow-hidden">
          <img
            src="/james-logo.png"
            alt="Logo do James"
            className="size-7 shrink-0 object-contain"
          />
          {expanded && (
            <span className="font-serif text-sm text-app-icon">James</span>
          )}
        </span>
        <button
          type="button"
          onClick={onToggle}
          aria-label={expanded ? 'Recolher menu' : 'Expandir menu'}
          className="grid size-8 place-items-center rounded-md text-app-muted transition-colors hover:bg-app-hover hover:text-app-text"
        >
          <PanelLeft className="size-4" />
        </button>
      </div>

      <ul className="flex flex-1 flex-col gap-1 px-2">
        {NAV_ITEMS.map((item) => {
          const isActive = item.id === active
          const Icon = item.icon
          return (
            <li key={item.id}>
              <button
                type="button"
                onClick={() => onSelect(item.id)}
                aria-current={isActive ? 'page' : undefined}
                title={item.label}
                className={cn(
                  'group flex w-full items-center rounded-lg text-left transition-colors',
                  expanded ? 'gap-3 px-3 py-2' : 'justify-center px-0 py-2',
                  isActive
                    ? 'bg-app-blue text-app-blue-foreground'
                    : 'text-app-icon hover:bg-app-hover',
                )}
              >
                <span
                  className={cn(
                    'grid shrink-0 place-items-center rounded-md transition-colors',
                    expanded ? 'size-5' : 'size-9',
                    !expanded && isActive && 'bg-app-blue text-app-blue-foreground',
                  )}
                >
                  <Icon className="size-5" />
                </span>
                {expanded && (
                  <span
                    className={cn(
                      'font-serif text-sm',
                      isActive ? 'text-app-blue-foreground' : 'text-app-icon',
                    )}
                  >
                    {item.label}
                  </span>
                )}
              </button>
            </li>
          )
        })}
      </ul>
    </nav>
  )
}
