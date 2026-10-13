import type { ComponentProps, MouseEvent } from 'react'
import { Trigger } from '@radix-ui/react-tabs'
import { cn } from '@/lib/utils'
import { makeContentId, makeTriggerId, useTabsState } from './context'

// Radix 在 mousedown 里调用 currentTarget.focus()，Chromium 会因此给按钮加上
// :focus-visible 焦点环；reka 交给浏览器原生聚焦，不会出现焦点环。这里在 Radix 的
// 处理器运行期间把 focus() 屏蔽掉（随后的微任务恢复），原生聚焦照常发生。
function suppressScriptFocus(e: MouseEvent<HTMLButtonElement>) {
  const el = e.currentTarget as HTMLButtonElement & { focus: HTMLElement['focus'] }
  if (Object.hasOwn(el, 'focus'))
    return
  el.focus = () => {}
  queueMicrotask(() => {
    delete (el as Partial<typeof el>).focus
  })
}

export function TabsTrigger({ className, value, onMouseDown, ...props }: ComponentProps<typeof Trigger>) {
  const state = useTabsState('TabsTrigger')
  const active = state.value === value
  return (
    <Trigger
      data-slot="tabs-trigger"
      className={cn(
        'gap-1.5 rounded-md border border-transparent px-2 py-1 text-sm font-medium group-data-vertical/tabs:px-2.5 group-data-vertical/tabs:py-1.5 [&_svg:not([class*=size-])]:size-4 has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 relative inline-flex h-[calc(100%-1px)] flex-1 items-center justify-center whitespace-nowrap text-foreground/60 transition-all group-data-vertical/tabs:w-full group-data-vertical/tabs:justify-start hover:text-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-1 focus-visible:outline-ring disabled:pointer-events-none disabled:opacity-50 dark:text-muted-foreground dark:hover:text-foreground [&_svg]:pointer-events-none [&_svg]:shrink-0',
        'group-data-[variant=line]/tabs-list:bg-transparent group-data-[variant=line]/tabs-list:data-active:bg-transparent dark:group-data-[variant=line]/tabs-list:data-active:border-transparent dark:group-data-[variant=line]/tabs-list:data-active:bg-transparent',
        'data-active:bg-background data-active:text-foreground dark:data-active:border-input dark:data-active:bg-input/30 dark:data-active:text-foreground',
        'after:absolute after:bg-foreground after:opacity-0 after:transition-opacity group-data-horizontal/tabs:after:inset-x-0 group-data-horizontal/tabs:after:bottom-[-5px] group-data-horizontal/tabs:after:h-0.5 group-data-vertical/tabs:after:inset-y-0 group-data-vertical/tabs:after:-right-1 group-data-vertical/tabs:after:w-0.5 group-data-[variant=line]/tabs-list:data-active:after:opacity-100',
        className,
      )}
      value={value}
      // reka 的 RovingFocusItem 在选中项上输出 data-active，上面的样式依赖它。
      data-active={active ? '' : undefined}
      // reka 的集合标记。去掉 Radix 的同类标记后，方向键导航按注册（挂载）顺序排序。
      data-reka-collection-item=""
      data-radix-collection-item={undefined}
      // 使用 reka 格式的 id；reka 只在对应 TabsContent 已挂载时输出 aria-controls。
      id={makeTriggerId(state.baseId, value)}
      aria-controls={state.registered.has(value) ? makeContentId(state.baseId, value) : undefined}
      {...props}
      onMouseDown={(e) => {
        onMouseDown?.(e)
        if (!e.defaultPrevented)
          suppressScriptFocus(e)
      }}
    />
  )
}
