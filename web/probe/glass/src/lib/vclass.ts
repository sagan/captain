// Exact port of Vue's normalizeClass: `class="a" :class="x"` compiles to
// normalizeClass(["a", x]), so vclass('a', x) yields the same string.
export type ClassValue = string | false | null | undefined | 0 | ClassValue[] | Record<string, unknown>

function normalizeClass(value: ClassValue): string {
  let res = ''
  if (typeof value === 'string') {
    res = value
  }
  else if (Array.isArray(value)) {
    for (const item of value) {
      const normalized = normalizeClass(item)
      if (normalized)
        res += `${normalized} `
    }
  }
  else if (value && typeof value === 'object') {
    for (const name in value) {
      if (value[name])
        res += `${name} `
    }
  }
  return res.trim()
}

export function vclass(...parts: ClassValue[]): string {
  return normalizeClass(parts)
}
