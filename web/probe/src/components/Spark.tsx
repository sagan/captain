// Tiny canvas sparkline: no chart library on the list page, one <canvas>
// per node, redrawn when the data changes.
import { useEffect, useRef } from 'react'

export function Spark({ values, color = '#22d3ee', height = 32, max }: { values: (number | null)[]; color?: string; height?: number; max?: number }) {
  const ref = useRef<HTMLCanvasElement>(null)
  useEffect(() => {
    const c = ref.current
    if (!c) return
    const dpr = window.devicePixelRatio || 1
    const w = c.clientWidth || 160
    c.width = w * dpr; c.height = height * dpr
    const ctx = c.getContext('2d')!
    ctx.scale(dpr, dpr)
    ctx.clearRect(0, 0, w, height)
    if (values.length < 2) return
    const m = max ?? Math.max(1, ...values.filter((v): v is number => v != null))
    const step = w / (values.length - 1)
    ctx.beginPath()
    let gap = true
    values.forEach((v, i) => {
      if (v == null) { gap = true; return }
      const y = height - (Math.min(v, m) / m) * (height - 2) - 1
      if (gap) ctx.moveTo(i * step, y); else ctx.lineTo(i * step, y)
      gap = false
    })
    ctx.strokeStyle = color; ctx.lineWidth = 1.5; ctx.stroke()

  }, [values, color, height, max])
  return <canvas ref={ref} style={{ width: '100%', height }} />
}
