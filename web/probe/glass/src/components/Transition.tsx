// React counterpart of Vue's <Transition> for one optional keyed child,
// built on Vue's own hook sequence (lib/transition.ts). The child must accept
// a ref to its root DOM element. Leaving children stay mounted until their
// leave finishes; mode="out-in" holds the next child back until then. As in
// Vue's BaseTransition, a key that re-enters while its old element is still
// leaving removes that element at once.
import { cloneElement, isValidElement, useCallback, useLayoutEffect, useRef, useState } from 'react'
import type { ReactElement, Ref } from 'react'
import { flushSync } from 'react-dom'
import { beforeEnter, enter, forceLeaveDone, leave, resolveClasses } from '@/lib/transition'
import type { TransitionClassProps } from '@/lib/transition'

export interface TransitionProps extends TransitionClassProps {
  /** false disables class handling, like Vue's :css="false". */
  css?: boolean
  appear?: boolean
  mode?: 'default' | 'out-in' | 'in-out'
  children?: ReactElement | false | null | undefined
}

interface Entry {
  id: number
  key: string
  element: ReactElement
  leaving: boolean
}

type ElementWithRef = ReactElement<{ ref?: Ref<HTMLElement> }>

let nextId = 0

function keyOf(el: ReactElement): string {
  return el.key ?? '__default__'
}

function assignRef(ref: Ref<HTMLElement> | undefined, node: HTMLElement | null): void {
  if (typeof ref === 'function')
    ref(node)
  else if (ref && typeof ref === 'object')
    (ref as { current: HTMLElement | null }).current = node
}

function enterEntry(list: Entry[], child: ReactElement): Entry[] {
  const key = keyOf(child)
  return [...list.filter(e => !(e.leaving && e.key === key)), { id: ++nextId, key, element: child, leaving: false }]
}

export default function Transition(props: TransitionProps) {
  const { css = true, mode = 'default', children } = props
  const child = isValidElement(children) ? children as ElementWithRef : null
  const mounted = useRef(false)
  const nodes = useRef(new Map<number, HTMLElement>())
  const phase = useRef(new Map<number, 'entering' | 'idle' | 'leaving'>())
  const appearing = useRef(new Set<number>())
  const propsRef = useRef(props)
  propsRef.current = props
  const pendingChild = useRef<ElementWithRef | null>(null)
  pendingChild.current = child

  const [entries, setEntries] = useState<Entry[]>(() => child ? enterEntry([], child) : [])

  // Reconcile the requested child with what is on screen.
  const childKey = child ? keyOf(child) : null
  const current = entries.find(e => !e.leaving)
  const holdForOutIn = mode === 'out-in' && css
  let next = entries
  if (child && current && current.key === childKey) {
    if (current.element !== child)
      next = entries.map(e => e === current ? { ...e, element: child } : e)
  }
  else if (current) {
    next = entries.map(e => e === current ? { ...e, leaving: true } : e)
    if (child && !holdForOutIn)
      next = enterEntry(next, child)
  }
  else if (child && !(holdForOutIn && entries.some(e => e.leaving))) {
    next = enterEntry(entries, child)
  }
  if (next !== entries)
    setEntries(next)

  const finishLeave = useCallback((id: number) => {
    // Vue removes the element in the same task that clears its leave classes;
    // flushSync commits the removal before that task ends.
    flushSync(() => setEntries((list) => {
      if (!list.some(e => e.id === id))
        return list
      const remaining = list.filter(e => e.id !== id)
      const pending = pendingChild.current
      if (pending && !remaining.some(e => !e.leaving))
        return enterEntry(remaining, pending)
      return remaining
    }))
  }, [])

  // beforeEnter must run before anything reads layout, so it happens as soon
  // as React attaches the new element.
  const attach = useCallback((id: number, node: HTMLElement | null) => {
    if (!node || nodes.current.has(id))
      return
    nodes.current.set(id, node)
    phase.current.set(id, 'entering')
    const p = propsRef.current
    const isAppear = !mounted.current
    if (p.css === false || (isAppear && !p.appear)) {
      phase.current.set(id, 'idle')
      return
    }
    if (isAppear)
      appearing.current.add(id)
    beforeEnter(node, resolveClasses(p), isAppear)
  }, [])

  useLayoutEffect(() => {
    const p = propsRef.current
    const classes = resolveClasses(p)
    const live = new Set(entries.map(e => e.id))
    for (const [id, el] of nodes.current) {
      if (live.has(id))
        continue
      // Removed in this commit while leaving (early removal): settle its callback.
      forceLeaveDone(el)
      nodes.current.delete(id)
      phase.current.delete(id)
      appearing.current.delete(id)
    }
    for (const entry of entries) {
      const el = nodes.current.get(entry.id)
      if (!el)
        continue
      const state = phase.current.get(entry.id)
      if (entry.leaving) {
        if (state === 'leaving')
          continue
        phase.current.set(entry.id, 'leaving')
        if (p.css === false)
          queueMicrotask(() => finishLeave(entry.id)) // flushSync is not allowed inside an effect
        else
          leave(el, classes, p, () => finishLeave(entry.id))
      }
      else if (state === 'entering') {
        phase.current.set(entry.id, 'idle')
        enter(el, classes, p, appearing.current.has(entry.id))
        appearing.current.delete(entry.id)
      }
    }
    mounted.current = true
  })

  return (
    <>
      {entries.map((entry) => {
        const own = (entry.element.props as { ref?: Ref<HTMLElement> }).ref
        return cloneElement(entry.element as ElementWithRef, {
          key: `${entry.key}#${entry.id}`,
          ref: (node: HTMLElement | null) => {
            attach(entry.id, node)
            assignRef(own, node)
          },
        })
      })}
    </>
  )
}
