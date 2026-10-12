import type { CSSProperties, ElementType, HTMLAttributes, ReactNode } from 'react'
import { cn } from '@/lib/utils'

type DataTooltipPlacement = 'top' | 'bottom' | 'left' | 'right'

export interface DataTooltipProps extends Omit<HTMLAttributes<HTMLElement>, 'content'> {
  /** 提示内容（对应 Vue 的 content 属性与 #content 插槽），为空时不渲染气泡 */
  content?: ReactNode
  /** 气泡相对触发元素的方位 */
  placement?: DataTooltipPlacement
  /** 气泡宽度，number 视为 px；默认由内容撑起 */
  width?: number | string
  /** 气泡高度，number 视为 px；默认由内容撑起 */
  height?: number | string
  /** 包裹元素标签，默认 div */
  as?: ElementType
  /** 气泡的附加类 */
  contentClass?: string
}

const placementClass: Record<DataTooltipPlacement, string> = {
  top: 'bottom-full left-1/2 mb-2 -translate-x-1/2',
  bottom: 'top-full left-1/2 mt-2 -translate-x-1/2',
  left: 'top-1/2 right-full mr-2 -translate-y-1/2',
  right: 'top-1/2 left-full ml-2 -translate-y-1/2',
}

export function DataTooltip({
  content,
  placement = 'top',
  width,
  height,
  as: Comp = 'div',
  className,
  contentClass,
  children,
  ...attrs
}: DataTooltipProps) {
  const sizeStyle: CSSProperties = {}
  if (width != null)
    sizeStyle.width = typeof width === 'number' ? `${width}px` : width
  if (height != null)
    sizeStyle.height = typeof height === 'number' ? `${height}px` : height

  return (
    <Comp
      {...attrs}
      data-slot="data-tooltip"
      className={cn('group/data-tooltip relative inline-block', className)}
    >
      {children}
      {(content || content === 0)
        ? (
            <span
              role="tooltip"
              className={cn(
                'pointer-events-none absolute z-20 hidden rounded bg-foreground/80 p-1 text-[10px] leading-none text-background shadow-lg group-hover/data-tooltip:block group-focus-within/data-tooltip:block whitespace-normal break-words',
                placementClass[placement],
                contentClass,
              )}
              style={sizeStyle}
            >
              {content}
            </span>
          )
        : null}
    </Comp>
  )
}
