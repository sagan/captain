import { useMemo } from 'react'
import type { NodeData } from '@/stores/nodes'
import { useNodes } from '@/stores/nodes'
import { clusterKey, summarizeClusters } from '@shared/composables/geoClusters'
import type { RegionCluster } from '@shared/composables/geoClusters'

export type { RegionCluster }

/** React counterpart of the Vue useNodeGeoClusters composable. */
export function useNodeGeoClusters(nodes?: NodeData[]) {
  const visibleNodes = useNodes(s => s.visibleNodes)
  const displayNodes = nodes ?? visibleNodes
  const summary = useMemo(() => summarizeClusters(displayNodes), [displayNodes])
  return {
    displayNodes,
    regionClusters: summary.clusters,
    totalServers: summary.totalServers,
    onlineServers: summary.onlineServers,
    offlineServers: summary.totalServers - summary.onlineServers,
    clusterKey,
  }
}
