import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

export interface EmptyProps {
  description?: string
  className?: string
  /** 默认插槽：替换默认的描述文字 */
  children?: ReactNode
  /** #extra 插槽 */
  extra?: ReactNode
}

export function Empty({ description, className, children, extra }: EmptyProps) {
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center gap-3 p-8 text-muted-foreground',
        className,
      )}
    >
      <svg
        xmlns="http://www.w3.org/2000/svg"
        width="56"
        height="56"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        className="opacity-60"
      >
        <rect width="18" height="14" x="3" y="3" rx="2" />
        <path d="M3 14h18M8 18h.01M12 18h.01M16 18h.01" />
      </svg>
      {children !== undefined && children !== null && children !== false
        ? children
        : <span className="text-sm">{description ?? '暂无数据'}</span>}
      {extra}
    </div>
  )
}
