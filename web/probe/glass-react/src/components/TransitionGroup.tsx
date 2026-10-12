// React counterpart of Vue's <TransitionGroup tag=... name=... appear css>
// built on the same hooks as Transition (lib/transition.ts) plus Vue's FLIP
// move handling (runtime-dom TransitionGroup: record positions before the
// update, apply inverse translations, then add the move class).
import { Children, cloneElement, createElement, isValidElement, useCallback, useLayoutEffect, useRef, useState } from 'react'
import type { HTMLAttributes, ReactElement, ReactNode, Ref } from 'react'
import { flushSync } from 'react-dom'
import { addTransitionClass, beforeEnter, enter, forceLeaveDone, leave, removeTransitionClass, resolveClasses } from '@/lib/transition'
import type { TransitionClassProps } from '@/lib/transition'

export interface TransitionGroupProps extends TransitionClassProps, Omit<HTMLAttributes<HTMLElement>, 'children'> {
  tag?: string
  appear?: boolean
  css?: boolean
  moveClass?: string
  children?: ReactNode
}

interface Entry {
  key: string
  element: ReactElement
  leaving: boolean
}

type ElementWithRef = ReactElement<{ ref?: Ref<HTMLElement> }>
type MovingEl = HTMLElement & { _moveCb?: ((e?: TransitionEvent) => void) | null, _enterCb?: (cancelled?: boolean) => void }

function assignRef(ref: Ref<HTMLElement> | undefined, node: HTMLElement | null): void {
  if (typeof ref === 'function')
    ref(node)
  else if (ref && typeof ref === 'object')
    (ref as { current: HTMLElement | null }).current = node
}

function keyedChildren(children: ReactNode): ElementWithRef[] {
  const out: ElementWithRef[] = []
  Children.forEach(children, (c) => {
    if (isValidElement(c) && c.key != null)
      out.push(c as ElementWithRef)
  })
  return out
}

// Merge the new child list with entries that are leaving: a leaving entry
// stays right after the entry that preceded it before the update, as Vue
// leaves a removed element in place until its leave finishes.
function reconcile(prev: Entry[], next: ElementWithRef[]): Entry[] {
  const nextKeys = new Set(next.map(c => String(c.key)))
  const result: Entry[] = next.map(c => ({ key: String(c.key), element: c, leaving: false }))
  let anchor: string | null = null
  for (const e of prev) {
    if (nextKeys.has(e.key)) {
      anchor = e.key
      continue
    }
    const at = anchor === null ? 0 : result.findIndex(r => r.key === anchor) + 1
    result.splice(at, 0, { ...e, leaving: true })
    anchor = e.key
  }
  return result
}

export default function TransitionGroup(props: TransitionGroupProps) {
  const { tag, appear = false, css = true, moveClass, name, children, enterFromClass, enterActiveClass, enterToClass, appearFromClass, appearActiveClass, appearToClass, leaveFromClass, leaveActiveClass, leaveToClass, type, duration, ...rest } = props
  const transitionProps: TransitionClassProps = { name, enterFromClass, enterActiveClass, enterToClass, appearFromClass, appearActiveClass, appearToClass, leaveFromClass, leaveActiveClass, leaveToClass, type, duration }
  const propsRef = useRef({ transitionProps, css, appear, moveClass })
  propsRef.current = { transitionProps, css, appear, moveClass }

  const next = keyedChildren(children)
  const [entries, setEntries] = useState<Entry[]>(() => next.map(c => ({ key: String(c.key), element: c, leaving: false })))
  const nodes = useRef(new Map<string, HTMLElement>())
  const phase = useRef(new Map<string, 'entering' | 'idle' | 'leaving'>())
  const appearing = useRef(new Set<string>())
  const mounted = useRef(false)
  const positions = useRef(new Map<string, { left: number, top: number }>())
  const container = useRef<HTMLElement | null>(null)

  // Same render-phase reconciliation as Transition.
  const live = entries.filter(e => !e.leaving)
  const changed = live.length !== next.length
    || live.some((e, i) => e.key !== String(next[i]!.key) || e.element !== next[i])
  if (changed) {
    // Record positions before the DOM changes (Vue: render function).
    if (mounted.current && propsRef.current.css) {
      for (const [k, el] of nodes.current) {
        const r = el.getBoundingClientRect()
        positions.current.set(k, { left: r.left, top: r.top })
      }
    }
    setEntries(reconcile(entries, next))
  }

  const finishLeave = useCallback((key: string) => {
    flushSync(() => setEntries(list => list.filter(e => !(e.key === key && e.leaving))))
  }, [])

  const attach = useCallback((key: string, node: HTMLElement | null) => {
    if (!node || nodes.current.get(key) === node)
      return
    nodes.current.set(key, node)
    phase.current.set(key, 'entering')
    const p = propsRef.current
    const isAppear = !mounted.current
    if (!p.css || (isAppear && !p.appear)) {
      phase.current.set(key, 'idle')
      return
    }
    if (isAppear)
      appearing.current.add(key)
    beforeEnter(node, resolveClasses(p.transitionProps), isAppear)
  }, [])

  useLayoutEffect(() => {
    const p = propsRef.current
    const classes = resolveClasses(p.transitionProps)
    const liveKeys = new Set(entries.map(e => e.key))
    for (const [key, el] of nodes.current) {
      if (liveKeys.has(key))
        continue
      forceLeaveDone(el)
      nodes.current.delete(key)
      phase.current.delete(key)
      appearing.current.delete(key)
      positions.current.delete(key)
    }
    for (const entry of entries) {
      const el = nodes.current.get(entry.key)
      if (!el)
        continue
      const state = phase.current.get(entry.key)
      if (entry.leaving) {
        if (state === 'leaving')
          continue
        phase.current.set(entry.key, 'leaving')
        if (!p.css)
          queueMicrotask(() => finishLeave(entry.key))
        else
          leave(el, classes, p.transitionProps, () => finishLeave(entry.key))
      }
      else if (state === 'entering') {
        phase.current.set(entry.key, 'idle')
        enter(el, classes, p.transitionProps, appearing.current.has(entry.key))
        appearing.current.delete(entry.key)
      }
    }

    // FLIP moves (Vue: onUpdated).
    if (mounted.current && p.css && positions.current.size) {
      const mc = p.moveClass || `${p.transitionProps.name || 'v'}-move`
      const moved: MovingEl[] = []
      for (const [key, oldPos] of positions.current) {
        const el = nodes.current.get(key) as MovingEl | undefined
        if (!el)
          continue
        el._moveCb?.()
        el._enterCb?.()
        const r = el.getBoundingClientRect()
        const dx = oldPos.left - r.left
        const dy = oldPos.top - r.top
        if (dx || dy) {
          let scaleX = el.offsetWidth ? r.width / el.offsetWidth : 1
          let scaleY = el.offsetHeight ? r.height / el.offsetHeight : 1
          if (!Number.isFinite(scaleX) || scaleX === 0 || Math.abs(scaleX - 1) < 0.01)
            scaleX = 1
          if (!Number.isFinite(scaleY) || scaleY === 0 || Math.abs(scaleY - 1) < 0.01)
            scaleY = 1
          el.style.transform = `translate(${dx / scaleX}px,${dy / scaleY}px)`
          el.style.transitionDuration = '0s'
          moved.push(el)
        }
      }
      positions.current.clear()
      if (moved.length && container.current) {
        void container.current.offsetHeight
        for (const el of moved) {
          addTransitionClass(el, mc)
          el.style.transform = el.style.transitionDuration = ''
          const cb = el._moveCb = (e?: TransitionEvent) => {
            if (e && e.target !== el)
              return
            if (!e || e.propertyName.endsWith('transform')) {
              el.removeEventListener('transitionend', cb as EventListener)
              el._moveCb = null
              removeTransitionClass(el, mc)
            }
          }
          el.addEventListener('transitionend', cb as EventListener)
        }
      }
    }
    positions.current.clear()
    mounted.current = true
  })

  const rendered = entries.map((entry) => {
    const own = (entry.element.props as { ref?: Ref<HTMLElement> }).ref
    return cloneElement(entry.element as ElementWithRef, {
      key: entry.key,
      ref: (node: HTMLElement | null) => {
        attach(entry.key, node)
        assignRef(own, node)
      },
    })
  })
  if (!tag)
    return <>{rendered}</>
  return createElement(tag, { ...rest, ref: container }, rendered)
}
