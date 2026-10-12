// Vue-side view of the shared Captain adapter. The shared module keeps
// the snapshot in a framework-neutral box; this mirrors it into a
// shallowRef so templates and computeds that read `snapshot.value` keep
// re-rendering when the snapshot changes. Everything else is re-exported
// untouched, so components import one module either way.
import { shallowRef, watch } from 'vue'
import { snapshot as sharedSnapshot } from '@shared/captain'
import type { Snapshot } from '@shared/captain'

export * from '@shared/captain'

export const snapshot = shallowRef<Snapshot | null>(sharedSnapshot.value)
sharedSnapshot.subscribe(() => { snapshot.value = sharedSnapshot.value })
watch(snapshot, (v) => { if (!Object.is(v, sharedSnapshot.value)) sharedSnapshot.value = v })
