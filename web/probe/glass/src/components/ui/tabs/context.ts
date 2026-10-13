import { createContext, useContext } from 'react'

// reka 的 TabsTrigger 依赖的 data-active、TabsList 上的 dir、以及仅在对应
// TabsContent 挂载后才输出的 aria-controls，Radix 都不提供；这里自行维护。
export interface TabsState {
  baseId: string
  value: string | undefined
  dir: 'ltr' | 'rtl'
  registered: ReadonlySet<string>
  register: (value: string) => () => void
}

export const TabsStateContext = createContext<TabsState | null>(null)

export function useTabsState(name: string): TabsState {
  const ctx = useContext(TabsStateContext)
  if (!ctx)
    throw new Error(`<${name}> must be used within <Tabs>`)
  return ctx
}

// reka 的 makeTriggerId / makeContentId
export function makeTriggerId(baseId: string, value: string) {
  return `${baseId}-trigger-${value}`
}
export function makeContentId(baseId: string, value: string) {
  return `${baseId}-content-${value}`
}
