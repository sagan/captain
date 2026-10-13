import type { ComponentProps, ElementType } from 'react'
import type { ButtonVariants } from '@shared/ui/button'
import { Slot } from '@radix-ui/react-slot'
import { buttonVariants } from '@shared/ui/button'
import { cn } from '@/lib/utils'

export interface ButtonProps extends ComponentProps<'button'> {
  variant?: ButtonVariants['variant']
  size?: ButtonVariants['size']
  as?: ElementType
  asChild?: boolean
}

export function Button({ className, variant, size, as = 'button', asChild = false, ...props }: ButtonProps) {
  const Comp: ElementType = asChild ? Slot : as
  return (
    <Comp
      data-slot="button"
      data-variant={variant}
      data-size={size}
      className={cn(buttonVariants({ variant, size }), className)}
      {...props}
    />
  )
}
