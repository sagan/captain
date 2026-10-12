// Carrier colours and bars from the MIT three-network theme; Captain supplies counted history.
import { computed, toValue } from 'vue'
import type { MaybeRefOrGetter } from 'vue'
import { snapshot, shows } from '@/captain'
import { carriers, selectPings } from '@shared/ping-selection'
import { useNodePingStats } from '@/composables/useNodePingStats'
import { CARRIER_DOT_CLASSES, buildEmptyBars, buildHistoryBars } from '@shared/composables/pingBars'
import { t } from '@shared/i18n'

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
