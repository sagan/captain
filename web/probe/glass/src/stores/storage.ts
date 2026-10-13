// localStorage persistence that reads and writes exactly what the Vue build's
// useStorageAsync does for the same keys, so switching the status page between
// the two bundles keeps each visitor's choices.
//
// VueUse picks the serializer from the default value: strings and null are
// stored raw, arrays/objects as JSON.

export type StorageKind = 'string' | 'any' | 'json'

function storage(): Storage | null {
  try {
    return typeof window === 'undefined' ? null : window.localStorage
  }
  catch {
    return null
  }
}

export function readStored<T>(key: string, fallback: T, kind: StorageKind): T {
  const s = storage()
  let raw: string | null = null
  try {
    raw = s?.getItem(key) ?? null
  }
  catch {
    return fallback
  }
  if (raw === null) {
    // VueUse writes the default back when the key is absent (writeDefaults).
    if (fallback !== null && fallback !== undefined)
      writeStored(key, fallback, kind)
    return fallback
  }
  if (kind === 'json') {
    try {
      return JSON.parse(raw) as T
    }
    catch {
      return fallback
    }
  }
  return raw as T
}

export function writeStored<T>(key: string, value: T, kind: StorageKind): void {
  const s = storage()
  if (!s)
    return
  try {
    if (value === null || value === undefined)
      s.removeItem(key)
    else
      s.setItem(key, kind === 'json' ? JSON.stringify(value) : String(value))
  }
  catch { /* quota or privacy mode: keep the in-memory value */ }
}

// Calls back with the new raw-decoded value when another tab changes the key.
export function onStoredChange<T>(key: string, fallback: T, kind: StorageKind, apply: (value: T) => void): void {
  if (typeof window === 'undefined')
    return
  window.addEventListener('storage', (event) => {
    if (event.key !== key && event.key !== null)
      return
    apply(readStored(key, fallback, kind))
  })
}
