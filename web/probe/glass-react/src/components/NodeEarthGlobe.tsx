// React counterpart of NodeEarthGlobe.vue: picks the earth renderer from the
// theme settings and loads it on demand, forwarding attributes to it.
import type { ComponentProps } from 'react'
import { asyncComponent } from '@/lib/asyncComponent'
import { useApp } from '@/stores/app'
import type { NodeData } from '@/stores/nodes'

export type NodeEarthProps = Omit<ComponentProps<'div'>, 'ref'> & {
  nodes?: NodeData[]
}

const renderers = {
  realistic: asyncComponent<NodeEarthProps>(() => import('@/components/NodeEarthRealisticGlobe')),
  cobe: asyncComponent<NodeEarthProps>(() => import('@/components/NodeEarthCobeGlobe')),
  tiled: asyncComponent<NodeEarthProps>(() => import('@/components/NodeEarthTiledMap')),
}

export default function NodeEarthGlobe({ nodes, ...attrs }: NodeEarthProps) {
  const earthRenderer = useApp(s => s.earthRenderer)
  const Earth = renderers[earthRenderer] ?? renderers.realistic
  return <Earth {...attrs} nodes={nodes} />
}
