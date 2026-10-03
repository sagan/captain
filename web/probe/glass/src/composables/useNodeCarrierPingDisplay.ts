// Carrier colours and bars from the MIT three-network theme; Captain supplies counted history.
import { computed, toValue } from 'vue'
import type { MaybeRefOrGetter } from 'vue'
import { snapshot, shows } from '@/captain'
import { carriers, selectPings } from '@/ping-selection'
import type { CarrierKey } from '@/ping-selection'
import type { NodePingHistoryPoint } from '@/ping-summary'
import { useNodePingStats } from '@/composables/useNodePingStats'
import { formatDateTime } from '@/utils/helper'
import { t } from '@/i18n'
interface CarrierPingBar { key: string; className: string; tooltip: string }
const EMPTY_PING_BAR_COUNT = 20

const CARRIER_DOT_CLASSES: Record<CarrierKey, string> = {
  CU: 'bg-rose-500',
  CT: 'bg-blue-500',
  CM: 'bg-emerald-500',
}

function getLatencyToneClass(latency: number): string {
  if (latency <= 60)
    return 'bg-emerald-600/90'
  if (latency <= 100)
    return 'bg-green-400/80'
  if (latency <= 160)
    return 'bg-lime-400/80'
  if (latency <= 200)
    return 'bg-yellow-400/80'
  return 'bg-rose-500/80'
}

function getLossToneClass(loss: number): string {
  if (loss <= 1)
    return 'bg-emerald-600/90'
  if (loss <= 3)
    return 'bg-green-400/90'
  if (loss <= 6)
    return 'bg-lime-400/90'
  if (loss <= 9)
    return 'bg-yellow-400/90'
  return 'bg-rose-500/80'
}

function buildHistoryBars(
  carrierLabel: string,
  carrierKey: CarrierKey,
  history: NodePingHistoryPoint[],
  metric: 'latency' | 'loss',
): CarrierPingBar[] {
  return history.map((point, index) => {
    const value = point[metric]
    const valueText = value === null
      ? 'N/A'
      : metric === 'latency'
        ? `${Math.round(value)} ms`
        : `${value.toFixed(1)}%`

    return {
      key: `${carrierKey}-${metric}-${point.time}-${index}`,
      className: value === null
        ? 'bg-muted-foreground/15'
        : metric === 'latency'
          ? getLatencyToneClass(value)
          : getLossToneClass(value),
      tooltip: `${carrierLabel}\n${formatDateTime(point.time, 'HH:mm:ss')}\n${valueText}`,
    }
  })
}

function buildEmptyBars(carrierKey: CarrierKey, metric: 'latency' | 'loss', tooltip: string): CarrierPingBar[] {
  return Array.from({ length: EMPTY_PING_BAR_COUNT }, (_, index) => ({
    key: `${carrierKey}-${metric}-empty-${index}`,
    className: 'bg-muted-foreground/10',
    tooltip,
  }))
}

export function useNodeCarrierPingDisplay(uuid: MaybeRefOrGetter<string>, enabled: MaybeRefOrGetter<boolean>) {
  const selection = computed(() => selectPings(snapshot.value?.nodes.find(n => String(n.id) === toValue(uuid)), snapshot.value?.carrier_ping ?? false))
  const states = carriers.map(c => useNodePingStats(uuid, {
    hours: 1,
    enabled: () => toValue(enabled) && selection.value.threeNetwork && shows('history') && shows('latency'),
    source: () => selection.value.carriers.find(v => v.key === c.key)?.source ?? null,
  }))
  const carrierDisplays = computed(() => carriers.map((c, i) => {
    const state = states[i]!, label = t(c.label)
    const empty = state.loading.value ? t('加载中') : state.error.value ? t('加载失败') : !shows('history') ? t('未启用记录') : t('无采样数据')
    const latency = state.hasData.value && Number.isFinite(state.avgLatency.value) ? `${Math.round(state.avgLatency.value)} ms` : '—'
    const loss = state.hasData.value ? `${state.avgLoss.value.toFixed(1)}%` : '—'
    return { key: c.key, label, dotClass: CARRIER_DOT_CLASSES[c.key], latencyDisplay: latency, lossDisplay: loss,
      latencyTooltip: `${label}: ${latency}`, lossTooltip: `${label}: ${loss}`,
      latencyBars: state.history.value.length ? buildHistoryBars(label, c.key, state.history.value, 'latency') : buildEmptyBars(c.key, 'latency', empty),
      lossBars: state.history.value.length ? buildHistoryBars(label, c.key, state.history.value, 'loss') : buildEmptyBars(c.key, 'loss', empty),
    }
  }))
  return { carrierDisplays }
}
