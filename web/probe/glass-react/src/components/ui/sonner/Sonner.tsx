import type { CSSProperties } from 'react'
import type { ToasterProps } from 'sonner'
import { Icon } from '@iconify/react'
import { useCallback, useRef } from 'react'
import { Toaster as Sonner } from 'sonner'
import { cn } from '@/lib/utils'

// @iconify/vue 把自身的 iconify 类追加在传入 class 之后，@iconify/react 则放在前面；
// 挂载后按 Vue 的顺序重写一次 class（React 只在 className 变化时才会再写）。
function vueClassOrder(node: SVGSVGElement | null) {
  if (!node)
    return
  const classes = (node.getAttribute('class') ?? '').split(' ')
  const own = classes.filter(c => c === 'iconify' || c.startsWith('iconify--'))
  const user = classes.filter(c => c && !own.includes(c))
  node.setAttribute('class', [...user, ...own].join(' '))
}

function ToastIcon({ icon, className }: { icon: string, className: string }) {
  return <Icon icon={icon} className={className} ref={vueClassOrder} />
}

const toasterStyle = {
  '--normal-bg': 'var(--popover)',
  '--normal-text': 'var(--popover-foreground)',
  '--normal-border': 'var(--border)',
  '--border-radius': 'var(--radius)',
} as CSSProperties

// sonner 与 vue-sonner 在若干恒定特性上不同，React 只在创建元素时写入它们，之后不会再改，
// 因此挂载后（以及新列表/按钮出现时）按 vue-sonner 的输出改写一次即可。
function alignToVueSonner(section: HTMLElement, theme: string, richColors: boolean, closeButtonPosition: string) {
  section.removeAttribute('data-react-aria-top-layer')
  for (const ol of section.querySelectorAll<HTMLElement>(':scope > ol[data-sonner-toaster]')) {
    ol.setAttribute('data-sonner-toaster', '')
    ol.setAttribute('data-theme', theme)
    ol.setAttribute('data-rich-colors', String(richColors))
  }
  for (const btn of section.querySelectorAll<HTMLElement>('[data-close-button]:not([data-close-button-position])'))
    btn.setAttribute('data-close-button-position', closeButtonPosition)
  // vue-sonner 按 toast.position 分组计算 toasts.length，未指定 position 的 toast 得到 0，
  // 所以 --z-index 是 0 - index（层叠顺序与 sonner 相同，只是数值不同）。
  for (const li of section.querySelectorAll<HTMLElement>('li[data-sonner-toast]')) {
    const want = String(0 - Number(li.dataset.index ?? 0))
    if (li.style.getPropertyValue('--z-index') !== want)
      li.style.setProperty('--z-index', want)
  }
}

function useVueSonnerAttrs(theme: string, richColors: boolean) {
  const observer = useRef<MutationObserver | null>(null)
  return useCallback((section: HTMLElement | null) => {
    observer.current?.disconnect()
    observer.current = null
    if (!section)
      return
    const sync = () => alignToVueSonner(section, theme, richColors, 'top-left')
    sync()
    observer.current = new MutationObserver(sync)
    observer.current.observe(section, { childList: true, subtree: true, attributes: true, attributeFilter: ['style', 'data-index'] })
  }, [theme, richColors])
}

export function Toaster({ className, style, icons, invert = false, ...props }: ToasterProps) {
  const ref = useVueSonnerAttrs(props.theme ?? 'light', props.richColors ?? false)
  return (
    <Sonner
      ref={ref}
      // vue-sonner 的 invert 默认 false，toast 上会输出 data-invert="false"
      invert={invert}
      className={cn('toaster group', className)}
      style={{ ...toasterStyle, ...style }}
      icons={{
        success: <ToastIcon icon="lucide:circle-check" className="size-4" />,
        info: <ToastIcon icon="lucide:info" className="size-4" />,
        warning: <ToastIcon icon="lucide:triangle-alert" className="size-4" />,
        error: <ToastIcon icon="lucide:octagon-x" className="size-4" />,
        // vue 版的 loading 插槽外包了一层 <div>
        loading: <div><ToastIcon icon="lucide:loader-2" className="size-4 animate-spin" /></div>,
        close: <ToastIcon icon="lucide:x" className="size-4" />,
        ...icons,
      }}
      {...props}
    />
  )
}
