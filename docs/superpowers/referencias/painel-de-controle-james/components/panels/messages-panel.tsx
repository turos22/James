'use client'

import { useRef, useState } from 'react'
import { Send, Sparkle } from 'lucide-react'
import { cn } from '@/lib/utils'

type Msg = { id: number; from: 'james' | 'me'; text: string }

const INITIAL: Msg[] = [
  { id: 1, from: 'james', text: 'Oi! Sou o James. Em que posso te ajudar hoje?' },
  { id: 2, from: 'me', text: 'Resuma minhas reuniões de ontem, por favor.' },
  {
    id: 3,
    from: 'james',
    text: 'Claro. Você teve 3 reuniões: planejamento do Q3, revisão de design e uma call com o cliente Vega. Quer os detalhes de alguma?',
  },
  { id: 4, from: 'me', text: 'A da Vega.' },
  {
    id: 5,
    from: 'james',
    text: 'A Vega aprovou o protótipo e pediu ajustes no fluxo de checkout até sexta. Adicionei isso ao seu histórico.',
  },
]

export function MessagesPanel() {
  const [messages, setMessages] = useState<Msg[]>(INITIAL)
  const [draft, setDraft] = useState('')
  const idRef = useRef(INITIAL.length + 1)

  const send = () => {
    const text = draft.trim()
    if (!text) return
    const mine: Msg = { id: idRef.current++, from: 'me', text }
    setMessages((m) => [...m, mine])
    setDraft('')
    setTimeout(() => {
      setMessages((m) => [
        ...m,
        {
          id: idRef.current++,
          from: 'james',
          text: 'Entendido! Já registrei isso e volto com uma resposta completa em instantes.',
        },
      ])
    }, 700)
  }

  return (
    <div className="flex h-full flex-col">
      <header className="flex items-center gap-3 border-b border-app-line px-6 py-4">
        <span className="grid size-9 place-items-center rounded-full bg-app-blue text-app-blue-foreground">
          <Sparkle className="size-4" />
        </span>
        <div>
          <p className="font-serif text-app-text">James</p>
          <p className="text-xs text-emerald-400">online</p>
        </div>
      </header>

      <div className="flex-1 space-y-4 overflow-y-auto px-6 py-6">
        {messages.map((m) => (
          <div
            key={m.id}
            className={cn('flex', m.from === 'me' ? 'justify-end' : 'justify-start')}
          >
            <div
              className={cn(
                'max-w-[75%] rounded-2xl px-4 py-2.5 text-sm leading-relaxed',
                m.from === 'me'
                  ? 'rounded-br-sm bg-app-blue text-app-blue-foreground'
                  : 'rounded-bl-sm bg-white/5 text-app-text',
              )}
            >
              {m.text}
            </div>
          </div>
        ))}
      </div>

      <div className="border-t border-app-line p-4">
        <div className="flex items-center gap-2 rounded-xl bg-white/5 px-3 py-2">
          <input
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.nativeEvent.isComposing && e.keyCode !== 229) {
                e.preventDefault()
                send()
              }
            }}
            placeholder="Escreva para o James…"
            className="flex-1 bg-transparent text-sm text-app-text placeholder:text-app-muted focus:outline-none"
          />
          <button
            type="button"
            onClick={send}
            aria-label="Enviar mensagem"
            className="grid size-8 place-items-center rounded-lg bg-app-blue text-app-blue-foreground transition-transform hover:scale-105 active:scale-95"
          >
            <Send className="size-4" />
          </button>
        </div>
      </div>
    </div>
  )
}
