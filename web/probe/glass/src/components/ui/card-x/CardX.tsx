import type { ComponentProps, ReactNode } from 'react'
import { cn } from '@/lib/utils'

export interface CardXProps extends Omit<ComponentProps<'div'>, 'title'> {
  title?: string
  size?: 'small' | 'medium' | 'large'
  hoverable?: boolean
  bordered?: boolean
  segmented?: boolean | { content?: boolean, footer?: 'soft' | true }
  contentClass?: string
  headerClass?: string
  /** #header 插槽 */
  header?: ReactNode
  /** #header-extra 插槽 */
  headerExtra?: ReactNode
  /** #footer 插槽 */
  footer?: ReactNode
}

function hasSlot(node: ReactNode): boolean {
  return node !== undefined && node !== null && node !== false
}

export function CardX({
  title,
  size = 'medium',
  hoverable = false,
  bordered = true,
  segmented,
  contentClass,
  headerClass,
  className,
  header,
  headerExtra,
  footer,
  children,
  ...attrs
}: CardXProps) {
  const hasHeader = hasSlot(header)
  const hasHeaderExtra = hasSlot(headerExtra)
  const hasFooter = hasSlot(footer)

  const isShowHeader = hasHeader || hasHeaderExtra || title
  const pt = isShowHeader ? 'pt-0' : ''
  const paddingClass = size === 'small' ? `p-3 ${pt}` : size === 'large' ? `p-6 ${pt}` : `p-4 ${pt}`
  const headerPaddingClass = size === 'small' ? 'px-3 py-2' : size === 'large' ? 'px-6 py-4' : 'px-4 py-3'
  const segmentedContent = typeof segmented === 'boolean' ? segmented : !!segmented?.content
  const segmentedFooter = typeof segmented === 'boolean' ? segmented : !!segmented?.footer

  return (
    <div
      {...attrs}
      className={cn(
        'bg-card text-card-foreground flex flex-col rounded-lg',
        bordered && 'border',
        hoverable && 'transition-colors hover:border-foreground/30',
        className,
      )}
    >
      {(hasHeader || title || hasHeaderExtra)
        ? (
            <div
              className={cn(
                'flex items-center gap-2',
                headerPaddingClass,
                segmentedContent && 'border-b',
                headerClass,
              )}
            >
              <div className="flex-1 min-w-0">
                {hasHeader ? header : <span className="font-medium">{title}</span>}
              </div>
              {hasHeaderExtra && (
                <div className="shrink-0">
                  {headerExtra}
                </div>
              )}
            </div>
          )
        : null}
      <div className={cn(paddingClass, contentClass)}>
        {children}
      </div>
      {hasFooter && (
        <div
          className={cn(
            headerPaddingClass,
            segmentedFooter && 'border-t bg-muted/40',
          )}
        >
          {footer}
        </div>
      )}
    </div>
  )
}
