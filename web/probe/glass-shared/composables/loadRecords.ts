// Shared load-history cache behind the useNodeLoadStats composable/hook.
// Entries hold observable boxes so the Vue and React builds read the same
// fetches, refresh timers and aborts.
import type { StatusRecord } from '@shared/utils/rpc'
import { box } from '@shared/box'
import type { Box } from '@shared/box'
import { LOAD_CONFIG, LOAD_RECORD_MAX_COUNT } from '@shared/constants/load'
import { abortLoadRecords, abortNodeLoadRecords, buildRecordsByClient, loadLoadRecords, loadNodeLoadRecords } from '@shared/services/history.service'
import { analyzeDiskPrediction, buildDiskPrediction } from '@shared/services/prediction.service'

export { analyzeDiskPrediction, buildDiskPrediction, loadNodeLoadRecords }
export type { DiskPredictionState, NodeDiskPrediction } from '@shared/services/prediction.service'

interface SharedLoadRecordsState {
  recordsByClient: Map<string, StatusRecord[]>
}

export interface SharedLoadRecordsEntry {
  data: Box<SharedLoadRecordsState | null>
  loading: Box<boolean>
  error: Box<string | null>
  promise: Promise<void> | null
  nodePromises: Map<string, Promise<StatusRecord[]>>
  nodeFetchedAt: Map<string, number>
  refreshTimer: ReturnType<typeof setInterval> | null
  subscribers: number
  lastFetchedAt: number
  fullLoadUnavailable: boolean
  fullLoadRetryAt: number
}

export const LOAD_RECORD_REFRESH_INTERVAL_MS = LOAD_CONFIG.records.refreshInterval
const sharedLoadRecordsCache = new Map<string, SharedLoadRecordsEntry>()
const zeroSampleWarningKeys = new Set<string>()

export function normalizeMaxCount(maxCount: number | null | undefined): number | undefined {
  if (typeof maxCount !== 'number' || !Number.isFinite(maxCount) || maxCount <= 0)
    return undefined
  return Math.floor(maxCount)
}

export function getSharedLoadRecordsKey(hours: number, maxCount?: number): string {
  return `${hours}:${maxCount ?? 'all'}`
}

function isAbortError(error: unknown): boolean {
  return error instanceof Error && error.name === 'AbortError'
}

function createSharedLoadRecordsEntry(): SharedLoadRecordsEntry {
  return {
    data: box<SharedLoadRecordsState | null>(null),
    loading: box(false),
    error: box<string | null>(null),
    promise: null,
    nodePromises: new Map<string, Promise<StatusRecord[]>>(),
    nodeFetchedAt: new Map<string, number>(),
    refreshTimer: null,
    subscribers: 0,
    lastFetchedAt: 0,
    fullLoadUnavailable: false,
    fullLoadRetryAt: 0,
  }
}

export function getSharedLoadRecordsEntry(key: string): SharedLoadRecordsEntry {
  const cachedEntry = sharedLoadRecordsCache.get(key)
  if (cachedEntry)
    return cachedEntry

  const nextEntry = createSharedLoadRecordsEntry()
  sharedLoadRecordsCache.set(key, nextEntry)
  return nextEntry
}

function setNodeRecords(entry: SharedLoadRecordsEntry, uuid: string, records: StatusRecord[]): void {
  const current = entry.data.value?.recordsByClient ?? new Map<string, StatusRecord[]>()
  const next = new Map(current)
  next.set(uuid, records)
  entry.data.value = { recordsByClient: next }
  entry.nodeFetchedAt.set(uuid, Date.now())
}

export function warnZeroSamplesIfNeeded(entry: SharedLoadRecordsEntry, uuid: string, cacheKey: string, online: boolean, maxCount?: number): void {
  if (!import.meta.env.DEV || !online || maxCount !== LOAD_RECORD_MAX_COUNT || entry.fullLoadUnavailable)
    return

  const records = entry.data.value?.recordsByClient.get(uuid)
  if (records && records.length > 0)
    return

  const warningKey = `${cacheKey}:${uuid}`
  if (zeroSampleWarningKeys.has(warningKey))
    return

  zeroSampleWarningKeys.add(warningKey)
  console.warn(`[Komari Glassmorphism] capped shared load history returned zero samples for online node ${uuid}; maxCount=${LOAD_RECORD_MAX_COUNT}. This may indicate cap starvation or missing backend history.`)
}

export async function loadNodeRecordsIntoEntry(entry: SharedLoadRecordsEntry, uuid: string, hours: number, maxCount?: number): Promise<StatusRecord[]> {
  const existingPromise = entry.nodePromises.get(uuid)
  if (existingPromise)
    return existingPromise

  const promise = loadNodeLoadRecords(uuid, hours, maxCount)
    .then((records) => {
      setNodeRecords(entry, uuid, records)
      return records
    })
    .catch((error) => {
      if (entry.nodePromises.get(uuid) === promise)
        entry.nodePromises.delete(uuid)
      throw error
    })
    .finally(() => {
      if (entry.nodePromises.get(uuid) === promise)
        entry.nodePromises.delete(uuid)
    })

  entry.nodePromises.set(uuid, promise)
  return promise
}

export async function loadSharedLoadRecords(entry: SharedLoadRecordsEntry, hours: number, maxCount?: number): Promise<void> {
  const now = Date.now()
  if (entry.fullLoadUnavailable && now < entry.fullLoadRetryAt)
    return
  if (entry.promise)
    return entry.promise

  entry.loading.value = true
  entry.error.value = null

  let promise: Promise<void> | null = null
  promise = (async () => {
    try {
      const records = await loadLoadRecords(undefined, hours, maxCount)
      entry.data.value = {
        recordsByClient: buildRecordsByClient(records),
      }
      entry.lastFetchedAt = Date.now()
      entry.fullLoadUnavailable = false
      entry.fullLoadRetryAt = 0
    }
    catch (err) {
      if (isAbortError(err))
        throw err

      entry.fullLoadUnavailable = true
      entry.fullLoadRetryAt = Date.now() + LOAD_RECORD_REFRESH_INTERVAL_MS
      entry.error.value = err instanceof Error ? err.message : '获取负载历史失败'
    }
    finally {
      entry.loading.value = false
      if (entry.promise === promise)
        entry.promise = null
    }
  })()

  entry.promise = promise

  return promise
}

function startSharedLoadRecordsRefresh(entry: SharedLoadRecordsEntry, hours: number, maxCount?: number): void {
  if (entry.refreshTimer)
    return

  entry.refreshTimer = setInterval(() => {
    void loadSharedLoadRecords(entry, hours, maxCount).catch(() => {})
  }, LOAD_RECORD_REFRESH_INTERVAL_MS)
}

function stopSharedLoadRecordsRefresh(entry: SharedLoadRecordsEntry): void {
  if (!entry.refreshTimer)
    return

  clearInterval(entry.refreshTimer)
  entry.refreshTimer = null
}

function abortSharedLoadRecordsEntry(entry: SharedLoadRecordsEntry, hours: number, maxCount?: number): void {
  abortLoadRecords(undefined, hours, maxCount)
  for (const uuid of entry.nodePromises.keys())
    abortNodeLoadRecords(uuid, hours, maxCount)
}

export function retainSharedLoadRecordsEntry(hours: number, maxCount?: number): () => void {
  const key = getSharedLoadRecordsKey(hours, maxCount)
  const entry = getSharedLoadRecordsEntry(key)
  entry.subscribers += 1
  startSharedLoadRecordsRefresh(entry, hours, maxCount)

  let released = false
  return () => {
    if (released)
      return

    released = true
    entry.subscribers = Math.max(0, entry.subscribers - 1)
    if (entry.subscribers === 0) {
      stopSharedLoadRecordsRefresh(entry)
      abortSharedLoadRecordsEntry(entry, hours, maxCount)
    }
  }
}

export async function loadSharedNodeLoadRecords(hours: number, maxCount: number | undefined = LOAD_RECORD_MAX_COUNT): Promise<Map<string, StatusRecord[]>> {
  const safeHours = Math.max(1, Math.floor(hours))
  const safeMaxCount = normalizeMaxCount(maxCount)
  const entry = getSharedLoadRecordsEntry(getSharedLoadRecordsKey(safeHours, safeMaxCount))
  const shouldLoadRecords = !entry.data.value
    || Date.now() - entry.lastFetchedAt >= LOAD_RECORD_REFRESH_INTERVAL_MS

  if (shouldLoadRecords)
    await loadSharedLoadRecords(entry, safeHours, safeMaxCount)

  return entry.data.value?.recordsByClient ?? new Map<string, StatusRecord[]>()
}
