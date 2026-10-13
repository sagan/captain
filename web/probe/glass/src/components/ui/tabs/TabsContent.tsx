import type { ComponentProps } from 'react'
import { Content } from '@radix-ui/react-tabs'
import { useEffect, useState } from 'react'
import { cn } from '@/lib/utils'
import { makeContentId, makeTriggerId, useTabsState } from './context'

export function TabsContent({ className, value, ...props }: ComponentProps<typeof Content>) {
  const { baseId, register } = useTabsState('TabsContent')
  useEffect(() => register(value), [register, value])
  // Radix 用 ref 记录挂载时的 animationDuration: 0s，rAF 后不会重渲染；reka 会。
  // 子组件的 rAF 先注册，这里随后触发一次重渲染，让 style 与 reka 一样变成空。
  const [, rerender] = useState(0)
  useEffect(() => {
    const id = requestAnimationFrame(() => rerender(n => n + 1))
    return () => cancelAnimationFrame(id)
  }, [])
  return (
    <Content
      data-slot="tabs-content"
      className={cn('text-sm flex-1 outline-none', className)}
      value={value}
      id={makeContentId(baseId, value)}
      aria-labelledby={makeTriggerId(baseId, value)}
      {...props}
    />
  )
}
