'use client'

import { useState } from 'react'
import { Sidebar } from '@/components/sidebar'
import { JamesPanel } from '@/components/panels/james-panel'
import { MessagesPanel } from '@/components/panels/messages-panel'
import { MemoryPanel } from '@/components/panels/memory-panel'
import { HistoryPanel } from '@/components/panels/history-panel'
import { DashboardPanel } from '@/components/panels/dashboard-panel'
import type { NavId } from '@/lib/nav'
import { cn } from '@/lib/utils'

const PANELS: Record<NavId, React.ReactNode> = {
  james: <JamesPanel />,
  mensagens: <MessagesPanel />,
  memoria: <MemoryPanel />,
  historico: <HistoryPanel />,
  dashboard: <DashboardPanel />,
}

export function AppShell() {
  const [active, setActive] = useState<NavId>('james')
  const [expanded, setExpanded] = useState(true)

  return (
    <div className="flex h-[86vh] w-full max-w-6xl overflow-hidden rounded-xl border border-app-line bg-app-bg shadow-2xl shadow-black/50">
      {/* traffic lights column sits above sidebar via absolute header */}
      <div className="relative">
        <div
          className={cn(
            'absolute top-4 z-10 flex transition-all duration-300 ease-out',
            expanded ? 'left-4 gap-2' : 'left-3 gap-1',
          )}
        >
          <span className="size-3 rounded-full bg-[#ff5f57]" />
          <span className="size-3 rounded-full bg-[#febc2e]" />
          <span className="size-3 rounded-full bg-[#28c840]" />
        </div>
        <Sidebar
          active={active}
          onSelect={setActive}
          expanded={expanded}
          onToggle={() => setExpanded((v) => !v)}
        />
      </div>

      <main className="relative flex-1 overflow-hidden bg-app-panel">
        {PANELS[active]}
      </main>
    </div>
  )
}
