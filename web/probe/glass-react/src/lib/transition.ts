// DOM side of Vue's <Transition> (runtime-dom resolveTransitionProps and
// whenTransitionEnds, Vue 3.5), ported so the React build adds and removes
// exactly the same classes at the same moments.

export interface TransitionClasses {
  enterFromClass: string
  enterActiveClass: string
  enterToClass: string
  appearFromClass: string
  appearActiveClass: string
  appearToClass: string
  leaveFromClass: string
  leaveActiveClass: string
  leaveToClass: string
}

export interface TransitionClassProps {
  name?: string
  enterFromClass?: string
  enterActiveClass?: string
  enterToClass?: string
  appearFromClass?: string
  appearActiveClass?: string
  appearToClass?: string
  leaveFromClass?: string
  leaveActiveClass?: string
  leaveToClass?: string
  type?: 'transition' | 'animation'
  duration?: number | { enter: number, leave: number }
}

export function resolveClasses(p: TransitionClassProps): TransitionClasses {
  const name = p.name ?? 'v'
  const enterFromClass = p.enterFromClass ?? `${name}-enter-from`
  const enterActiveClass = p.enterActiveClass ?? `${name}-enter-active`
  const enterToClass = p.enterToClass ?? `${name}-enter-to`
  return {
    enterFromClass,
    enterActiveClass,
    enterToClass,
    appearFromClass: p.appearFromClass ?? enterFromClass,
    appearActiveClass: p.appearActiveClass ?? enterActiveClass,
    appearToClass: p.appearToClass ?? enterToClass,
    leaveFromClass: p.leaveFromClass ?? `${name}-leave-from`,
    leaveActiveClass: p.leaveActiveClass ?? `${name}-leave-active`,
    leaveToClass: p.leaveToClass ?? `${name}-leave-to`,
  }
}

type El = HTMLElement & { _endId?: number, _isLeaving?: boolean, _enterCancelled?: boolean }

export function addTransitionClass(el: Element, cls: string): void {
  for (const c of cls.split(/\s+/)) if (c) el.classList.add(c)
}

export function removeTransitionClass(el: Element, cls: string): void {
  for (const c of cls.split(/\s+/)) if (c) el.classList.remove(c)
}

export function nextFrame(cb: () => void): void {
  requestAnimationFrame(() => {
    requestAnimationFrame(cb)
  })
}

function toMs(s: string): number {
  if (s === 'auto')
    return 0
  return Number(s.slice(0, -1).replace(',', '.')) * 1e3
}

function getTimeout(delays: string[], durations: string[]): number {
  while (delays.length < durations.length) delays = delays.concat(delays)
  return Math.max(...durations.map((d, i) => toMs(d) + toMs(delays[i]!)))
}

function getTransitionInfo(el: Element, expectedType?: 'transition' | 'animation') {
  const styles = window.getComputedStyle(el)
  const get = (key: 'transitionDelay' | 'transitionDuration' | 'animationDelay' | 'animationDuration') => (styles[key] || '').split(', ')
  const transitionDelays = get('transitionDelay')
  const transitionDurations = get('transitionDuration')
  const transitionTimeout = getTimeout(transitionDelays, transitionDurations)
  const animationDelays = get('animationDelay')
  const animationDurations = get('animationDuration')
  const animationTimeout = getTimeout(animationDelays, animationDurations)
  let type: 'transition' | 'animation' | null = null
  let timeout = 0
  let propCount = 0
  if (expectedType === 'transition') {
    if (transitionTimeout > 0) {
      type = 'transition'
      timeout = transitionTimeout
      propCount = transitionDurations.length
    }
  }
  else if (expectedType === 'animation') {
    if (animationTimeout > 0) {
      type = 'animation'
      timeout = animationTimeout
      propCount = animationDurations.length
    }
  }
  else {
    timeout = Math.max(transitionTimeout, animationTimeout)
    type = timeout > 0 ? (transitionTimeout > animationTimeout ? 'transition' : 'animation') : null
    propCount = type ? (type === 'transition' ? transitionDurations.length : animationDurations.length) : 0
  }
  return { type, timeout, propCount }
}

let endId = 0
export function whenTransitionEnds(el: El, expectedType: 'transition' | 'animation' | undefined, explicitTimeout: number | undefined, resolve: () => void): void {
  const id = el._endId = ++endId
  const resolveIfNotStale = () => {
    if (id === el._endId)
      resolve()
  }
  if (explicitTimeout != null) {
    setTimeout(resolveIfNotStale, explicitTimeout)
    return
  }
  const { type, timeout, propCount } = getTransitionInfo(el, expectedType)
  if (!type) {
    resolve()
    return
  }
  const endEvent = `${type}end`
  let ended = 0
  const end = () => {
    el.removeEventListener(endEvent, onEnd)
    resolveIfNotStale()
  }
  const onEnd = (e: Event) => {
    if (e.target === el && ++ended >= propCount)
      end()
  }
  setTimeout(() => {
    if (ended < propCount)
      end()
  }, timeout + 1)
  el.addEventListener(endEvent, onEnd)
}

function forceReflow(el: Element): number {
  return el.ownerDocument.body.offsetHeight
}

function durations(d: TransitionClassProps['duration']): [number | undefined, number | undefined] {
  if (d == null)
    return [undefined, undefined]
  if (typeof d === 'object')
    return [d.enter, d.leave]
  return [d, d]
}

// The hooks below follow Vue's BaseTransition (runtime-core) combined with the
// DOM class hooks (runtime-dom resolveTransitionProps).

type HookEl = El & { _enterCb?: (cancelled?: boolean) => void, _leaveCb?: (cancelled?: boolean) => void }

function finishEnter(el: HookEl, c: TransitionClasses, appear: boolean, done?: () => void, isCancelled?: boolean): void {
  el._enterCancelled = isCancelled
  removeTransitionClass(el, appear ? c.appearToClass : c.enterToClass)
  removeTransitionClass(el, appear ? c.appearActiveClass : c.enterActiveClass)
  done?.()
}

/** beforeEnter / beforeAppear: called before the element is first painted. */
export function beforeEnter(el: HookEl, c: TransitionClasses, appear: boolean): void {
  addTransitionClass(el, appear ? c.appearFromClass : c.enterFromClass)
  addTransitionClass(el, appear ? c.appearActiveClass : c.enterActiveClass)
}

/** enter / appear: called once the element is in the document. */
export function enter(el: HookEl, c: TransitionClasses, p: TransitionClassProps, appear: boolean): void {
  let called = false
  const done = el._enterCb = (cancelled?: boolean) => {
    if (called)
      return
    called = true
    if (cancelled)
      finishEnter(el, c, appear, undefined, true) // onEnterCancelled
    el._enterCb = undefined
  }
  const resolve = () => finishEnter(el, c, appear, done)
  nextFrame(() => {
    removeTransitionClass(el, appear ? c.appearFromClass : c.enterFromClass)
    addTransitionClass(el, appear ? c.appearToClass : c.enterToClass)
    whenTransitionEnds(el, p.type, durations(p.duration)[0], resolve)
  })
}

/** leave: `remove` takes the element out of the document when the leave ends. */
export function leave(el: HookEl, c: TransitionClasses, p: TransitionClassProps, remove: () => void): void {
  if (el._enterCb)
    el._enterCb(true)
  let called = false
  const done = el._leaveCb = () => {
    if (called)
      return
    called = true
    remove()
    el._leaveCb = undefined
  }
  el._isLeaving = true
  const resolve = () => {
    el._isLeaving = false
    removeTransitionClass(el, c.leaveFromClass)
    removeTransitionClass(el, c.leaveToClass)
    removeTransitionClass(el, c.leaveActiveClass)
    done()
  }
  addTransitionClass(el, c.leaveFromClass)
  if (!el._enterCancelled) {
    forceReflow(el)
    addTransitionClass(el, c.leaveActiveClass)
  }
  else {
    addTransitionClass(el, c.leaveActiveClass)
    forceReflow(el)
  }
  nextFrame(() => {
    if (!el._isLeaving)
      return
    removeTransitionClass(el, c.leaveFromClass)
    addTransitionClass(el, c.leaveToClass)
    whenTransitionEnds(el, p.type, durations(p.duration)[1], resolve)
  })
}

/** Early removal of a leaving element whose key re-enters (Vue: leavingVNode leaveCb()). */
export function forceLeaveDone(el: HookEl): void {
  el._leaveCb?.()
}
