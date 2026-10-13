import type { ComponentProps } from 'react'
import type { TabsListVariants } from '@shared/ui/tabs'
import { List } from '@radix-ui/react-tabs'
import { tabsListVariants } from '@shared/ui/tabs'
import { cn } from '@/lib/utils'
import { useTabsState } from './context'

export interface TabsListProps extends ComponentProps<typeof List> {
  variant?: TabsListVariants['variant']
}

// Vue 包装层把未传的 loop（Boolean 类型）转成 false 再显式传给 reka，覆盖了 reka 的默认 true；
// 这里同样默认 loop=false，保证方向键在两端停止而不循环。
export function TabsList({ className, variant = 'default', loop = false, ...props }: TabsListProps) {
  const { dir } = useTabsState('TabsList')
  return (
    <List
      data-slot="tabs-list"
      data-variant={variant}
      // reka 的 RovingFocusGroup 会把 dir 渲染到 tablist 上，Radix 不会。
      dir={dir}
      loop={loop}
      {...props}
      className={cn(tabsListVariants({ variant }), className)}
    />
  )
}
