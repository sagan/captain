import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

export interface SpinnerProps {
  size?: number | string
  stroke?: number | string
  show?: boolean
  /** 描述文字（对应 Vue 的 description 属性与 #description 插槽） */
  description?: ReactNode
  contentClass?: string
  className?: string
  children?: ReactNode
}

function toSize(v: number | string) {
  return typeof v === 'number' ? `${v}px` : v
}

export function Spinner({ size = 24, stroke = 2, show = true, description, contentClass, className, children }: SpinnerProps) {
  return (
    <div className={cn('relative', className)}>
      {children}
      {show && (
        <div
          className={cn(
            'absolute inset-0 z-10 flex flex-col items-center justify-center gap-3 bg-card/60 backdrop-blur-sm',
            contentClass,
          )}
        >
          <span
            className="inline-block animate-spin rounded-full border-solid"
            style={{
              width: toSize(size),
              height: toSize(size),
              borderWidth: toSize(stroke),
              borderColor: 'color-mix(in srgb, currentColor 18%, transparent)',
              borderTopColor: 'currentColor',
            }}
          />
          {(description || description === 0)
            ? (
                <span className="text-sm text-muted-foreground">
                  {description}
                </span>
              )
            : null}
        </div>
      )}
    </div>
  )
}
