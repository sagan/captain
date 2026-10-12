import { Icon } from '@iconify/react'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'

export interface BackTopProps {
  visibilityHeight?: number
  onScrolled?: (scrolled: boolean) => void
}

const ENTER_FROM = 'opacity-0 translate-y-2'
const ENTER_ACTIVE = 'transition-all duration-200'
const ENTER_TO = 'opacity-100 translate-y-0'
const LEAVE_FROM = 'opacity-100 translate-y-0'
const LEAVE_ACTIVE = 'transition-all duration-200'
const LEAVE_TO = 'opacity-0 translate-y-2'
const HIDDEN_STYLE = { display: 'none' }

function addClass(el: HTMLElement, cls: string) {
  cls.split(/\s+/).forEach(c => c && el.classList.add(c))
}
function removeClass(el: HTMLElement, cls: string) {
  cls.split(/\s+/).forEach(c => c && el.classList.remove(c))
}
function nextFrame(cb: () => void) {
  let id = requestAnimationFrame(() => {
    id = requestAnimationFrame(cb)
  })
  return () => cancelAnimationFrame(id)
}
function toMs(s: string) {
  return s === 'auto' ? 0 : Number(s.slice(0, -1).replace(',', '.')) * 1000
}
// 与 Vue <Transition> 的 whenTransitionEnds 相同：按计算样式的时长/属性数收尾
function whenTransitionEnds(el: HTMLElement, done: () => void) {
  const styles = getComputedStyle(el)
  const durations = styles.transitionDuration.split(', ')
  const delays = styles.transitionDelay.split(', ')
  let timeout = 0
  durations.forEach((d, i) => {
    timeout = Math.max(timeout, toMs(d) + toMs(delays[i % delays.length] ?? '0s'))
  })
  if (!timeout) {
    done()
    return () => {}
  }
  const propCount = durations.length
  let ended = 0
  let finished = false
  const finish = () => {
    if (finished)
      return
    finished = true
    el.removeEventListener('transitionend', onEnd)
    clearTimeout(timer)
    done()
  }
  const onEnd = (e: TransitionEvent) => {
    if (e.target === el && ++ended >= propCount)
      finish()
  }
  const timer = setTimeout(finish, timeout + 1)
  el.addEventListener('transitionend', onEnd)
  return () => {
    finished = true
    el.removeEventListener('transitionend', onEnd)
    clearTimeout(timer)
  }
}

// 替代 <Transition> + v-show：初始 display:none，进出场时按 Vue 的顺序
// 直接增删同一组过渡类（类追加在原 class 之后），结束后清除。
export function BackTop({ visibilityHeight = 320, onScrolled }: BackTopProps) {
  const [show, setShow] = useState(false)
  const el = useRef<HTMLButtonElement | null>(null)
  const first = useRef(true)
  const latest = useRef({ visibilityHeight, onScrolled })
  latest.current = { visibilityHeight, onScrolled }

  useEffect(() => {
    function handleScroll() {
      setShow(window.scrollY > latest.current.visibilityHeight)
      latest.current.onScrolled?.(window.scrollY > 1)
    }
    window.addEventListener('scroll', handleScroll, { passive: true })
    handleScroll()
    return () => window.removeEventListener('scroll', handleScroll)
  }, [])

  useLayoutEffect(() => {
    const node = el.current
    if (!node)
      return
    if (first.current) {
      first.current = false
      if (!show)
        return
    }
    const cleanups: Array<() => void> = []
    if (show) {
      addClass(node, ENTER_FROM)
      addClass(node, ENTER_ACTIVE)
      node.style.display = ''
      cleanups.push(nextFrame(() => {
        removeClass(node, ENTER_FROM)
        addClass(node, ENTER_TO)
        cleanups.push(whenTransitionEnds(node, () => {
          removeClass(node, ENTER_TO)
          removeClass(node, ENTER_ACTIVE)
        }))
      }))
    }
    else {
      addClass(node, LEAVE_FROM)
      void node.offsetHeight
      addClass(node, LEAVE_ACTIVE)
      cleanups.push(nextFrame(() => {
        removeClass(node, LEAVE_FROM)
        addClass(node, LEAVE_TO)
        cleanups.push(whenTransitionEnds(node, () => {
          node.style.display = 'none'
          removeClass(node, LEAVE_FROM)
          removeClass(node, LEAVE_TO)
          removeClass(node, LEAVE_ACTIVE)
        }))
      }))
    }
    return () => {
      // 被下一次切换打断：与 Vue 的 enterCancelled/leaveCancelled 一样清掉过渡类
      cleanups.forEach(fn => fn())
      for (const c of [ENTER_FROM, ENTER_ACTIVE, ENTER_TO, LEAVE_FROM, LEAVE_TO])
        removeClass(node, c)
      if (!show)
        node.style.display = 'none'
    }
  }, [show])

  function scrollToTop() {
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  return (
    <button
      ref={el}
      style={HIDDEN_STYLE}
      className="fixed bottom-16 right-3 z-[60] flex size-9 items-center justify-center rounded-full border bg-background/80 text-foreground shadow-sm backdrop-blur-md transition-colors hover:bg-accent md:bottom-8 md:right-8 md:size-10"
      aria-label="返回顶部"
      onClick={scrollToTop}
    >
      <Icon icon="tabler:arrow-up" width={18} height={18} />
    </button>
  )
}
