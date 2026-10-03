import type { NetworkHistory } from '../../src/lib/network'
export interface NodePingHistoryPoint { time: string; latency: number | null; loss: number | null }
export interface NodePingStatsState {
  avgLatency: number; avgLoss: number; avgVolatility: number; history: NodePingHistoryPoint[]; hasData: boolean
}
export function summarizePings(history: NetworkHistory, hours: number): NodePingStatsState {
  const from = history.to - hours * 3600, width = hours * 3600 / 20
  const buckets = Array.from({length: 20}, () => ({ total: 0, lost: 0, sum: 0 }))
  for (const p of history.points ?? []) {
    if (p.ts < from || p.ts > history.to || p.n <= 0) continue
    const b = buckets[Math.min(19, Math.floor((p.ts - from) / width))]!
    b.total += p.n; b.lost += p.lost; b.sum += Math.max(0, p.n - p.lost) * (p.n > p.lost ? p.avg_ms : 0)
  }
  const total = buckets.reduce((n, b) => n + b.total, 0), lost = buckets.reduce((n, b) => n + b.lost, 0)
  const sum = buckets.reduce((n, b) => n + b.sum, 0)
  return {
    hasData: total > 0, avgLatency: total > lost ? sum / (total - lost) : Number.NaN,
    avgLoss: total ? lost / total * 100 : Number.NaN, avgVolatility: 0,
    history: total ? buckets.map((b, i) => ({ time: new Date((from + i * width) * 1000).toISOString(),
      latency: b.total > b.lost ? b.sum / (b.total - b.lost) : null, loss: b.total ? b.lost / b.total * 100 : null })) : [],
  }
}
