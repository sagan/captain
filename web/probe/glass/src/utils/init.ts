import { useAppStore } from '@/stores/app'
import { useNodesStore } from '@/stores/nodes'
import { snapshot, readJSON, clientFor, statusFor, shows, clearHistoryCache } from '@/captain'
import type { Snapshot } from '../../../src/lib/api'
import type { PublicSettings } from './api'

let timer: ReturnType<typeof setTimeout> | undefined
let controller: AbortController | undefined
let stopped = false
const system = window.matchMedia('(prefers-color-scheme: dark)')

async function refresh(): Promise<boolean> {
  const app = useAppStore(), nodes = useNodesStore()
  controller?.abort()
  controller = new AbortController()
  try {
    const s = await readJSON<Snapshot>('/api/probe', controller.signal)
    if (stopped) return false
    if (snapshot.value && (s.appearance?.preset !== 'glass' || JSON.stringify(snapshot.value.public_sections) !== JSON.stringify(s.public_sections))) {
      // Clear component-local chart caches when publication permissions change.
      window.location.reload()
      return false
    }
    snapshot.value = s
    const scheme = s.appearance_scheme ?? s.appearance?.scheme ?? 'dark'
    document.title = s.title
    app.publicSettings = {
      sitename: s.title, description: '', allow_cors: false, custom_body: '', custom_head: '', disable_password_login: true, oauth_enable: false, oauth_provider: '', theme: 'glass', record_enabled: shows('history'), record_preserve_time: 720,
      ping_record_preserve_time: shows('latency') && shows('history') ? 720 : 0,
      visitor_audit_enabled: false, private_site: false,
      theme_settings: {
        themeMode: scheme === 'auto' ? (system.matches ? 'dark' : 'light') : scheme,
        dataUpdateInterval: s.beat_seconds, visitorInfoEnabled: false, hideEarth: !s.show_globe || !shows('system'),
        defaultViewMode: s.layout === 'compact' ? 'list' : 'card',
        hidePriceWhenLoggedOut: true,
        // No billing/currency inference from Captain's free-form price strings.
        generalCardKeys: [...(shows('memory') ? ['memory'] : []), ...(shows('disk') ? ['disk'] : []), ...(shows('system') ? ['remainingValue'] : ['onlineNodes']), ...(shows('network') ? ['totalTraffic', 'uploadSpeed', 'downloadSpeed'] : [])].join(','),
      },
    } as PublicSettings
    nodes.updateNodeClients(Object.fromEntries(s.nodes.map((n, i) => [String(n.id), clientFor(n, i)])))
    nodes.updateNodeStatuses(Object.fromEntries(s.nodes.map(n => [String(n.id), statusFor(n)])))
    // Exact group labels; commas in Captain group names are not separators.
    for (const n of nodes.nodes) {
      (n as typeof n & { captainTraffic: number }).captainTraffic = shows('traffic') ? s.nodes.find(v => String(v.id) === n.uuid)!.traffic.used : Number.NaN
      n.groups = s.nodes.find(v => String(v.id) === n.uuid)?.group ? [s.nodes.find(v => String(v.id) === n.uuid)!.group!] : []
    }
    app.connectionError = false
    return true
  } catch (error) {
    if (!stopped && !(error instanceof DOMException && error.name === 'AbortError')) {
      app.connectionError = true
      if (error instanceof Error && ['401', '403', '404'].includes(error.message)) {
        snapshot.value = null
        clearHistoryCache()
        nodes.nodes = []
      }
    }
    return false
  } finally {
    app.loading = false
    if (!stopped) timer = setTimeout(() => void refresh(), Math.max(5, snapshot.value?.beat_seconds ?? 15) * 1000)
  }
}
function systemChanged() { if ((snapshot.value?.appearance_scheme ?? snapshot.value?.appearance?.scheme) === 'auto') void retryInitApp() }
export async function initApp() { stopped = false; system.addEventListener('change', systemChanged); return refresh() }
export async function retryInitApp() { clearTimeout(timer); return refresh() }
export function destroyInitManager() { stopped = true; clearTimeout(timer); controller?.abort(); system.removeEventListener('change', systemChanged); clearHistoryCache() }
