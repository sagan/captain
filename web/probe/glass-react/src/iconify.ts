import { addCollection } from '@iconify/react'
// Same vendored icon set as the Vue build (scripts/glass-icons.mjs); no CDN
// requests. @iconify/react 6 keeps no browser cache, so nothing to disable.
import collections from '../../glass/src/icons.json'

export function setupIconify(): void {
  for (const collection of collections) addCollection(collection as unknown as Parameters<typeof addCollection>[0])
}
