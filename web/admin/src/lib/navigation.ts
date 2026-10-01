import { settingsCatalog, systemAreas } from './settings-catalog'

export type StaffRole = 'admin' | 'operator' | 'support'
export interface NavigationItem { to: string; labelKey: string }
export interface NavigationGroup { id: string; labelKey: string; items: NavigationItem[] }
export const navigation: NavigationGroup[] = [
  { id: 'overview', labelKey: 'nav.dashboard', items: [{ to: '/', labelKey: 'nav.dashboard' }] },
  { id: 'users', labelKey: 'nav.users', items: [
    { to: '/users', labelKey: 'nav.users' }, { to: '/users?tab=groups', labelKey: 'users.groups' }, { to: '/settings/area/users', labelKey: 'workspace.registration' },
  ] },
  { id: 'nodes', labelKey: 'nav.nodes', items: [
    { to: '/nodes', labelKey: 'nav.nodes' }, { to: '/infrastructure', labelKey: 'nav.infrastructure' }, { to: '/domains', labelKey: 'nav.domains' }, { to: '/settings/area/nodes', labelKey: 'workspace.nodeSettings' },
  ] },
  { id: 'subscription', labelKey: 'workspace.subscription', items: [
    { to: '/entries', labelKey: 'nav.entries' }, { to: '/external', labelKey: 'nav.external' }, { to: '/sub-templates', labelKey: 'workspace.templates' }, { to: '/settings/subscription', labelKey: 'workspace.subscriptionSettings' }, { to: '/settings/clients', labelKey: 'clients.title' },
  ] },
  { id: 'monitoring', labelKey: 'nav.monitoring', items: [
    { to: '/monitoring', labelKey: 'nav.monitoring' }, { to: '/speedtest', labelKey: 'nav.speedtest' }, { to: '/monitoring?tab=alerts', labelKey: 'alerts.title' }, { to: '/settings/area/monitoring', labelKey: 'workspace.monitorSettings' },
  ] },
  { id: 'business', labelKey: 'workspace.business', items: [
    { to: '/plans', labelKey: 'nav.plans' }, { to: '/orders', labelKey: 'nav.orders' }, { to: '/business/marketing', labelKey: 'workspace.marketing' }, { to: '/tickets', labelKey: 'nav.tickets' }, { to: '/business/content', labelKey: 'workspace.content' }, { to: '/settings/area/business', labelKey: 'workspace.businessSettings' },
  ] },
]

// Preserve the existing role matrix when moving settings into business modules.
// Hiding navigation is not authorization: the server remains authoritative.
export function canVisit(to: string, role: string) {
  const path = to.split('?')[0]
  if (role === 'admin') return true
  if (role === 'operator') return !['/settings', '/site', '/admins', '/sub-templates', '/infrastructure'].some(p => path === p || path.startsWith(`${p}/`))
  if (path === '/users' && new URLSearchParams(to.split('?')[1]).get('tab') === 'groups') return false
  return ['/', '/users', '/orders', '/tickets', '/account'].includes(path)
}

export function navigationGroup(path: string) {
  const entry = settingsCatalog.find(e => path === `/settings/${e.id}`)
  if (entry) return systemAreas.includes(entry.area) ? 'system' : entry.area
  if (path.startsWith('/settings/area/')) {
    const area = path.split('/')[3]
    return systemAreas.some(value => value === area) ? 'system' : area
  }
  if (path.startsWith('/settings/')) return 'system'
  if (path === '/account') return 'account'
  if (path.startsWith('/business/')) return 'business'
  if (['/coupons', '/gifts', '/withdrawals', '/articles', '/site'].includes(path)) return 'business'
  return navigation.find(g => g.items.some(i => i.to.split('?')[0] === path || (i.to !== '/' && path.startsWith(`${i.to}/`))))?.id ?? 'system'
}
