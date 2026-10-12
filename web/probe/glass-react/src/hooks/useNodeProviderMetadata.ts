import { useCallback, useEffect, useRef, useState } from 'react'
import type { PermissionKey } from '@shared/services/auth.service'
import type { NodeProviderMetadata } from '@shared/services/provider.service'
import type { NodeData } from '@/stores/nodes'
import { getNodeProviderFingerprint, getSharedMetadataEntry, releaseSharedMetadataEntry } from '@shared/composables/providerMetadata'
import { appStore, useApp } from '@/stores/app'

export type { NodeProviderMetadata } from '@shared/services/provider.service'

interface UseNodeProviderMetadataOptions {
  nodes: NodeData[]
  customAliases: string
  enabled?: boolean
  allowGeoLookup?: boolean
  geoPermission?: PermissionKey
}

/** React counterpart of the Vue useNodeProviderMetadata composable. */
export function useNodeProviderMetadata(options: UseNodeProviderMetadataOptions) {
  const authStatus = useApp(s => s.authStatus)
  // The Vue watch is immediate, so metadata exists on the first render; read
  // the shared cache synchronously here for the same first frame.
  const [metadataByUuid, setMetadataByUuid] = useState<Record<string, NodeProviderMetadata>>(() => {
    const initial: Record<string, NodeProviderMetadata> = {}
    if ((options.enabled ?? true) && !((options.allowGeoLookup ?? true) && options.geoPermission)) {
      for (const node of options.nodes)
        initial[node.uuid] = getSharedMetadataEntry(node, options.customAliases, options.allowGeoLookup ?? true).entry.metadata
    }
    return initial
  })
  const geoPermissionVerified = useRef(false)
  const refreshSeq = useRef(0)
  const nodesRef = useRef(options.nodes)
  nodesRef.current = options.nodes

  const enabled = options.enabled ?? true
  const allowGeoLookup = options.allowGeoLookup ?? true
  const { customAliases, geoPermission } = options
  // Re-run only when a node's provider-relevant fields change, as the Vue watch did.
  const fingerprint = options.nodes.map(node => getNodeProviderFingerprint(node, customAliases, allowGeoLookup)).join('')

  useEffect(() => {
    const seq = ++refreshSeq.current
    const nodes = nodesRef.current
    const nextMetadata: Record<string, NodeProviderMetadata> = {}
    const unsubscribers: Array<() => void> = []
    const activeFingerprints = new Map<string, string>()
    let cancelled = false

    void (async () => {
      if (!enabled) {
        setMetadataByUuid(nextMetadata)
        return
      }

      let effectiveAllowGeoLookup = allowGeoLookup
      if (allowGeoLookup && geoPermission) {
        if (authStatus !== 'authenticated')
          geoPermissionVerified.current = false

        if (!geoPermissionVerified.current) {
          const granted = await appStore.requireLoginPermission(geoPermission, { force: false })
          if (cancelled || seq !== refreshSeq.current)
            return
          geoPermissionVerified.current = granted
          effectiveAllowGeoLookup = granted
        }
      }

      for (const node of nodes) {
        const { fingerprint: nodeFingerprint, entry } = getSharedMetadataEntry(node, customAliases, effectiveAllowGeoLookup)
        activeFingerprints.set(node.uuid, nodeFingerprint)
        nextMetadata[node.uuid] = entry.metadata

        const subscriber = (metadata: NodeProviderMetadata) => {
          if (seq !== refreshSeq.current || activeFingerprints.get(node.uuid) !== nodeFingerprint)
            return
          setMetadataByUuid(current => ({ ...current, [node.uuid]: metadata }))
        }
        entry.subscribers.add(subscriber)
        unsubscribers.push(() => releaseSharedMetadataEntry(entry, subscriber))
      }

      if (!cancelled)
        setMetadataByUuid(nextMetadata)
    })()

    return () => {
      cancelled = true
      for (const unsubscribe of unsubscribers)
        unsubscribe()
    }
  }, [fingerprint, customAliases, enabled, allowGeoLookup, authStatus, geoPermission])

  const getNodeProviderMetadata = useCallback(
    (node: NodeData): NodeProviderMetadata | null => metadataByUuid[node.uuid] ?? null,
    [metadataByUuid],
  )

  return { metadataByUuid, getNodeProviderMetadata }
}
