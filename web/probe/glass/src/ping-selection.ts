import type { Node, PublicPingTask } from '../../src/lib/api'
import type { NetworkHistory } from '../../src/lib/network'

export const carriers = [
  { key: 'CU', label: '联通', dotClass: 'bg-rose-500' },
  { key: 'CT', label: '电信', dotClass: 'bg-blue-500' },
  { key: 'CM', label: '移动', dotClass: 'bg-emerald-500' },
] as const
export type CarrierKey = typeof carriers[number]['key']

function carrierKey(name: string): CarrierKey | undefined {
  const value = name.trim().toUpperCase()
  if (['CU', 'CUCC', 'UNICOM', 'CHINA UNICOM', '联通', '聯通', '中国联通', '中國聯通'].includes(value)) return 'CU'
  if (['CT', 'CTCC', 'TELECOM', 'CHINA TELECOM', '电信', '電信', '中国电信', '中國電信'].includes(value)) return 'CT'
  if (['CM', 'CMCC', 'MOBILE', 'CHINA MOBILE', '移动', '移動', '中国移动', '中國移動'].includes(value)) return 'CM'
}

export function selectPings(node: Node | undefined, carrierPing: boolean) {
  // Configured identities survive missing samples. Old servers only offer the
  // last public snapshot; never infer the first task from history arrival order.
  const tasks: PublicPingTask[] = node?.ping_tasks ?? (node?.host?.pings ?? []).map(p => ({ id: p.task_id, name: p.name }))
  const selected = carriers.map(c => ({ ...c, source: carrierPing ? tasks.find(p => p.id === 0 && carrierKey(p.name) === c.key) ?? null : null }))
  const threeNetwork = carrierPing && selected.every(c => c.source !== null)
  const first = tasks.filter(p => p.id > 0).sort((a, b) => a.id - b.id)[0]
    // Custom non-carrier points keep their real names, not fabricated CT/CU/CM labels.
    ?? (carrierPing ? tasks.find(p => p.id === 0) : undefined) ?? null
  return { threeNetwork, carriers: selected, first }
}

export function selectedHistory(history: NetworkHistory, source: PublicPingTask | null): NetworkHistory {
  return { ...history, points: source ? history.points.filter(p => p.task_id === source.id && p.name === source.name) : [] }
}
