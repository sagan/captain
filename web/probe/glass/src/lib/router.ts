// Minimal hash router matching the Vue build's routes and URL format:
//   #/              home
//   #/instance/:id  instance-detail
import { createContext, useContext, useEffect, useRef, useSyncExternalStore } from 'react'

export type RouteName = 'home' | 'instance-detail'

export interface Route {
  name: RouteName | undefined
  path: string
  params: { id?: string }
}

function parse(hash: string): Route {
  const path = hash.replace(/^#/, '') || '/'
  if (path === '/')
    return { name: 'home', path, params: {} }
  const m = /^\/instance\/([^/?#]+)$/.exec(path)
  if (m)
    return { name: 'instance-detail', path, params: { id: decodeURIComponent(m[1]!) } }
  return { name: undefined, path, params: {} }
}

let current = parse(typeof location === 'undefined' ? '' : location.hash)
const listeners = new Set<() => void>()

function sync(): void {
  const next = parse(location.hash)
  if (next.path === current.path)
    return
  current = next
  for (const l of listeners) l()
}

if (typeof window !== 'undefined') {
  // vue-router's hash history normalises a missing hash to "#/" on start.
  if (!location.hash)
    history.replaceState(history.state, '', '#/')
  window.addEventListener('hashchange', sync)
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener)
  return () => { listeners.delete(listener) }
}

type Target = string | { name: 'home' } | { name: 'instance-detail', params: { id: string } }

function toPath(target: Target): string {
  if (typeof target === 'string')
    return target
  if (target.name === 'home')
    return '/'
  return `/instance/${encodeURIComponent(target.params.id)}`
}

export const router = {
  get currentRoute(): Route {
    return current
  },
  push(target: Target): Promise<void> {
    const path = toPath(target)
    if (path !== current.path) {
      // vue-router's hash history uses pushState; hashchange does not fire for it.
      history.pushState(history.state, '', `#${path}`)
      sync()
    }
    return Promise.resolve()
  },
}

if (typeof window !== 'undefined')
  window.addEventListener('popstate', sync)

export function useRoute(): Route {
  return useSyncExternalStore(subscribe, () => current, () => current)
}

export function useRouter() {
  return router
}

/** Whether the enclosing kept-alive view is active (Vue's KeepAlive). */
export const ActiveContext = createContext(true)

/** Runs callbacks like Vue's onActivated/onDeactivated; activation also fires on mount. */
export function useActivation(onActivated?: () => void, onDeactivated?: () => void): boolean {
  const active = useContext(ActiveContext)
  const cbs = useRef({ onActivated, onDeactivated })
  cbs.current = { onActivated, onDeactivated }
  useEffect(() => {
    if (active)
      cbs.current.onActivated?.()
    else
      cbs.current.onDeactivated?.()
  }, [active])
  return active
}
