import type { ComponentProps } from 'react'
import { Provider } from '@radix-ui/react-tooltip'

export function TooltipProvider({ delayDuration = 0, ...props }: ComponentProps<typeof Provider>) {
  return <Provider delayDuration={delayDuration} {...props} />
}
