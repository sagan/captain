import type { ComponentProps, ElementType } from 'react'
import type { BadgeVariants } from '@shared/ui/badge'
import { Slot } from '@radix-ui/react-slot'
import { badgeVariants } from '@shared/ui/badge'
import { cn } from '@/lib/utils'

export interface BadgeProps extends ComponentProps<'div'> {
  variant?: BadgeVariants['variant']
  as?: ElementType
  asChild?: boolean
}

export function Badge({ className, variant, as = 'div', asChild = false, ...props }: BadgeProps) {
  const Comp: ElementType = asChild ? Slot : as
  // Vue 版用 v-bind 透传 reka 的 delegatedProps，variant 会作为普通特性落到 DOM 上。
  const variantAttr = variant != null ? { variant } : null
  return (
    <Comp
      data-slot="badge"
      data-variant={variant}
      className={cn(badgeVariants({ variant }), className)}
      {...variantAttr}
      {...props}
    />
  )
}
