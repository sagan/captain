import { useMemo } from 'react'
import { PING_SUMMARY_MAX_COUNT } from '@shared/constants/load'
import { buildEmptyPingBars, buildPingBars, emptyPingTooltip, pingDisplays } from '@shared/composables/pingBars'
import type { NodePingMetric, PingDisplayTexts } from '@shared/composables/pingBars'
import { useApp } from '@/stores/app'
import { useNodePingStats } from './useNodePingStats'

export type { NodePingBar, NodePingMetric } from '@shared/composables/pingBars'

interface UseNodePingDisplayOptions extends PingDisplayTexts {
  enabled?: boolean
}

export function useNodePingDisplay(uuid: string, options: UseNodePingDisplayOptions = {}) {
  const publicSettings = useApp(s => s.publicSettings)

  let pingStatsEnabled = true
  if (options.enabled === false)
    pingStatsEnabled = false
  else if (publicSettings?.record_enabled === false)
    pingStatsEnabled = false
  else
    pingStatsEnabled = publicSettings?.ping_record_preserve_time !== 0

  const preserveTime = publicSettings?.ping_record_preserve_time
  const pingStatsHours = typeof preserveTime === 'number' && preserveTime > 0 ? Math.min(preserveTime, 1) : 1

  const pingStats = useNodePingStats(uuid, {
    hours: pingStatsHours,
    enabled: pingStatsEnabled,
    maxCount: PING_SUMMARY_MAX_COUNT,
  })

  const { history, loading, error, hasData, avgLatency, avgLoss, avgVolatility, source } = pingStats
  const renderBars = (metric: NodePingMetric) => {
    const bars = buildPingBars(history, metric)
    return bars.length
      ? bars
      : buildEmptyPingBars(metric, emptyPingTooltip({ loading, error }, pingStatsEnabled, metric))
  }
  const latencyRenderBars = useMemo(() => renderBars('latency'), [history, loading, error, pingStatsEnabled]) // eslint-disable-line react-hooks/exhaustive-deps
  const lossRenderBars = useMemo(() => renderBars('loss'), [history, loading, error, pingStatsEnabled]) // eslint-disable-line react-hooks/exhaustive-deps

  const { loadingDisplayText, emptyDisplayText, loadingPanelTooltipText, emptyPanelTooltipText } = options
  const displays = useMemo(() => pingDisplays({
    loading,
    hasData,
    avgLatency,
    avgLoss,
    avgVolatility,
    sourceName: source?.name ?? '',
  }, { loadingDisplayText, emptyDisplayText, loadingPanelTooltipText, emptyPanelTooltipText }), [loading, hasData, avgLatency, avgLoss, avgVolatility, source, loadingDisplayText, emptyDisplayText, loadingPanelTooltipText, emptyPanelTooltipText])

  return {
    pingStats,
    pingStatsEnabled,
    pingStatsHours,
    latencyRenderBars,
    lossRenderBars,
    ...displays,
  }
}
