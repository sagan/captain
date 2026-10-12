import type { CSSProperties, ComponentProps } from 'react'
import { Arrow, Content, Portal } from '@radix-ui/react-tooltip'
import { useCallback, useId, useLayoutEffect, useState } from 'react'
import { cn } from '@/lib/utils'

// 类名里的 origin-(--reka-tooltip-content-transform-origin) 依赖 reka 的变量名，
// 这里把同名变量指向 Radix popper 的对应变量。
const REKA_VARS = {
  '--reka-tooltip-content-transform-origin': 'var(--radix-popper-transform-origin)',
  '--reka-tooltip-content-available-width': 'var(--radix-popper-available-width)',
  '--reka-tooltip-content-available-height': 'var(--radix-popper-available-height)',
  '--reka-tooltip-trigger-width': 'var(--radix-popper-anchor-width)',
  '--reka-tooltip-trigger-height': 'var(--radix-popper-anchor-height)',
} as CSSProperties

// reka VisuallyHidden 的内联样式（按其声明顺序）
const REKA_VISUALLY_HIDDEN = 'position: absolute; border: 0px; width: 1px; height: 1px; padding: 0px; margin: -1px; overflow: hidden; clip: rect(0px, 0px, 0px, 0px); clip-path: inset(50%); white-space: nowrap; overflow-wrap: normal; top: -1px; left: -1px;'

// reka 的 Arrow 把未声明为 DOM 属性的 rounded 也渲染成了特性
const ARROW_ATTRS: Record<string, string> = { rounded: 'false' }

export function TooltipContent({
  className,
  sideOffset = 0,
  children,
  style,
  'aria-label': ariaLabelProp,
  ...props
}: ComponentProps<typeof Content>) {
  // reka 总是渲染一个 VisuallyHidden 的 role="tooltip"，内容为 aria-label 或挂载时的 textContent，
  // 内容元素本身不带 role/id。Radix 只有在传入 aria-label 时才这样渲染，所以挂载后读取文本再传入。
  const [measured, setMeasured] = useState<string | undefined>(undefined)
  const [node, setNode] = useState<HTMLDivElement | null>(null)
  const measure = useCallback((el: HTMLDivElement | null) => {
    setNode(el)
    if (!el)
      return
    setMeasured(el.textContent ?? '')
    return () => setMeasured(undefined)
  }, [])
  const ariaLabel = ariaLabelProp || measured
  // 传入 id 会成为 Radix 的 contentId（触发器 aria-describedby 与隐藏 span 使用），沿用 reka 的前缀。
  const contentId = `reka-tooltip-content-${useId()}`

  // 对齐 reka 的 DOM：定位外层用 data-reka-popper-content-wrapper；隐藏文本 span 带
  // reka VisuallyHidden 额外的 clip-path/top/left。这些都是 React 不再改写的常量。
  useLayoutEffect(() => {
    const wrapper = node?.parentElement
    if (wrapper?.hasAttribute('data-radix-popper-content-wrapper')) {
      wrapper.removeAttribute('data-radix-popper-content-wrapper')
      wrapper.setAttribute('data-reka-popper-content-wrapper', '')
    }
    const hidden = node?.querySelector<HTMLElement>(':scope > span[role="tooltip"]')
    if (hidden)
      hidden.style.cssText = REKA_VISUALLY_HIDDEN
  }, [node, ariaLabel])

  return (
    <Portal>
      <Content
        ref={measure}
        data-slot="tooltip-content"
        data-dismissable-layer=""
        dir="ltr"
        sideOffset={sideOffset}
        id={contentId}
        {...props}
        aria-label={ariaLabel}
        style={{ ...REKA_VARS, ...style }}
        className={cn('data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95 data-[state=delayed-open]:animate-in data-[state=delayed-open]:fade-in-0 data-[state=delayed-open]:zoom-in-95 data-closed:animate-out data-closed:fade-out-0 data-closed:zoom-out-95 data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-right-2 data-[side=right]:slide-in-from-left-2 data-[side=top]:slide-in-from-bottom-2 inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs has-data-[slot=kbd]:pr-1.5 **:data-[slot=kbd]:relative **:data-[slot=kbd]:isolate **:data-[slot=kbd]:z-50 **:data-[slot=kbd]:rounded-sm bg-primary text-primary-foreground z-50 w-fit max-w-xs origin-(--reka-tooltip-content-transform-origin)', className)}
      >
        {children}
        {/* asChild + 自带 svg：与 reka 的 Arrow（viewBox 0 0 12 6 的 path）一致，Radix 默认是 polygon */}
        <Arrow asChild>
          <svg className="size-2.5 rotate-45 rounded-[2px] data-[side=left]:translate-x-[-1.5px] data-[side=right]:translate-x-[1.5px] bg-primary fill-primary z-50 translate-y-[calc(-50%_-_2px)]" viewBox="0 0 12 6" preserveAspectRatio="none" {...ARROW_ATTRS}>
            <path d="M0 0L6 6L12 0" />
          </svg>
        </Arrow>
      </Content>
    </Portal>
  )
}
