import { cn } from '@/lib/utils'

export interface ProgressThinProps {
  percentage?: number
  status?: 'default' | 'success' | 'warning' | 'error' | 'info'
  height?: number | string
  className?: string
}

function statusClassOf(status: ProgressThinProps['status']) {
  switch (status) {
    case 'success': return 'bg-success'
    case 'warning': return 'bg-warning'
    case 'error': return 'bg-destructive'
    case 'info': return 'bg-info'
    default: return 'bg-primary'
  }
}

export function ProgressThin({ percentage = 0, status = 'default', height = 4, className }: ProgressThinProps) {
  const clamped = Math.max(0, Math.min(100, Number(percentage) || 0))
  return (
    <div
      className={cn('relative w-full overflow-hidden rounded-full bg-muted', className)}
      style={{ height: typeof height === 'number' ? `${height}px` : height }}
    >
      {/* Vue 的静态 class + :class 只做字符串拼接，不经过 cn */}
      <div
        className={`h-full rounded-full transition-[width] duration-300 ease-out ${statusClassOf(status)}`}
        style={{ width: `${clamped}%` }}
      />
    </div>
  )
}
