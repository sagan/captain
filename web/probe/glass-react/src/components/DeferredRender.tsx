// React counterpart of DeferredRender.vue: renders its children once the
// placeholder is near the viewport (or after an idle delay), keeping the
// placeholder's height until then. Activation follows the enclosing
// kept-alive view like Vue's onActivated/onDeactivated.
import type { HTMLAttributes, ReactNode } from 'react'
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { useActivation } from '@/lib/router'

const VIEWPORT_MARGIN = 700

export interface DeferredRenderProps extends HTMLAttributes<HTMLDivElement> {
  enabled?: boolean
  idleDelay?: number
  minHeight?: number
  children?: ReactNode
}

export default function DeferredRender({ enabled = false, idleDelay = 0, minHeight = 260, children, className, style, ...attrs }: DeferredRenderProps) {
  const root = useRef<HTMLDivElement | null>(null)
  const [rendered, setRendered] = useState(!enabled)
  const renderedRef = useRef(rendered)
  renderedRef.current = rendered
  const props = useRef({ enabled, idleDelay })
  props.current = { enabled, idleDelay }
  const handles = useRef<{ observer: IntersectionObserver | null, frame: number | null, timer: number | null, idle: number | null }>({ observer: null, frame: null, timer: null, idle: null })

  const isNearViewport = useCallback((): boolean => {
    if (!root.current)
      return true
    const rect = root.current.getBoundingClientRect()
    return rect.bottom >= -VIEWPORT_MARGIN && rect.top <= window.innerHeight + VIEWPORT_MARGIN
  }, [])

  const reveal = useCallback(() => {
    renderedRef.current = true
    setRendered(true)
  }, [])

  const stopObserving = useCallback(() => {
    const h = handles.current
    h.observer?.disconnect()
    h.observer = null
    window.removeEventListener('scroll', scheduleFallbackCheck)
    window.removeEventListener('resize', scheduleFallbackCheck)
    if (h.frame != null)
      window.cancelAnimationFrame(h.frame)
    if (h.timer != null)
      window.clearTimeout(h.timer)
    if (h.idle != null)
      window.clearTimeout(h.idle)
    h.frame = null
    h.timer = null
    h.idle = null
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function revealWhenNear(): void {
    handles.current.frame = null
    handles.current.timer = null
    if (!props.current.enabled || renderedRef.current || !isNearViewport())
      return
    reveal()
    stopObserving()
  }

  function revealDeferred(): void {
    if (!props.current.enabled || renderedRef.current)
      return
    reveal()
    stopObserving()
  }

  function scheduleFallbackCheck(): void {
    const h = handles.current
    if (h.frame != null || h.timer != null)
      return
    if (typeof window.requestAnimationFrame === 'function')
      h.frame = window.requestAnimationFrame(revealWhenNear)
    else
      h.timer = window.setTimeout(revealWhenNear, 16)
  }

  const startObserving = useCallback(() => {
    stopObserving()
    if (renderedRef.current || !props.current.enabled || !root.current)
      return
    if (isNearViewport()) {
      reveal()
      return
    }
    if (handles.current.idle == null && props.current.idleDelay > 0)
      handles.current.idle = window.setTimeout(revealDeferred, props.current.idleDelay)
    if (typeof IntersectionObserver === 'undefined') {
      window.addEventListener('scroll', scheduleFallbackCheck, { passive: true })
      window.addEventListener('resize', scheduleFallbackCheck, { passive: true })
      scheduleFallbackCheck()
      return
    }
    const observer = new IntersectionObserver((entries) => {
      if (!entries.some(entry => entry.isIntersecting))
        return
      reveal()
      stopObserving()
    }, { rootMargin: `${VIEWPORT_MARGIN}px 0px` })
    handles.current.observer = observer
    observer.observe(root.current)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // onMounted(startObserving) runs before paint in Vue.
  useLayoutEffect(() => {
    startObserving()
    return stopObserving
  }, [startObserving, stopObserving])

  // onActivated / onDeactivated (the mount-time activation is covered above).
  const mountedOnce = useRef(false)
  useActivation(() => {
    if (mountedOnce.current)
      startObserving()
    mountedOnce.current = true
  }, stopObserving)

  // watch(() => props.enabled): not immediate.
  const prevEnabled = useRef(enabled)
  useEffect(() => {
    if (prevEnabled.current === enabled)
      return
    prevEnabled.current = enabled
    stopObserving()
    if (!enabled) {
      reveal()
      return
    }
    let cancelled = false
    queueMicrotask(() => {
      if (cancelled)
        return
      const near = isNearViewport()
      renderedRef.current = near
      setRendered(near)
      queueMicrotask(() => {
        if (!cancelled)
          startObserving()
      })
    })
    return () => {
      cancelled = true
    }
  }, [enabled, isNearViewport, reveal, startObserving, stopObserving])

  return (
    <div
      {...attrs}
      ref={root}
      className={className ? `min-w-0 ${className}` : 'min-w-0'}
      style={rendered ? style : { ...style, minHeight: `${minHeight}px` }}
    >
      {rendered ? children : null}
    </div>
  )
}
