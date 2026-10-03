// Captain stores counted time buckets, not individual Komari ping events.
// Preserve the original 20-bar presentation without fabricating raw attempts.
import { computed, onScopeDispose, ref, shallowRef, toValue, watch } from 'vue'
import type { MaybeRefOrGetter } from 'vue'
import { historyFor, snapshot } from '@/captain'
import type { PublicPingTask } from '../../../src/lib/api'
import { selectPings, selectedHistory } from '@/ping-selection'
import type { NetworkHistory } from '../../../src/lib/network'
import { summarizePings } from '@/ping-summary'
import type { NodePingStatsState } from '@/ping-summary'
export function useNodePingStats(uuid: MaybeRefOrGetter<string>, options?: { hours?: MaybeRefOrGetter<number>; enabled?: MaybeRefOrGetter<boolean>; maxCount?: MaybeRefOrGetter<number | undefined>; source?: MaybeRefOrGetter<PublicPingTask | null> }) {
  const loading = ref(false), error = ref<string | null>(null), state = shallowRef<NodePingStatsState | null>(null)
  let epoch = 0, timer: ReturnType<typeof setTimeout> | undefined
  const source = computed(() => options?.source !== undefined ? toValue(options.source) : selectPings(snapshot.value?.nodes.find(n => String(n.id) === toValue(uuid)), snapshot.value?.carrier_ping ?? false).first)
  // Watching a primitive identity prevents each snapshot poll restarting charts.
  const resolved = computed(() => JSON.stringify({ id: toValue(uuid), hours: toValue(options?.hours) ?? 1, enabled: toValue(options?.enabled) !== false, source: source.value }))
  async function refresh() {
    const current = ++epoch, {id, hours, enabled, source: selected} = JSON.parse(resolved.value) as {id: string; hours: number; enabled: boolean; source: PublicPingTask | null}
    clearTimeout(timer)
    if (!enabled || !selected) { state.value = null; loading.value = false; error.value = null; return }
    loading.value = true
    try {
      const history = await historyFor<NetworkHistory>(id, 'pings', hours)
      if (current === epoch) { state.value = summarizePings(selectedHistory(history, selected), hours); error.value = null }
    } catch (e) { if (current === epoch) { error.value = String(e); state.value = null } }
    finally { if (current === epoch) { loading.value = false; timer = setTimeout(refresh, 60_000) } }
  }
  watch(resolved, () => { state.value = null; void refresh() }, {immediate: true})
  onScopeDispose(() => { epoch++; clearTimeout(timer) })
  return { loading, error, refresh, source, history: computed(() => state.value?.history ?? []), hasData: computed(() => state.value?.hasData ?? false),
    avgLatency: computed(() => state.value?.avgLatency ?? Number.NaN), avgLoss: computed(() => state.value?.avgLoss ?? Number.NaN), avgVolatility: computed(() => 0) }
}
