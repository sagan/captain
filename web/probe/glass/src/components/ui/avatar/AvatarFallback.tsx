import type { ComponentProps } from 'react'
import { Fallback } from '@radix-ui/react-avatar'
import { useContext } from 'react'
import { cn } from '@/lib/utils'
import { AvatarStateContext } from './context'

export function AvatarFallback({ className, ...props }: ComponentProps<typeof Fallback>) {
  // 图片状态由本目录的 AvatarImage 维护（见 AvatarImage.tsx），Radix 只负责 delayMs。
  const { status } = useContext(AvatarStateContext)
  if (status === 'loaded')
    return null
  return (
    <Fallback
      data-slot="avatar-fallback"
      className={cn('bg-muted text-muted-foreground rounded-full flex size-full items-center justify-center text-sm group-data-[size=sm]/avatar:text-xs', className)}
      {...props}
    />
  )
}
