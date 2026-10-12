import type { MaybeRefOrGetter } from 'vue'
import type { PermissionKey } from '@shared/services/auth.service'
import type { NodeProviderMetadata } from '@shared/services/provider.service'
import type { NodeData } from '@/stores/nodes'
import { markRaw, ref, toValue, watch } from 'vue'
import { getNodeProviderFingerprint, getSharedMetadataEntry, releaseSharedMetadataEntry, setProviderMetadataWrapper } from '@shared/composables/providerMetadata'
import { useAppStore } from '@/stores/app'

export type { NodeProviderMetadata } from '@shared/services/provider.service'

setProviderMetadataWrapper(metadata => markRaw(metadata))

interface UseNodeProviderMetadataOptions {
  nodes: MaybeRefOrGetter<NodeData[]>
  customAliases: MaybeRefOrGetter<string>
  enabled?: MaybeRefOrGetter<boolean>
  allowGeoLookup?: MaybeRefOrGetter<boolean>
  geoPermission?: PermissionKey
}

export function useNodeProviderMetadata(options: UseNodeProviderMetadataOptions) {
  const appStore = useAppStore()
  const metadataByUuid = ref<Record<string, NodeProviderMetadata>>({})
  const activeFingerprints = new Map<string, string>()
  let refreshSeq = 0
  let geoPermissionVerified = false

  function getNodeProviderMetadata(node: NodeData): NodeProviderMetadata | null {
    return metadataByUuid.value[node.uuid] ?? null
  }

  watch(
    () => {
      const nodes = toValue(options.nodes)
      const customAliases = toValue(options.customAliases)
      const enabled = options.enabled === undefined ? true : toValue(options.enabled)
      const allowGeoLookup = options.allowGeoLookup === undefined ? true : toValue(options.allowGeoLookup)
      return {
        allowGeoLookup,
        authStatus: appStore.authStatus,
        customAliases,
        enabled,
        fingerprint: nodes.map(node => getNodeProviderFingerprint(node, customAliases, allowGeoLookup)).join(''),
      }
    },
    async ({ allowGeoLookup, authStatus, customAliases, enabled }, _previous, onCleanup) => {
      const seq = ++refreshSeq
      const nodes = toValue(options.nodes)
      const nextMetadata: Record<string, NodeProviderMetadata> = {}
      const unsubscribers: Array<() => void> = []
      let cancelled = false
      activeFingerprints.clear()

      onCleanup(() => {
        cancelled = true
        for (const unsubscribe of unsubscribers)
          unsubscribe()
      })

      if (!enabled) {
        metadataByUuid.value = nextMetadata
        return
      }

      let effectiveAllowGeoLookup = allowGeoLookup
      if (allowGeoLookup && options.geoPermission) {
        if (authStatus !== 'authenticated')
          geoPermissionVerified = false

        if (!geoPermissionVerified) {
          const granted = await appStore.requireLoginPermission(options.geoPermission, { force: false })
          if (cancelled || seq !== refreshSeq)
            return
          geoPermissionVerified = granted
          effectiveAllowGeoLookup = granted
        }
      }

      for (const node of nodes) {
        const { fingerprint, entry } = getSharedMetadataEntry(node, customAliases, effectiveAllowGeoLookup)
        activeFingerprints.set(node.uuid, fingerprint)
        nextMetadata[node.uuid] = entry.metadata

        const subscriber = (metadata: NodeProviderMetadata) => {
          if (seq !== refreshSeq || activeFingerprints.get(node.uuid) !== fingerprint)
            return

          metadataByUuid.value = {
            ...metadataByUuid.value,
            [node.uuid]: metadata,
          }
        }
        entry.subscribers.add(subscriber)
        unsubscribers.push(() => releaseSharedMetadataEntry(entry, subscriber))
      }

      metadataByUuid.value = nextMetadata
    },
    { immediate: true },
  )

  return {
    metadataByUuid,
    getNodeProviderMetadata,
  }
}
