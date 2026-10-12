// Carrier colours and bars from the MIT three-network theme; Captain supplies counted history.
import { useMemo } from 'react'
import { shows, snapshot } from '@shared/captain'
import { CARRIER_DOT_CLASSES, buildEmptyBars, buildHistoryBars } from '@shared/composables/pingBars'
import { t } from '@shared/i18n'
import { carriers, selectPings } from '@shared/ping-selection'
import { useBox } from './useBox'
import { useNodePingStats } from './useNodePingStats'

if (carriers.length !== 3)
  throw new Error('useNodeCarrierPingDisplay expects exactly three carriers')

export function useNodeCarrierPingDisplay(uuid: string, enabled: boolean) {
  const snap = useBox(snapshot)
  const selection = useMemo(
    () => selectPings(snap?.nodes.find(n => String(n.id) === uuid), snap?.carrier_ping ?? false),
    [snap, uuid],
  )
  const on = enabled && selection.threeNetwork && shows('history') && shows('latency')
  const sourceOf = (i: number) => selection.carriers.find(v => v.key === carriers[i]!.key)?.source ?? null
  // `carriers` is a module constant of length three, so the hook order is fixed.
  const states = [
    useNodePingStats(uuid, { hours: 1, enabled: on, source: sourceOf(0) }),
    useNodePingStats(uuid, { hours: 1, enabled: on, source: sourceOf(1) }),
    useNodePingStats(uuid, { hours: 1, enabled: on, source: sourceOf(2) }),
  ]
  const historyShown = shows('history')
  const carrierDisplays = useMemo(() => carriers.map((c, i) => {
    const state = states[i]!, label = t(c.label)
    const empty = state.loading ? t('加载中') : state.error ? t('加载失败') : !historyShown ? t('未启用记录') : t('无采样数据')
    const latency = state.hasData && Number.isFinite(state.avgLatency) ? `${Math.round(state.avgLatency)} ms` : '—'
    const loss = state.hasData ? `${state.avgLoss.toFixed(1)}%` : '—'
    return { key: c.key, label, dotClass: CARRIER_DOT_CLASSES[c.key], latencyDisplay: latency, lossDisplay: loss,
      latencyTooltip: `${label}: ${latency}`, lossTooltip: `${label}: ${loss}`,
      latencyBars: state.history.length ? buildHistoryBars(label, c.key, state.history, 'latency') : buildEmptyBars(c.key, 'latency', empty),
      lossBars: state.history.length ? buildHistoryBars(label, c.key, state.history, 'loss') : buildEmptyBars(c.key, 'loss', empty),
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }), [historyShown, ...states.flatMap(s => [s.loading, s.error, s.hasData, s.avgLatency, s.avgLoss, s.history])])
  return { carrierDisplays }
}
