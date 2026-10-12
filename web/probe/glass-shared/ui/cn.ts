// Shared by the Vue and React primitives: merges class lists with
// Tailwind-aware de-duplication, exactly as the original theme does.
import type { ClassValue } from 'clsx'
import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
