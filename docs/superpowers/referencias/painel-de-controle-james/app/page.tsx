import { AppShell } from '@/components/app-shell'

export default function Page() {
  return (
    <main
      className="flex min-h-screen items-center justify-center bg-app-bg p-4 sm:p-8"
      style={{
        backgroundImage:
          'radial-gradient(oklch(0.82 0.01 265 / 0.12) 1px, transparent 1px)',
        backgroundSize: '18px 18px',
      }}
    >
      <AppShell />
    </main>
  )
}
