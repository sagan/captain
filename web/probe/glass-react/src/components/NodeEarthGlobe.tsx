// React counterpart of NodeEarthGlobe.vue: loads the earth renderer on demand
// and forwards attributes to it.
import type { ComponentProps } from 'react'
import { asyncComponent } from '@/lib/asyncComponent'
import { useApp } from '@/stores/app'
import type { NodeData } from '@/stores/nodes'

export type NodeEarthProps = Omit<ComponentProps<'div'>, 'ref'> & {
  nodes?: NodeData[]
}

// Captain never sets the theme's earthRenderer setting, so it is always
// 'realistic'; the upstream cobe and tiled renderers are unreachable and not
// ported. Any other value falls back to the realistic globe, as in Vue.
const realistic = asyncComponent<NodeEarthProps>(() => import('@/components/NodeEarthRealisticGlobe'))
const renderers: Record<string, typeof realistic> = { realistic }

export default function NodeEarthGlobe({ nodes, ...attrs }: NodeEarthProps) {
  const earthRenderer = useApp(s => s.earthRenderer)
  const Earth = renderers[earthRenderer] ?? renderers.realistic
  return <Earth {...attrs} nodes={nodes} />
}
