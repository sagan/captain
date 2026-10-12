import type { ComponentProps } from 'react'
import { Root as TooltipRoot } from '@radix-ui/react-tooltip'

// reka 与 Radix 的 Root 都不渲染元素，data-slot="tooltip" 在两边都不会出现在 DOM 中。
export function Tooltip(props: ComponentProps<typeof TooltipRoot>) {
  return <TooltipRoot {...props} />
}
