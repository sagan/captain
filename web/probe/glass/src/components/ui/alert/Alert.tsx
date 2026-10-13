import type { ComponentProps } from 'react'
import type { AlertVariants } from '@shared/ui/alert'
import { alertVariants } from '@shared/ui/alert'
import { cn } from '@/lib/utils'

export interface AlertProps extends ComponentProps<'div'> {
  variant?: AlertVariants['variant']
}

export function Alert({ className, variant, ...props }: AlertProps) {
  return (
    <div
      data-slot="alert"
      className={cn(alertVariants({ variant }), className)}
      role="alert"
      {...props}
    />
  )
}
