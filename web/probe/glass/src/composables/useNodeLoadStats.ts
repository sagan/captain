import type { MaybeRefOrGetter } from 'vue'
import type { PermissionKey } from '@shared/services/auth.service'
import type { StatusRecord } from '@shared/utils/rpc'
import { computed, onScopeDispose, ref, toValue, watch } from 'vue'
import { LOAD_RECORD_MAX_COUNT } from '@shared/constants/load'
import {
  LOAD_RECORD_REFRESH_INTERVAL_MS,
  analyzeDiskPrediction,
  getSharedLoadRecordsEntry,
  getSharedLoadRecordsKey,
  loadNodeRecordsIntoEntry,
  loadSharedLoadRecords,
  normalizeMaxCount,
  retainSharedLoadRecordsEntry,
  warnZeroSamplesIfNeeded,
} from '@shared/composables/loadRecords'
import { useAppStore } from '@/stores/app'

export { analyzeDiskPrediction, buildDiskPrediction, loadNodeLoadRecords, loadSharedNodeLoadRecords } from '@shared/composables/loadRecords'
export type { DiskPredictionState, NodeDiskPrediction } from '@shared/composables/loadRecords'

export function useNodeLoadStats(
  uuid: MaybeRefOrGetter<string>,
  options?: {
    hours?: MaybeRefOrGetter<number>
    enabled?: MaybeRefOrGetter<boolean>
    diskTotal?: MaybeRefOrGetter<number>
    maxCount?: MaybeRefOrGetter<number | undefined>
    online?: MaybeRefOrGetter<boolean>
    permission?: PermissionKey
  },
) {
  const appStore = useAppStore()
  const loading = ref(false)
  const error = ref<string | null>(null)
  const recordsAllowed = ref(false)

  const resolved = computed(() => {
    const hours = Math.max(1, Math.floor(toValue(options?.hours) ?? 24))
    const maxCount = normalizeMaxCount(toValue(options?.maxCount) ?? LOAD_RECORD_MAX_COUNT)
    return {
      uuid: toValue(uuid),
      hours,
      maxCount,
      cacheKey: getSharedLoadRecordsKey(hours, maxCount),
      enabled: toValue(options?.enabled) ?? true,
      online: toValue(options?.online) ?? false,
      authStatus: appStore.authStatus,
      diskTotal: Math.max(0, toValue(options?.diskTotal) ?? 0),
    }
  })

  let activeCacheKey: string | null = null
  let releaseSharedRecords: (() => void) | null = null
  let permissionVerified = false

  function syncSharedRecordsSubscription(hours: number | null, maxCount?: number): void {
    const cacheKey = hours === null ? null : getSharedLoadRecordsKey(hours, maxCount)
    if (activeCacheKey === cacheKey)
      return

    releaseSharedRecords?.()
    releaseSharedRecords = null
    activeCacheKey = null

    if (hours === null)
      return

    releaseSharedRecords = retainSharedLoadRecordsEntry(hours, maxCount)
    activeCacheKey = cacheKey
  }

  onScopeDispose(() => {
    syncSharedRecordsSubscription(null)
  })

  // The shared entry lives outside Vue; a version ref makes `records` re-read it.
  const dataVersion = ref(0)
  let unwatchData: (() => void) | null = null
  watch(() => resolved.value.cacheKey, (cacheKey) => {
    unwatchData?.()
    unwatchData = getSharedLoadRecordsEntry(cacheKey).data.subscribe(() => { dataVersion.value++ })
  }, { immediate: true })
  onScopeDispose(() => {
    unwatchData?.()
    unwatchData = null
  })

  const records = computed<StatusRecord[]>(() => {
    void dataVersion.value
    const { uuid: nodeUuid, cacheKey, enabled } = resolved.value
    if (!enabled || !recordsAllowed.value || !nodeUuid.trim())
      return []

    const entry = getSharedLoadRecordsEntry(cacheKey)
    return entry.data.value?.recordsByClient.get(nodeUuid) ?? []
  })

  const diskPredictionState = computed(() => analyzeDiskPrediction(records.value, resolved.value.diskTotal))
  const diskPrediction = computed(() => diskPredictionState.value.prediction)

  watch(
    resolved,
    async (next, _previous, onCleanup) => {
      let cancelled = false
      onCleanup(() => {
        cancelled = true
      })

      const { uuid: nodeUuid, hours, maxCount, cacheKey, enabled, online, authStatus } = next
      if (!enabled || !nodeUuid.trim()) {
        recordsAllowed.value = false
        syncSharedRecordsSubscription(null)
        loading.value = false
        error.value = null
        return
      }

      if (options?.permission) {
        if (authStatus !== 'authenticated')
          permissionVerified = false

        if (!permissionVerified) {
          const granted = await appStore.requireLoginPermission(options.permission, { force: false })
          if (cancelled)
            return
          permissionVerified = granted
          recordsAllowed.value = granted
          if (!granted) {
            syncSharedRecordsSubscription(null)
            loading.value = false
            error.value = null
            return
          }
        }
      }
      else {
        recordsAllowed.value = true
      }

      syncSharedRecordsSubscription(hours, maxCount)
      const entry = getSharedLoadRecordsEntry(cacheKey)
      const shouldLoadRecords = !entry.data.value
        || Date.now() - entry.lastFetchedAt >= LOAD_RECORD_REFRESH_INTERVAL_MS

      if (!shouldLoadRecords) {
        loading.value = false
        error.value = null
        return
      }

      loading.value = !entry.data.value
      error.value = null

      try {
        await loadSharedLoadRecords(entry, hours, maxCount)
        if (!cancelled)
          warnZeroSamplesIfNeeded(entry, nodeUuid, cacheKey, online, maxCount)
        if (!cancelled && entry.fullLoadUnavailable) {
          const nodeFetchedAt = entry.nodeFetchedAt.get(nodeUuid) ?? 0
          const shouldLoadNodeRecords = !entry.data.value?.recordsByClient.has(nodeUuid)
            || Date.now() - nodeFetchedAt >= LOAD_RECORD_REFRESH_INTERVAL_MS
          if (shouldLoadNodeRecords)
            await loadNodeRecordsIntoEntry(entry, nodeUuid, hours, maxCount)
        }
      }
      catch (err) {
        if (!cancelled)
          error.value = err instanceof Error ? err.message : '获取负载历史失败'
      }
      finally {
        if (!cancelled)
          loading.value = false
      }
    },
    { immediate: true },
  )

  return {
    records,
    loading,
    error,
    diskPrediction,
    diskPredictionState,
    hasData: computed(() => records.value.length > 0),
  }
}
