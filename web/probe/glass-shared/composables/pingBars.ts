// Pure parts of the ping display composables, shared by the Vue and React
// builds so both colour and label the 20 bars identically.
import { t } from '@shared/i18n'
import type { CarrierKey } from '@shared/ping-selection'
import type { NodePingHistoryPoint } from '@shared/ping-summary'
import { formatDateTime } from '@shared/utils/helper'

export type NodePingMetric = 'latency' | 'loss'

export interface NodePingBar {
  key: string
  className: string
  tooltip: string
}

export const EMPTY_PING_BAR_COUNT = 20

export function getLatencyToneClass(latency: number): string {
  if (latency <= 60)
    return 'bg-signal-1'
  if (latency <= 100)
    return 'bg-signal-2'
  if (latency <= 160)
    return 'bg-signal-3 ping-signal-pattern-2'
  if (latency <= 200)
    return 'bg-signal-4 ping-signal-pattern-3'
  return 'bg-signal-5 ping-signal-pattern-4'
}

export function getLossToneClass(loss: number): string {
  if (loss <= 1)
    return 'bg-signal-1'
  if (loss <= 3)
    return 'bg-signal-2'
  if (loss <= 6)
    return 'bg-signal-3 ping-signal-pattern-2'
  if (loss <= 9)
    return 'bg-signal-4 ping-signal-pattern-3'
  return 'bg-signal-5 ping-signal-pattern-4'
}

export function buildPingBars(points: NodePingHistoryPoint[], metric: NodePingMetric): NodePingBar[] {
  if (!points.length)
    return []

  return points.map((point, index) => {
    const value = point[metric]

    return {
      key: `${point.time}-${index}`,
      className: value === null
        ? 'bg-muted-foreground/15'
        : metric === 'latency'
          ? getLatencyToneClass(value)
          : getLossToneClass(value),
      tooltip: value === null
        ? `${formatDateTime(point.time, 'HH:mm:ss')}\n${t('无采样数据')}`
        : metric === 'latency'
          ? `${formatDateTime(point.time, 'HH:mm:ss')}\n${Math.round(value)} ms`
          : `${formatDateTime(point.time, 'HH:mm:ss')}\n${value.toFixed(1)}%`,
    }
  })
}

export function emptyPingTooltip(state: { loading: boolean, error: string | null }, enabled: boolean, metric: NodePingMetric): string {
  return state.loading
    ? t('加载中')
    : state.error
      ? t('加载失败')
      : !enabled
          ? t('未启用记录')
          : metric === 'latency'
            ? t('无采样数据')
            : t('无采样数据')
}

export function buildEmptyPingBars(metric: NodePingMetric, tooltip: string): NodePingBar[] {
  return Array.from({ length: EMPTY_PING_BAR_COUNT }, (_, index) => ({
    key: `${metric}-empty-${index}`,
    className: 'bg-muted-foreground/10',
    tooltip,
  }))
}

export interface PingDisplayTexts {
  loadingDisplayText?: string
  emptyDisplayText?: string
  loadingPanelTooltipText?: Partial<Record<NodePingMetric, string>>
  emptyPanelTooltipText?: Partial<Record<NodePingMetric, string>>
}

export interface PingStatsValues {
  loading: boolean
  hasData: boolean
  avgLatency: number
  avgLoss: number
  avgVolatility: number
  sourceName: string
}

export function pingDisplays(s: PingStatsValues, options: PingDisplayTexts) {
  const latencyDisplay = s.hasData && Number.isFinite(s.avgLatency)
    ? `${Math.round(s.avgLatency)} ms`
    : s.loading ? (options.loadingDisplayText ?? t('加载中')) : (options.emptyDisplayText ?? '-')
  const lossDisplay = s.hasData
    ? `${s.avgLoss.toFixed(1)}%`
    : s.loading ? (options.loadingDisplayText ?? t('加载中')) : (options.emptyDisplayText ?? '-')
  let latencyPanelTooltip: string
  if (!s.hasData || !Number.isFinite(s.avgLatency)) {
    latencyPanelTooltip = s.loading
      ? (options.loadingPanelTooltipText?.latency ?? '')
      : (options.emptyPanelTooltipText?.latency ?? `${s.sourceName}\n${t('无采样数据')}`)
  }
  else {
    latencyPanelTooltip = `${s.sourceName}\n${t('平均延迟')} ${Math.round(s.avgLatency)} ms`
  }
  let lossPanelTooltip: string
  if (!s.hasData) {
    lossPanelTooltip = s.loading
      ? (options.loadingPanelTooltipText?.loss ?? '')
      : (options.emptyPanelTooltipText?.loss ?? `${s.sourceName}\n${t('无采样数据')}`)
  }
  else {
    const volatility = s.avgVolatility > 0
      ? `，${t('平均波动')} ${s.avgVolatility.toFixed(2)}`
      : ''
    lossPanelTooltip = `${s.sourceName}\n${t('平均丢包')} ${s.avgLoss.toFixed(1)}%${volatility}`
  }
  return { latencyDisplay, lossDisplay, latencyPanelTooltip, lossPanelTooltip }
}

// ---- three-network (carrier) bars

export interface CarrierPingBar { key: string, className: string, tooltip: string }

export const CARRIER_DOT_CLASSES: Record<CarrierKey, string> = {
  CU: 'bg-rose-500',
  CT: 'bg-blue-500',
  CM: 'bg-emerald-500',
}

export function getCarrierLatencyToneClass(latency: number): string {
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

export function getCarrierLossToneClass(loss: number): string {
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

export function buildHistoryBars(
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
          ? getCarrierLatencyToneClass(value)
          : getCarrierLossToneClass(value),
      tooltip: `${carrierLabel}\n${formatDateTime(point.time, 'HH:mm:ss')}\n${valueText}`,
    }
  })
}

export function buildEmptyBars(carrierKey: CarrierKey, metric: 'latency' | 'loss', tooltip: string): CarrierPingBar[] {
  return Array.from({ length: EMPTY_PING_BAR_COUNT }, (_, index) => ({
    key: `${carrierKey}-${metric}-empty-${index}`,
    className: 'bg-muted-foreground/10',
    tooltip,
  }))
}
