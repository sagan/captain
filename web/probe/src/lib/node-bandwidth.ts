export interface NodeBandwidth { inbound_mbps: number; outbound_mbps: number; public: boolean }

// Advertised decimal bit rates, independent of live byte/s samples and shaping.
export function nodeBandwidth(b?: NodeBandwidth, locale = 'en'): string {
  if (!b?.public) return ''
  const known = (v: number) => Number.isFinite(v) && v > 0
  if (!known(b.inbound_mbps) && !known(b.outbound_mbps)) return ''
  const rate = (v: number) => !known(v) ? '—' : `${new Intl.NumberFormat(locale, { maximumFractionDigits: 6 }).format(v >= 1000 ? v / 1000 : v)} ${v >= 1000 ? 'Gbps' : 'Mbps'}`
  if (b.inbound_mbps === b.outbound_mbps) return rate(b.inbound_mbps)
  return `RX ${rate(b.inbound_mbps)} · TX ${rate(b.outbound_mbps)}`
}
