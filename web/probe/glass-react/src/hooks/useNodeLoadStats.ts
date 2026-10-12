import { useEffect, useMemo, useRef, useState } from 'react'
import type { PermissionKey } from '@shared/services/auth.service'
import type { StatusRecord } from '@shared/utils/rpc'
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
import { appStore, useApp } from '@/stores/app'
import { useBox } from './useBox'

export { analyzeDiskPrediction, buildDiskPrediction, loadNodeLoadRecords, loadSharedNodeLoadRecords } from '@shared/composables/loadRecords'
export type { DiskPredictionState, NodeDiskPrediction } from '@shared/composables/loadRecords'

const NO_RECORDS: StatusRecord[] = []

/** React counterpart of the Vue useNodeLoadStats composable. */
export function useNodeLoadStats(
  uuid: string,
  options?: {
    hours?: number
    enabled?: boolean
    diskTotal?: number
    maxCount?: number
    online?: boolean
    permission?: PermissionKey
  },
) {
  const authStatus = useApp(s => s.authStatus)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [recordsAllowed, setRecordsAllowed] = useState(false)
  const permissionVerified = useRef(false)

  const hours = Math.max(1, Math.floor(options?.hours ?? 24))
  const maxCount = normalizeMaxCount(options?.maxCount ?? LOAD_RECORD_MAX_COUNT)
  const cacheKey = getSharedLoadRecordsKey(hours, maxCount)
  const enabled = options?.enabled ?? true
  const online = options?.online ?? false
  const diskTotal = Math.max(0, options?.diskTotal ?? 0)
  const permission = options?.permission

  const data = useBox(getSharedLoadRecordsEntry(cacheKey).data)
  const records = !enabled || !recordsAllowed || !uuid.trim()
    ? NO_RECORDS
    : data?.recordsByClient.get(uuid) ?? NO_RECORDS

  // Shared subscription: keeps the entry's refresh timer alive while used.
  const active = enabled && uuid.trim() !== '' && (!permission || recordsAllowed)
  useEffect(() => {
    if (!active)
      return
    return retainSharedLoadRecordsEntry(hours, maxCount)
  }, [active, hours, maxCount])

  useEffect(() => {
    let cancelled = false
    void (async () => {
      if (!enabled || !uuid.trim()) {
        setRecordsAllowed(false)
        setLoading(false)
        setError(null)
        return
      }

      if (permission) {
        if (authStatus !== 'authenticated')
          permissionVerified.current = false

        if (!permissionVerified.current) {
          const granted = await appStore.requireLoginPermission(permission, { force: false })
          if (cancelled)
            return
          permissionVerified.current = granted
          setRecordsAllowed(granted)
          if (!granted) {
            setLoading(false)
            setError(null)
            return
          }
        }
      }
      else {
        setRecordsAllowed(true)
      }

      const entry = getSharedLoadRecordsEntry(cacheKey)
      const shouldLoadRecords = !entry.data.value
        || Date.now() - entry.lastFetchedAt >= LOAD_RECORD_REFRESH_INTERVAL_MS

      if (!shouldLoadRecords) {
        setLoading(false)
        setError(null)
        return
      }

      setLoading(!entry.data.value)
      setError(null)

      try {
        await loadSharedLoadRecords(entry, hours, maxCount)
        if (!cancelled)
          warnZeroSamplesIfNeeded(entry, uuid, cacheKey, online, maxCount)
        if (!cancelled && entry.fullLoadUnavailable) {
          const nodeFetchedAt = entry.nodeFetchedAt.get(uuid) ?? 0
          const shouldLoadNodeRecords = !entry.data.value?.recordsByClient.has(uuid)
            || Date.now() - nodeFetchedAt >= LOAD_RECORD_REFRESH_INTERVAL_MS
          if (shouldLoadNodeRecords)
            await loadNodeRecordsIntoEntry(entry, uuid, hours, maxCount)
        }
      }
      catch (err) {
        if (!cancelled)
          setError(err instanceof Error ? err.message : '获取负载历史失败')
      }
      finally {
        if (!cancelled)
          setLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [uuid, hours, maxCount, cacheKey, enabled, online, authStatus, diskTotal, permission])

  const diskPredictionState = useMemo(() => analyzeDiskPrediction(records, diskTotal), [records, diskTotal])

  return {
    records,
    loading,
    error,
    diskPrediction: diskPredictionState.prediction,
    diskPredictionState,
    hasData: records.length > 0,
  }
}
