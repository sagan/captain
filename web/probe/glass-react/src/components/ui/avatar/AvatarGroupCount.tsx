import type { ComponentProps } from 'react'
import { cn } from '@/lib/utils'

export function AvatarGroupCount({ className, ...props }: ComponentProps<'div'>) {
  return (
    <div
      data-slot="avatar-group-count"
      className={cn(
        'bg-muted text-muted-foreground size-8 rounded-full text-sm group-has-data-[size=lg]/avatar-group:size-10 group-has-data-[size=sm]/avatar-group:size-6 [&>svg]:size-4 group-has-data-[size=lg]/avatar-group:[&>svg]:size-5 group-has-data-[size=sm]/avatar-group:[&>svg]:size-3 relative flex shrink-0 items-center justify-center ring-2 ring-background',
        className,
      )}
      {...props}
    />
  )
}
