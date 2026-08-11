'use client'

import { useEffect, useRef, useState } from 'react'
import { Play, Pause } from 'lucide-react'

export function JamesPanel() {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const logoRef = useRef<HTMLImageElement>(null)
  const audioCtxRef = useRef<AudioContext | null>(null)
  const analyserRef = useRef<AnalyserNode | null>(null)
  const sourcesRef = useRef<{ osc: OscillatorNode; lfo: OscillatorNode; gain: GainNode }[]>([])
  const rafRef = useRef<number>(0)
  const [playing, setPlaying] = useState(false)

  // Draw loop — a shadowed black orb whose border wobbles and whose size
  // breathes with the live audio volume. The logo floats on top of it.
  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const dpr = Math.min(window.devicePixelRatio || 1, 2)
    const size = 320
    canvas.width = size * dpr
    canvas.height = size * dpr
    ctx.scale(dpr, dpr)

    const cx = size / 2
    const cy = size / 2
    let idle = 0

    const render = () => {
      idle += 0.03
      const analyser = analyserRef.current
      let data: Uint8Array | null = null
      if (analyser) {
        data = new Uint8Array(analyser.frequencyBinCount)
        analyser.getByteFrequencyData(data)
      }

      // overall energy (0..1) -> orb size + glow
      let energy = 0
      if (data) {
        for (let i = 0; i < data.length; i++) energy += data[i]
        energy = energy / data.length / 255
      } else {
        energy = 0.12 + (Math.sin(idle) * 0.5 + 0.5) * 0.1
      }

      ctx.clearRect(0, 0, size, size)

      const baseR = 96 + energy * 34
      const wobblePts = 96

      // wobbling border path
      ctx.beginPath()
      for (let i = 0; i <= wobblePts; i++) {
        const angle = (i / wobblePts) * Math.PI * 2
        let amp = 0
        if (data) {
          const bin = Math.floor((i / wobblePts) * (data.length - 1))
          amp = (data[bin] || 0) / 255
        }
        const wobble =
          Math.sin(angle * 3 + idle * 1.6) * (4 + amp * 14) +
          Math.sin(angle * 6 - idle * 1.1) * (3 + amp * 10) +
          Math.cos(angle * 2 + idle) * 3
        const r = baseR + wobble
        const x = cx + Math.cos(angle) * r
        const y = cy + Math.sin(angle) * r
        if (i === 0) ctx.moveTo(x, y)
        else ctx.lineTo(x, y)
      }
      ctx.closePath()

      // deep shadow around the black orb
      ctx.save()
      ctx.shadowColor = 'rgba(0,0,0,0.85)'
      ctx.shadowBlur = 50 + energy * 60
      const grad = ctx.createRadialGradient(cx, cy - 20, 20, cx, cy, baseR + 20)
      grad.addColorStop(0, '#1a1a1f')
      grad.addColorStop(0.7, '#0a0a0c')
      grad.addColorStop(1, '#000000')
      ctx.fillStyle = grad
      ctx.fill()
      ctx.restore()

      // subtle moving rim highlight
      ctx.save()
      ctx.lineWidth = 2
      ctx.strokeStyle = `rgba(120,140,255,${0.15 + energy * 0.4})`
      ctx.shadowColor = 'rgba(90,110,255,0.6)'
      ctx.shadowBlur = 18 + energy * 30
      ctx.stroke()
      ctx.restore()

      // scale the logo with the volume
      if (logoRef.current) {
        const scale = 1 + energy * 0.28
        logoRef.current.style.transform = `translate(-50%, -50%) scale(${scale.toFixed(3)})`
      }

      rafRef.current = requestAnimationFrame(render)
    }

    render()
    return () => cancelAnimationFrame(rafRef.current)
  }, [])

  const stop = () => {
    sourcesRef.current.forEach(({ osc, lfo }) => {
      try {
        osc.stop()
        lfo.stop()
      } catch {}
    })
    sourcesRef.current = []
    setPlaying(false)
  }

  const start = () => {
    const Ctx = window.AudioContext || (window as any).webkitAudioContext
    const audioCtx = audioCtxRef.current ?? new Ctx()
    audioCtxRef.current = audioCtx
    if (audioCtx.state === 'suspended') void audioCtx.resume()

    const analyser = audioCtx.createAnalyser()
    analyser.fftSize = 128
    analyser.connect(audioCtx.destination)
    analyserRef.current = analyser

    // Ambient chord so the orb has something rich to react to.
    const freqs = [174, 261, 329, 392]
    freqs.forEach((f, idx) => {
      const osc = audioCtx.createOscillator()
      const gain = audioCtx.createGain()
      const lfo = audioCtx.createOscillator()
      const lfoGain = audioCtx.createGain()

      osc.type = idx % 2 === 0 ? 'sine' : 'triangle'
      osc.frequency.value = f
      gain.gain.value = 0.08

      lfo.frequency.value = 0.15 + idx * 0.12
      lfoGain.gain.value = 0.06
      lfo.connect(lfoGain)
      lfoGain.connect(gain.gain)

      osc.connect(gain)
      gain.connect(analyser)
      osc.start()
      lfo.start()
      sourcesRef.current.push({ osc, lfo, gain })
    })
    setPlaying(true)
  }

  useEffect(() => () => stop(), [])

  return (
    <div className="flex h-full flex-col items-center justify-center gap-8 p-8">
      <div className="text-center">
        <p className="font-serif text-lg text-app-text">James</p>
        <p className="text-sm text-app-muted">
          {playing ? 'Ouvindo e respondendo…' : 'Toque para ativar a voz'}
        </p>
      </div>

      <div className="relative" style={{ width: 320, height: 320 }}>
        <canvas
          ref={canvasRef}
          style={{ width: 320, height: 320 }}
          className="max-w-full"
          aria-label="Visualização de áudio do James"
        />
        <img
          ref={logoRef}
          src="/james-logo.png"
          alt="Logo do James"
          className="pointer-events-none absolute left-1/2 top-1/2 w-28 select-none object-contain"
          style={{ transform: 'translate(-50%, -50%)' }}
        />
      </div>

      <button
        type="button"
        onClick={playing ? stop : start}
        className="flex items-center gap-2 rounded-full bg-app-blue px-6 py-2.5 text-sm font-medium text-app-blue-foreground transition-transform hover:scale-[1.03] active:scale-95"
      >
        {playing ? <Pause className="size-4" /> : <Play className="size-4" />}
        {playing ? 'Parar' : 'Reproduzir'}
      </button>
    </div>
  )
}
