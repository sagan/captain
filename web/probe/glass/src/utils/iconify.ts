import { addCollection, disableCache } from '@iconify/vue'
import collections from '@/icons.json'
export async function setupIconify(): Promise<void> {
  disableCache('all')
  for (const collection of collections) addCollection(collection as unknown as Parameters<typeof addCollection>[0])
}
