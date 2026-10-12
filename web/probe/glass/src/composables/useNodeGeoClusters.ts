import type { NodeData } from '@/stores/nodes'
import { computed } from 'vue'
import { useNodesStore } from '@/stores/nodes'
import { clusterKey, summarizeClusters } from '@shared/composables/geoClusters'
import type { ClusterSummary, RegionCluster } from '@shared/composables/geoClusters'

export type { RegionCluster }

interface UseNodeGeoClustersOptions {
  nodes?: () => NodeData[] | undefined
}

export function useNodeGeoClusters(options: UseNodeGeoClustersOptions = {}) {
  const nodesStore = useNodesStore()

  const displayNodes = computed(() => options.nodes?.() ?? nodesStore.visibleNodes)
  const clusterSummary = computed<ClusterSummary>(() => summarizeClusters(displayNodes.value))

  const regionClusters = computed<RegionCluster[]>(() => clusterSummary.value.clusters)
  const totalServers = computed(() => clusterSummary.value.totalServers)
  const onlineServers = computed(() => clusterSummary.value.onlineServers)
  const offlineServers = computed(() => totalServers.value - onlineServers.value)

  return {
    displayNodes,
    regionClusters,
    totalServers,
    onlineServers,
    offlineServers,
    clusterKey,
  }
}
