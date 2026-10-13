import type { ComponentProps } from 'react'
import type { AvatarVariants } from '@shared/ui/avatar'
import type { ImageLoadingStatus } from './context'
import { Root as AvatarRoot } from '@radix-ui/react-avatar'
import { avatarVariants } from '@shared/ui/avatar'
import { useMemo, useState } from 'react'
import { cn } from '@/lib/utils'
import { AvatarStateContext } from './context'

export interface AvatarProps extends ComponentProps<typeof AvatarRoot> {
  size?: AvatarVariants['size']
}

export function Avatar({ className, size, ...props }: AvatarProps) {
  const [status, setStatus] = useState<ImageLoadingStatus>('idle')
  const state = useMemo(() => ({ status, setStatus }), [status])
  return (
    <AvatarStateContext.Provider value={state}>
      <AvatarRoot
        data-slot="avatar"
        data-size={size ?? 'default'}
        className={cn(avatarVariants({ size }), className)}
        {...props}
      />
    </AvatarStateContext.Provider>
  )
}
