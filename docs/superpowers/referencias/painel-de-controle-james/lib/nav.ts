import type { LucideIcon } from 'lucide-react'
import { Sparkle, MessageCircle, Gem, Hourglass, LayoutDashboard } from 'lucide-react'

export type NavId = 'james' | 'mensagens' | 'memoria' | 'historico' | 'dashboard'

export type NavItem = {
  id: NavId
  label: string
  icon: LucideIcon
}

export const NAV_ITEMS: NavItem[] = [
  { id: 'james', label: 'James', icon: Sparkle },
  { id: 'mensagens', label: 'Mensagens', icon: MessageCircle },
  { id: 'memoria', label: 'Memória', icon: Gem },
  { id: 'historico', label: 'Histórico', icon: Hourglass },
  { id: 'dashboard', label: 'DashBoard', icon: LayoutDashboard },
]
