// Region clusters for the globe and map; shared by the Vue composable and
// the React hook so both builds group nodes identically.
import type { NodeData } from '@shared/types/node'
import { getCoordByCode, getCountryCodeFromRegion } from '@shared/utils/geoHelper'
import { getRegionDisplayName } from '@shared/utils/regionHelper'

export interface RegionCluster {
  id: string
  code: string
  coord: [number, number]
  label: string
  asn?: string
  org?: string
  servers: number
  onlineServers: number
}

export interface ClusterSummary {
  clusters: RegionCluster[]
  totalServers: number
  onlineServers: number
}

function nodeClusterInfo(node: NodeData): { id: string, code: string, coord: [number, number], label: string, asn?: string, org?: string } | null {
  const countryCode = getCountryCodeFromRegion(node.region)
  if (countryCode) {
    const coord = getCoordByCode(countryCode)
    if (coord)
      return { id: countryCode.toLowerCase(), code: countryCode, coord, label: getRegionDisplayName(node.region) || getRegionDisplayName(countryCode) || '' }
  }

  return null
}

export function summarizeClusters(displayNodes: readonly NodeData[]): ClusterSummary {
  const clustersById = new Map<string, RegionCluster>()
  let onlineServers = 0

  for (const node of displayNodes) {
    if (node.online)
      onlineServers += 1

    const info = nodeClusterInfo(node)
    if (!info)
      continue

    let cluster = clustersById.get(info.id)
    if (!cluster) {
      cluster = { id: info.id, code: info.code, coord: info.coord, label: info.label, asn: info.asn, org: info.org, servers: 0, onlineServers: 0 }
      clustersById.set(info.id, cluster)
    }
    if (!cluster.asn && info.asn)
      cluster.asn = info.asn
    if (!cluster.org && info.org)
      cluster.org = info.org
    cluster.servers += 1

    if (node.online)
      cluster.onlineServers += 1
  }

  return {
    clusters: Array.from(clustersById.values()).sort((a, b) => b.servers - a.servers),
    totalServers: displayNodes.length,
    onlineServers,
  }
}

export function clusterKey(cluster: RegionCluster) {
  return `${cluster.id}:${cluster.coord[0]},${cluster.coord[1]}:${cluster.label}:${cluster.asn ?? ''}:${cluster.org ?? ''}:${cluster.servers}:${cluster.onlineServers}`
}
