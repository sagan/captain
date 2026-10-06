import { runNodeJob } from './api'
import type { RealityResult } from '../components/RealityScan'

// Aborting stops polling; an already queued read-only scan may still finish
// on the node. Its result must not update a closed or edited form.
export async function scanRealityViaNode(nodeID: number, hosts: string[], signal?: AbortSignal): Promise<RealityResult[]> {
  return (await runNodeJob<RealityResult[]>(nodeID, 'reality_scan', { hosts }, 150_000, signal)) ?? []
}
