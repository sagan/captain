// A minimal observable cell shared by the Vue and React builds of the
// status page. It reads and writes like a Vue ref (`.value`) so the
// framework-neutral code and both component trees use one spelling, and
// it notifies subscribers so each framework can mirror it into its own
// reactivity (shallowRef + watch on the Vue side, useSyncExternalStore on
// the React side) without the shared code importing either.
export interface Box<T> {
  value: T
  subscribe(listener: () => void): () => void
}

export function box<T>(initial: T): Box<T> {
  let current = initial
  const listeners = new Set<() => void>()
  return {
    get value() { return current },
    set value(next: T) {
      if (Object.is(next, current)) return
      current = next
      for (const l of listeners) l()
    },
    subscribe(listener) {
      listeners.add(listener)
      return () => { listeners.delete(listener) }
    },
  }
}
