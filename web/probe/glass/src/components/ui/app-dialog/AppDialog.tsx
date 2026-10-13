import type { KeyboardEvent, ReactNode } from 'react'
import { Icon } from '@iconify/react'
import { t } from '@shared/i18n'
import { useEffect, useId, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import {
  acquireBodyPointerEvents,
  focusElement,
  focusFirst,
  getAnimationName,
  getTabbableCandidates,
  getTabbableEdges,
  hideOthers,
  layerStack,
  lockBodyScroll,
  releaseBodyPointerEvents,
} from './dialog-utils'

export interface AppDialogProps {
  open: boolean
  title: string
  description?: string
  contentClass?: string
  /** 对应 Vue 的 update:open */
  onOpenChange?: (open: boolean) => void
  children?: ReactNode
}

const OVERLAY_STYLE = { pointerEvents: 'auto' } as const

// 遮罩带 tw-animate 退出动画：data-state=closed 后若动画名改变则等 animationend 再卸载（同 reka Presence）。
function usePresence(present: boolean) {
  const [mounted, setMounted] = useState(present)
  const [node, setNode] = useState<HTMLDivElement | null>(null)
  const prevAnimation = useRef('none')

  useEffect(() => {
    if (!node)
      return
    prevAnimation.current = getAnimationName(node)
    const onStart = (e: AnimationEvent) => {
      if (e.target === node)
        prevAnimation.current = getAnimationName(node)
    }
    node.addEventListener('animationstart', onStart)
    return () => node.removeEventListener('animationstart', onStart)
  }, [node])

  useLayoutEffect(() => {
    if (present) {
      setMounted(true)
      return
    }
    const current = getAnimationName(node)
    if (!node || current === 'none' || getComputedStyle(node).display === 'none' || prevAnimation.current === current) {
      setMounted(false)
      return
    }
    const onEnd = (e: AnimationEvent) => {
      if (e.target === node && getAnimationName(node).includes(CSS.escape(e.animationName)))
        setMounted(false)
    }
    node.addEventListener('animationend', onEnd)
    node.addEventListener('animationcancel', onEnd)
    return () => {
      node.removeEventListener('animationend', onEnd)
      node.removeEventListener('animationcancel', onEnd)
    }
    // node 变化不应重新判断退出
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [present])

  return { isPresent: present || mounted, ref: setNode }
}

export function AppDialog({ open, title, description, contentClass, onOpenChange, children }: AppDialogProps) {
  const [isMounted, setIsMounted] = useState(false)
  useEffect(() => setIsMounted(true), [])

  const reactId = useId()
  const titleId = `reka-dialog-title-${reactId}`
  const descriptionId = `reka-dialog-description-${reactId}`
  const state = open ? 'open' : 'closed'

  const overlay = usePresence(open)
  const contentRef = useRef<HTMLDivElement | null>(null)
  const onOpenChangeRef = useRef(onOpenChange)
  onOpenChangeRef.current = onOpenChange

  // 遮罩挂载期间锁定 body 滚动（含退出动画）
  const overlayPresent = overlay.isPresent
  useLayoutEffect(() => {
    if (!overlayPresent)
      return
    return lockBodyScroll()
  }, [overlayPresent])

  // 内容挂载期间：aria-hidden 其他节点、禁用 body 指针事件、焦点陷阱、Esc / 外部点击关闭
  useLayoutEffect(() => {
    const container = contentRef.current
    if (!open || !container)
      return
    const doc = container.ownerDocument
    const close = () => onOpenChangeRef.current?.(false)

    const undoHide = hideOthers(container)
    acquireBodyPointerEvents(doc, container)
    layerStack.push(container)

    const previouslyFocused = doc.activeElement as HTMLElement | null
    let lastFocused: HTMLElement | null = null
    if (!container.contains(previouslyFocused)) {
      focusFirst(getTabbableCandidates(container))
      if (doc.activeElement === previouslyFocused)
        focusElement(container)
    }

    const onFocusIn = (e: FocusEvent) => {
      const target = e.target as HTMLElement | null
      if (target && container.contains(target))
        lastFocused = target
      else
        focusElement(lastFocused, true)
    }
    const onFocusOut = (e: FocusEvent) => {
      const related = e.relatedTarget as HTMLElement | null
      if (related === null)
        return
      if (!container.contains(related))
        focusElement(lastFocused, true)
    }
    const onKeyDown = (e: globalThis.KeyboardEvent) => {
      if (e.key === 'Escape' && layerStack[layerStack.length - 1] === container)
        close()
    }
    let clickHandler: (() => void) | null = null
    const onPointerDown = (e: PointerEvent) => {
      const target = e.target as Element | null
      if (!target)
        return
      if (container.contains(target)) {
        if (clickHandler)
          doc.removeEventListener('click', clickHandler)
        return
      }
      const isRightClick = e.button === 2 || (e.button === 0 && e.ctrlKey)
      if (isRightClick)
        return
      if (e.pointerType === 'touch') {
        if (clickHandler)
          doc.removeEventListener('click', clickHandler)
        clickHandler = close
        doc.addEventListener('click', clickHandler, { once: true })
      }
      else {
        close()
      }
    }
    doc.addEventListener('focusin', onFocusIn)
    doc.addEventListener('focusout', onFocusOut)
    window.addEventListener('keydown', onKeyDown)
    const timer = window.setTimeout(() => doc.addEventListener('pointerdown', onPointerDown), 0)

    return () => {
      window.clearTimeout(timer)
      doc.removeEventListener('pointerdown', onPointerDown)
      if (clickHandler)
        doc.removeEventListener('click', clickHandler)
      doc.removeEventListener('focusin', onFocusIn)
      doc.removeEventListener('focusout', onFocusOut)
      window.removeEventListener('keydown', onKeyDown)
      const idx = layerStack.indexOf(container)
      if (idx >= 0)
        layerStack.splice(idx, 1)
      releaseBodyPointerEvents(doc, container)
      undoHide()
      setTimeout(() => focusElement(previouslyFocused ?? doc.body, true), 0)
    }
  }, [open, isMounted])

  // Tab 循环（reka FocusScope loop + trapped）
  function handleKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    const isTabKey = e.key === 'Tab' && !e.altKey && !e.ctrlKey && !e.metaKey
    const focused = document.activeElement
    if (!isTabKey || !focused)
      return
    const container = e.currentTarget
    const [first, last] = getTabbableEdges(container)
    if (!(first && last)) {
      if (focused === container)
        e.preventDefault()
    }
    else if (!e.shiftKey && focused === last) {
      e.preventDefault()
      focusElement(first, true)
    }
    else if (e.shiftKey && focused === first) {
      e.preventDefault()
      focusElement(last, true)
    }
  }

  if (!isMounted)
    return null

  return createPortal(
    <>
      {overlay.isPresent && (
        <div
          ref={overlay.ref}
          data-state={state}
          style={OVERLAY_STYLE}
          className="fixed inset-0 z-100 bg-black/45 backdrop-blur-[2px] data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0"
          onPointerDown={(e) => {
            if (e.button === 0 && e.target === e.currentTarget)
              e.preventDefault()
          }}
        />
      )}
      {open && (
        <div
          ref={contentRef}
          tabIndex={-1}
          data-dismissable-layer=""
          style={OVERLAY_STYLE}
          id=""
          role="dialog"
          aria-describedby={descriptionId}
          aria-labelledby={titleId}
          data-state={state}
          className={`fixed left-1/2 top-1/2 z-101 flex max-h-[calc(100dvh-2rem)] w-[calc(100vw-2rem)] max-w-5xl -translate-x-1/2 -translate-y-1/2 flex-col overflow-hidden rounded-lg border border-border/70 bg-card/95 text-card-foreground shadow-2xl backdrop-blur-xl focus:outline-none${contentClass ? ` ${contentClass}` : ''}`}
          onKeyDown={handleKeyDown}
        >
          <div className="flex items-start gap-3 border-b border-border/60 px-4 py-3 sm:px-5">
            <div className="min-w-0 flex-1">
              <h2 id={titleId} className="truncate text-base font-semibold">
                {title}
              </h2>
              <p id={descriptionId} className={`mt-0.5 text-xs text-muted-foreground${!description ? ' sr-only' : ''}`}>
                {description || title}
              </p>
            </div>
            <button
              type="button"
              className="inline-flex size-8 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-background hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              aria-label={t('关闭')}
              onClick={() => onOpenChangeRef.current?.(false)}
            >
              <Icon icon="tabler:x" width="17" height="17" />
            </button>
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto p-4 sm:p-5">
            {children}
          </div>
        </div>
      )}
    </>,
    document.body,
  )
}
