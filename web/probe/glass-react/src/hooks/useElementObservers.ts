// Ports of the @vueuse/core helpers the globes use: useElementSize
// (content-box, synchronous read on mount, then ResizeObserver),
// useElementVisibility (starts false, IntersectionObserver) and
// useDocumentVisibility.
import { useEffect, useLayoutEffect, useState } from 'react'
import type { RefObject } from 'react'

export function useElementSize(ref: RefObject<HTMLElement | null>) {
  const [size, setSize] = useState({ width: 0, height: 0 })
  useLayoutEffect(() => {
    const el = ref.current
    if (!el)
      return
    const cs = window.getComputedStyle(el)
    const padX = Number.parseFloat(cs.paddingLeft) + Number.parseFloat(cs.paddingRight)
    const padY = Number.parseFloat(cs.paddingTop) + Number.parseFloat(cs.paddingBottom)
    const bdX = Number.parseFloat(cs.borderLeftWidth) + Number.parseFloat(cs.borderRightWidth)
    const bdY = Number.parseFloat(cs.borderTopWidth) + Number.parseFloat(cs.borderBottomWidth)
    setSize({ width: el.offsetWidth - padX - bdX, height: el.offsetHeight - padY - bdY })
    const observer = new ResizeObserver(([entry]) => {
      const box = entry!.contentBoxSize
      if (box) {
        const list = Array.isArray(box) ? box : [box]
        setSize({
          width: list.reduce((acc, { inlineSize }) => acc + inlineSize, 0),
          height: list.reduce((acc, { blockSize }) => acc + blockSize, 0),
        })
      }
      else {
        setSize({ width: entry!.contentRect.width, height: entry!.contentRect.height })
      }
    })
    observer.observe(el)
    return () => observer.disconnect()
  }, [ref])
  return size
}

export function useElementVisibility(ref: RefObject<HTMLElement | null>): boolean {
  const [visible, setVisible] = useState(false)
  useEffect(() => {
    const el = ref.current
    if (!el || typeof IntersectionObserver === 'undefined')
      return
    const observer = new IntersectionObserver((entries) => {
      setVisible((current) => {
        let isIntersecting = current
        let latestTime = 0
        for (const entry of entries) {
          if (entry.time >= latestTime) {
            latestTime = entry.time
            isIntersecting = entry.isIntersecting
          }
        }
        return isIntersecting
      })
    }, { threshold: 0 })
    observer.observe(el)
    return () => observer.disconnect()
  }, [ref])
  return visible
}

export function useDocumentVisibility(): DocumentVisibilityState {
  const [state, setState] = useState(() => document.visibilityState)
  useEffect(() => {
    const update = () => setState(document.visibilityState)
    document.addEventListener('visibilitychange', update, { passive: true })
    return () => document.removeEventListener('visibilitychange', update)
  }, [])
  return state
}
