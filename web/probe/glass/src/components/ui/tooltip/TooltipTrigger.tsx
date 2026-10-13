import type { ComponentProps } from 'react'
import { Trigger } from '@radix-ui/react-tooltip'

export function TooltipTrigger(props: ComponentProps<typeof Trigger>) {
  return (
    <Trigger
      data-slot="tooltip-trigger"
      // reka 的 TooltipTrigger 额外输出该属性，Radix 没有。
      data-grace-area-trigger=""
      // Radix 的 PopperAnchor 会在触发元素上写 data-radix-popper-side/align，reka 没有。
      data-radix-popper-side={undefined}
      data-radix-popper-align={undefined}
      {...props}
    />
  )
}
