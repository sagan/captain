// 复刻 reka-ui Dialog 依赖的几项 DOM 副作用（项目未安装 @radix-ui/react-dialog）。

// ---- body pointer-events 锁（reka shared/bodyPointerEvents）
const pointerLocks = new WeakMap<Document, { owners: Set<unknown>, original: string }>()

export function acquireBodyPointerEvents(doc: Document, owner: unknown) {
  let lock = pointerLocks.get(doc)
  if (!lock) {
    lock = { owners: new Set(), original: doc.body.style.pointerEvents }
    pointerLocks.set(doc, lock)
  }
  lock.owners.add(owner)
  doc.body.style.pointerEvents = 'none'
}

export function releaseBodyPointerEvents(doc: Document, owner: unknown) {
  const lock = pointerLocks.get(doc)
  if (!lock?.owners.delete(owner))
    return
  if (lock.owners.size === 0) {
    pointerLocks.delete(doc)
    doc.body.style.pointerEvents = lock.original
  }
}

// ---- body 滚动锁（reka shared/useBodyScrollLock，默认 scrollBody 配置）
const scrollOwner = Symbol('useBodyScrollLock')
let scrollLockCount = 0
let initialOverflow: string | undefined

export function lockBodyScroll(): () => void {
  scrollLockCount++
  if (scrollLockCount === 1) {
    const body = document.body
    if (initialOverflow === undefined)
      initialOverflow = body.style.overflow
    const verticalScrollbarWidth = window.innerWidth - document.documentElement.clientWidth
    if (verticalScrollbarWidth > 0) {
      body.style.paddingRight = `${verticalScrollbarWidth}px`
      body.style.marginRight = '0px'
      document.documentElement.style.setProperty('--scrollbar-width', `${verticalScrollbarWidth}px`)
      body.style.overflow = 'hidden'
    }
    queueMicrotask(() => {
      if (scrollLockCount === 0)
        return
      acquireBodyPointerEvents(document, scrollOwner)
      document.body.style.overflow = 'hidden'
    })
  }
  let released = false
  return () => {
    if (released)
      return
    released = true
    scrollLockCount--
    if (scrollLockCount === 0) {
      const body = document.body
      body.style.paddingRight = ''
      body.style.marginRight = ''
      releaseBodyPointerEvents(document, scrollOwner)
      document.documentElement.style.removeProperty('--scrollbar-width')
      body.style.overflow = initialOverflow ?? ''
      initialOverflow = undefined
    }
  }
}

// ---- aria-hidden（aria-hidden 包的 hideOthers，默认 marker data-aria-hidden）
const counterMap = new WeakMap<Element, number>()
const uncontrolled = new WeakSet<Element>()

export function hideOthers(target: Element): () => void {
  const parent = target.ownerDocument.body
  const keep = new Set<Element>([target, ...Array.from(parent.querySelectorAll('[aria-live], script'))])
  const ancestors = new Set<Element>()
  for (const el of keep) {
    let node: Element | null = el
    while (node && !ancestors.has(node)) {
      ancestors.add(node)
      node = node.parentElement
    }
  }
  const hidden: Element[] = []
  const walk = (node: Element) => {
    if (keep.has(node))
      return
    for (const child of Array.from(node.children)) {
      if (ancestors.has(child)) {
        walk(child)
        continue
      }
      const attr = child.getAttribute('aria-hidden')
      const alreadyHidden = attr !== null && attr !== 'false'
      const count = (counterMap.get(child) ?? 0) + 1
      counterMap.set(child, count)
      hidden.push(child)
      if (count === 1) {
        if (alreadyHidden)
          uncontrolled.add(child)
        child.setAttribute('data-aria-hidden', 'true')
      }
      if (!alreadyHidden)
        child.setAttribute('aria-hidden', 'true')
    }
  }
  walk(parent)
  return () => {
    for (const node of hidden) {
      const count = (counterMap.get(node) ?? 1) - 1
      counterMap.set(node, count)
      if (!count) {
        if (!uncontrolled.has(node))
          node.removeAttribute('aria-hidden')
        uncontrolled.delete(node)
        node.removeAttribute('data-aria-hidden')
      }
    }
  }
}

// ---- 焦点工具（reka FocusScope/utils）
export function getTabbableCandidates(container: HTMLElement): HTMLElement[] {
  const nodes: HTMLElement[] = []
  const walker = document.createTreeWalker(container, NodeFilter.SHOW_ELEMENT, {
    acceptNode: (node) => {
      const el = node as HTMLElement & { disabled?: boolean }
      const isHiddenInput = el.tagName === 'INPUT' && (el as HTMLInputElement).type === 'hidden'
      if (el.disabled || el.hidden || isHiddenInput)
        return NodeFilter.FILTER_SKIP
      return el.tabIndex >= 0 ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_SKIP
    },
  })
  while (walker.nextNode())
    nodes.push(walker.currentNode as HTMLElement)
  return nodes.sort((a, b) => {
    if (a.tabIndex === b.tabIndex)
      return 0
    if (a.tabIndex === 0)
      return 1
    if (b.tabIndex === 0)
      return -1
    return a.tabIndex - b.tabIndex
  })
}

function isHidden(node: HTMLElement, upTo: HTMLElement) {
  if (getComputedStyle(node).visibility === 'hidden')
    return true
  let cur: HTMLElement | null = node
  while (cur) {
    if (cur === upTo)
      return false
    if (getComputedStyle(cur).display === 'none')
      return true
    cur = cur.parentElement
  }
  return false
}

export function getTabbableEdges(container: HTMLElement) {
  const candidates = getTabbableCandidates(container)
  const first = candidates.find(el => !isHidden(el, container))
  const last = [...candidates].reverse().find(el => !isHidden(el, container))
  return [first, last] as const
}

export function focusElement(element: HTMLElement | null | undefined, select = false) {
  if (element && element.focus) {
    const previous = document.activeElement
    element.focus({ preventScroll: true })
    if (element !== previous && select && element instanceof HTMLInputElement && 'select' in element)
      element.select()
  }
}

export function focusFirst(candidates: HTMLElement[]) {
  const previous = document.activeElement
  for (const candidate of candidates) {
    focusElement(candidate, true)
    if (document.activeElement !== previous)
      return true
  }
  return false
}

// ---- 层级栈：只让最上层响应 Escape
export const layerStack: HTMLElement[] = []

// ---- Presence：退出动画结束后再卸载（reka Presence/usePresence 的简化）
export function getAnimationName(el: HTMLElement | null) {
  return el ? getComputedStyle(el).animationName || 'none' : 'none'
}
