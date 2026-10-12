// Provider metadata cache shared by the Vue composable and the React hook:
// one geo lookup per node fingerprint, fanned out to every subscriber.
import type { NodeProviderMetadata } from '@shared/services/provider.service'
import type { NodeData } from '@shared/types/node'
import { CACHE_CONFIG } from '@shared/constants/cache'
import { SharedCache } from '@shared/services/cache.service'
import { buildNodeProviderMetadata, getNodeIps, getNodeProviderFingerprint, lookupNodeGeo } from '@shared/services/provider.service'

export type { NodeProviderMetadata }
export { getNodeProviderFingerprint }

export interface SharedCacheEntry {
  metadata: NodeProviderMetadata
  subscribers: Set<(metadata: NodeProviderMetadata) => void>
  promise: Promise<void> | null
}

const sharedMetadataCache = new SharedCache<SharedCacheEntry>({
  maxSize: CACHE_CONFIG.providerMetadata.maxSize,
  ttl: CACHE_CONFIG.providerMetadata.ttl,
  cleanupInterval: CACHE_CONFIG.cleanup.interval,
  canEvict: entry => entry.subscribers.size === 0 && entry.promise === null,
})

// Vue marks metadata raw so it is not made deeply reactive; React needs nothing.
let wrapMetadata: (metadata: NodeProviderMetadata) => NodeProviderMetadata = metadata => metadata
export function setProviderMetadataWrapper(wrap: (metadata: NodeProviderMetadata) => NodeProviderMetadata): void {
  wrapMetadata = wrap
}

function buildRawNodeProviderMetadata(...args: Parameters<typeof buildNodeProviderMetadata>): NodeProviderMetadata {
  return wrapMetadata(buildNodeProviderMetadata(...args))
}

function notifySharedEntry(entry: SharedCacheEntry): void {
  for (const subscriber of entry.subscribers)
    subscriber(entry.metadata)
}

export function releaseSharedMetadataEntry(entry: SharedCacheEntry, subscriber: (metadata: NodeProviderMetadata) => void): void {
  entry.subscribers.delete(subscriber)
  sharedMetadataCache.sweep()
}

export function getSharedMetadataEntry(node: NodeData, customAliases: string, allowGeoLookup: boolean): { fingerprint: string, entry: SharedCacheEntry } {
  const fingerprint = getNodeProviderFingerprint(node, customAliases, allowGeoLookup)
  const cached = sharedMetadataCache.get(fingerprint)
  if (cached)
    return { fingerprint, entry: cached }

  const hasIps = allowGeoLookup && getNodeIps(node).length > 0
  const entry: SharedCacheEntry = {
    metadata: buildRawNodeProviderMetadata(node, customAliases, null, hasIps),
    subscribers: new Set(),
    promise: null,
  }
  sharedMetadataCache.set(fingerprint, entry)

  if (hasIps) {
    entry.promise = lookupNodeGeo(node)
      .then((geo) => {
        entry.metadata = buildRawNodeProviderMetadata(node, customAliases, geo, false)
        notifySharedEntry(entry)
      })
      .catch(() => {
        entry.metadata = buildRawNodeProviderMetadata(node, customAliases, null, false)
        notifySharedEntry(entry)
      })
      .finally(() => {
        entry.promise = null
        sharedMetadataCache.sweep()
      })
  }

  return { fingerprint, entry }
}
