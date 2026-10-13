import { useSyncExternalStore } from 'react'
import type { Box } from '@shared/box'

/** Reads a shared observable box (e.g. the Captain snapshot) as React state. */
export function useBox<T>(source: Box<T>): T {
  return useSyncExternalStore(source.subscribe, () => source.value, () => source.value)
}
