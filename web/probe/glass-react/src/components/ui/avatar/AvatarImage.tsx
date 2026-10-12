import type { ComponentProps } from 'react'
import type { ImageLoadingStatus } from './context'
import { useContext, useLayoutEffect, useRef, useState } from 'react'
import { AvatarStateContext } from './context'

export interface AvatarImageProps extends Omit<ComponentProps<'img'>, 'src'> {
  src: string
  onLoadingStatusChange?: (status: ImageLoadingStatus) => void
}

const HIDDEN = { display: 'none' }
const SHOWN = {}

// reka 的 AvatarImage 始终渲染同一个 <img role="img">，用 v-show 在加载完成前隐藏
// （完成后留下 style=""）；Radix 在加载前不渲染元素。为保持 DOM 一致，这里按 reka 的
// useImageLoadingStatus 自行跟踪状态，并通过 Avatar 的上下文通知 AvatarFallback。
export function AvatarImage({ src, referrerPolicy, crossOrigin, onLoadingStatusChange, children, ...props }: AvatarImageProps) {
  const root = useContext(AvatarStateContext)
  const [status, setStatus] = useState<ImageLoadingStatus>('idle')
  const latest = useRef({ root, onLoadingStatusChange })
  latest.current = { root, onLoadingStatusChange }

  useLayoutEffect(() => {
    const img = new window.Image()
    let alive = true
    const update = (next: ImageLoadingStatus) => () => {
      if (alive)
        setStatus(next)
    }
    const onLoad = update('loaded')
    const onError = update('error')
    img.addEventListener('load', onLoad)
    img.addEventListener('error', onError)
    if (referrerPolicy)
      img.referrerPolicy = referrerPolicy
    if (typeof crossOrigin === 'string')
      img.crossOrigin = crossOrigin
    if (!src) {
      setStatus('error')
    }
    else {
      img.src = src
      setStatus(img.complete && img.naturalWidth > 0 ? 'loaded' : 'loading')
    }
    return () => {
      alive = false
      img.removeEventListener('load', onLoad)
      img.removeEventListener('error', onError)
    }
  }, [src, referrerPolicy, crossOrigin])

  useLayoutEffect(() => {
    latest.current.onLoadingStatusChange?.(status)
    if (status !== 'idle')
      latest.current.root.setStatus(status)
  }, [status])

  return (
    <img
      data-slot="avatar-image"
      {...props}
      role="img"
      src={src}
      referrerPolicy={referrerPolicy}
      crossOrigin={crossOrigin}
      className="rounded-full aspect-square size-full object-cover"
      style={status === 'loaded' ? SHOWN : HIDDEN}
    >
      {children}
    </img>
  )
}
