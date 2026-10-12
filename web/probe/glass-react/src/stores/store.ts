import { useRef, useSyncExternalStore } from 'react'

// Minimal external store: the React counterpart of the Pinia stores. State is
// replaced, never mutated, so selectors can compare by reference.
export interface Store<T> {
  getState(): T
  setState(patch: Partial<T> | ((state: T) => Partial<T>)): void
  subscribe(listener: () => void): () => void
}

export function createStore<T extends object>(initial: T): Store<T> {
  let state = initial
  const listeners = new Set<() => void>()
  return {
    getState: () => state,
    setState(patch) {
      const next = typeof patch === 'function' ? patch(state) : patch
      let changed = false
      for (const key of Object.keys(next) as (keyof T)[]) {
        if (!Object.is(state[key], next[key])) {
          changed = true
          break
        }
      }
      if (!changed)
        return
      state = { ...state, ...next }
      for (const listener of listeners) listener()
    },
    subscribe(listener) {
      listeners.add(listener)
      return () => { listeners.delete(listener) }
    },
  }
}

// Subscribes to a derived value; re-renders only when the selection changes
// by `equal` (reference equality by default).
export function useSelector<T, S>(
  subscribe: (listener: () => void) => () => void,
  read: () => T,
  select: (value: T) => S,
  equal: (a: S, b: S) => boolean = Object.is,
): S {
  const cache = useRef<{ source: T, selected: S } | null>(null)
  const getSnapshot = () => {
    const source = read()
    const prev = cache.current
    if (prev && Object.is(prev.source, source))
      return prev.selected
    const selected = select(source)
    if (prev && equal(prev.selected, selected)) {
      cache.current = { source, selected: prev.selected }
      return prev.selected
    }
    cache.current = { source, selected }
    return selected
  }
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot)
}

export function shallowEqual<T>(a: T, b: T): boolean {
  if (Object.is(a, b))
    return true
  if (typeof a !== 'object' || typeof b !== 'object' || !a || !b)
    return false
  const ka = Object.keys(a) as (keyof T)[]
  if (ka.length !== Object.keys(b).length)
    return false
  return ka.every(k => Object.is(a[k], b[k]))
}
