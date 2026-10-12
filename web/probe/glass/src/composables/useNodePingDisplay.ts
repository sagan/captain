import type { MaybeRefOrGetter } from 'vue'
import { computed, toValue } from 'vue'
import { useNodePingStats } from '@/composables/useNodePingStats'
import { PING_SUMMARY_MAX_COUNT } from '@shared/constants/load'
import { useAppStore } from '@/stores/app'
import { buildEmptyPingBars, buildPingBars, emptyPingTooltip, pingDisplays } from '@shared/composables/pingBars'
import type { NodePingMetric, PingDisplayTexts } from '@shared/composables/pingBars'

export type { NodePingBar, NodePingMetric } from '@shared/composables/pingBars'

interface UseNodePingDisplayOptions extends PingDisplayTexts {
  enabled?: MaybeRefOrGetter<boolean>
}

export function useNodePingDisplay(
  uuid: MaybeRefOrGetter<string>,
  options: UseNodePingDisplayOptions = {},
) {
  const appStore = useAppStore()

  const pingStatsEnabled = computed(() => {
    if (toValue(options.enabled) === false)
      return false
    if (appStore.publicSettings?.record_enabled === false)
      return false
    return appStore.publicSettings?.ping_record_preserve_time !== 0
  })

  const pingStatsHours = computed(() => {
    const preserveTime = appStore.publicSettings?.ping_record_preserve_time
    if (typeof preserveTime === 'number' && preserveTime > 0)
      return Math.min(preserveTime, 1)
    return 1
  })

  const pingStats = useNodePingStats(uuid, {
    hours: pingStatsHours,
    enabled: pingStatsEnabled,
    maxCount: PING_SUMMARY_MAX_COUNT,
  })

  function renderBars(metric: NodePingMetric) {
    const bars = buildPingBars(pingStats.history.value, metric)
    return bars.length
      ? bars
      : buildEmptyPingBars(metric, emptyPingTooltip({ loading: pingStats.loading.value, error: pingStats.error.value }, pingStatsEnabled.value, metric))
  }

  const latencyRenderBars = computed(() => renderBars('latency'))
  const lossRenderBars = computed(() => renderBars('loss'))

  const displays = computed(() => pingDisplays({
    loading: pingStats.loading.value,
    hasData: pingStats.hasData.value,
    avgLatency: pingStats.avgLatency.value,
    avgLoss: pingStats.avgLoss.value,
    avgVolatility: pingStats.avgVolatility.value,
    sourceName: pingStats.source.value?.name ?? '',
  }, options))

  return {
    pingStats,
    pingStatsEnabled,
    pingStatsHours,
    latencyRenderBars,
    lossRenderBars,
    latencyDisplay: computed(() => displays.value.latencyDisplay),
    lossDisplay: computed(() => displays.value.lossDisplay),
    latencyPanelTooltip: computed(() => displays.value.latencyPanelTooltip),
    lossPanelTooltip: computed(() => displays.value.lossPanelTooltip),
  }
}
