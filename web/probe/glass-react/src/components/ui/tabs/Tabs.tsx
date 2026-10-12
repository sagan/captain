import type { ComponentProps } from 'react'
import type { TabsState } from './context'
import { Root as TabsRoot } from '@radix-ui/react-tabs'
import { useCallback, useId, useMemo, useState } from 'react'
import { cn } from '@/lib/utils'
import { TabsStateContext } from './context'

export interface TabsProps extends Omit<ComponentProps<typeof TabsRoot>, 'value' | 'defaultValue' | 'onValueChange'> {
  value?: string
  defaultValue?: string
  onValueChange?: (value: string) => void
}

export function Tabs({ className, value: valueProp, defaultValue, onValueChange, orientation, dir, ...props }: TabsProps) {
  const [inner, setInner] = useState(defaultValue)
  const value = valueProp !== undefined ? valueProp : inner
  const baseId = `reka-tabs-${useId()}`
  const [registered, setRegistered] = useState<ReadonlySet<string>>(() => new Set())

  const handleChange = useCallback((next: string) => {
    if (valueProp === undefined)
      setInner(next)
    onValueChange?.(next)
  }, [valueProp, onValueChange])

  const register = useCallback((v: string) => {
    setRegistered(prev => new Set([...prev, v]))
    return () => setRegistered((prev) => {
      const next = new Set(prev)
      next.delete(v)
      return next
    })
  }, [])

  const state = useMemo<TabsState>(() => ({
    baseId,
    value,
    dir: dir ?? 'ltr',
    registered,
    register,
  }), [baseId, value, dir, registered, register])

  return (
    <TabsStateContext.Provider value={state}>
      <TabsRoot
        data-slot="tabs"
        data-orientation={orientation || 'horizontal'}
        {...props}
        orientation={orientation}
        dir={dir}
        value={value ?? ''}
        onValueChange={handleChange}
        className={cn('gap-2 group/tabs flex data-horizontal:flex-col', className)}
      />
    </TabsStateContext.Provider>
  )
}
