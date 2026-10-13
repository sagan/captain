// React counterpart of vue-echarts 8.3.2 <VChart :option autoresize /> as the
// glass theme uses it: same <x-vue-echarts class="echarts"> root and
// .echarts-host child, same stylesheet, the same deferred first setOption
// after a resize when autoresize is on, and the same per-update merge plan
// (vendored planUpdate), so ECharts receives identical calls.
import type { CSSProperties } from 'react'
import { useLayoutEffect, useRef } from 'react'
import { init, throttle } from 'echarts/core'
import type { EChartsCoreOption, EChartsType } from 'echarts/core'
import { planUpdate } from '@/lib/vendor/vue-echarts-update.js'

const TAG_NAME = 'x-vue-echarts'
const STYLE = 'x-vue-echarts{display:block;width:100%;height:100%;min-width:0;min-height:0;}\nx-vue-echarts>.echarts-host{display:block;width:100%;height:100%;min-width:0;border-radius:inherit;}\nx-vue-echarts>.echarts-host>:first-child,x-vue-echarts>.echarts-host>:first-child>canvas{border-radius:inherit;}\n'

// vue-echarts injects its stylesheet once at module load.
if (typeof document !== 'undefined') {
  if (Array.isArray(document.adoptedStyleSheets) && 'replaceSync' in CSSStyleSheet.prototype) {
    const sheet = new CSSStyleSheet()
    sheet.replaceSync(STYLE)
    document.adoptedStyleSheets = [...document.adoptedStyleSheets, sheet]
  }
  else {
    const styleEl = document.createElement('style')
    styleEl.textContent = STYLE
    document.head.appendChild(styleEl)
  }
}

// vue-echarts defines the custom element (its lifecycle only defers disposal).
if (typeof customElements !== 'undefined' && !customElements.get(TAG_NAME)) {
  class ECElement extends HTMLElement {}
  customElements.define(TAG_NAME, ECElement)
}

declare module 'react' {
  // eslint-disable-next-line ts/no-namespace
  namespace JSX {
    interface IntrinsicElements {
      'x-vue-echarts': import('react').HTMLAttributes<HTMLElement> & { class?: string }
    }
  }
}

type Signature = ReturnType<typeof planUpdate>['signature']

function hasZeroDimension(width: number, height: number): boolean {
  return width === 0 || height === 0
}

export interface EChartProps {
  option: EChartsCoreOption
  autoresize?: boolean
  className?: string
  style?: CSSProperties
}

export default function EChart({ option, autoresize = false, className, style }: EChartProps) {
  const hostRef = useRef<HTMLDivElement | null>(null)
  const chartRef = useRef<EChartsType | null>(null)
  const state = useRef({ lastSignature: undefined as Signature | null | undefined, initDeferred: false, lastOption: undefined as EChartsCoreOption | undefined })
  const optionRef = useRef(option)
  optionRef.current = option

  function applyOption(instance: EChartsType, opt: EChartsCoreOption) {
    const s = state.current
    s.initDeferred = false
    const planned = planUpdate(s.lastSignature ?? undefined, opt)
    const updateOptions = !planned.signature.hasAction && s.lastSignature === null
      ? { ...planned.plan, notMerge: true }
      : planned.plan
    s.lastSignature = null
    instance.setOption(opt, updateOptions)
    s.lastSignature = planned.signature
    s.lastOption = opt
  }

  // Mount: init, then (autoresize) resize and apply on the next tick. Vue's
  // onMounted runs right after the DOM is attached, so use a layout effect.
  useLayoutEffect(() => {
    const host = hostRef.current!
    const instance = init(host)
    chartRef.current = instance
    let cancelled = false
    const commit = () => {
      if (optionRef.current)
        applyOption(instance, optionRef.current)
    }
    if (autoresize) {
      state.current.initDeferred = true
      queueMicrotask(() => {
        if (cancelled || chartRef.current !== instance)
          return
        if (!hasZeroDimension(host.offsetWidth, host.offsetHeight))
          instance.resize()
        if (state.current.initDeferred)
          commit()
      })
    }
    else {
      commit()
    }

    let stopResize: (() => void) | undefined
    if (autoresize) {
      let observedWidth = instance.getWidth()
      let observedHeight = instance.getHeight()
      const resize = () => {
        if (hasZeroDimension(observedWidth, observedHeight))
          return
        const width = instance.getWidth()
        const height = instance.getHeight()
        if (host.clientWidth === width && host.clientHeight === height)
          return
        instance.getZr().resize()
        if (width === instance.getWidth() && height === instance.getHeight())
          return
        instance.resize()
      }
      const throttled = throttle(resize, 100) as ((() => void) & { clear?: () => void })
      const observer = new ResizeObserver(([entry]) => {
        observedWidth = entry!.contentRect.width
        observedHeight = entry!.contentRect.height
        throttled()
      })
      observer.observe(host)
      stopResize = () => {
        observer.disconnect()
        throttled.clear?.()
      }
    }

    return () => {
      cancelled = true
      stopResize?.()
      chartRef.current = null
      state.current = { lastSignature: undefined, initDeferred: false, lastOption: undefined }
      instance.dispose()
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [autoresize])

  // Option changes after mount: vue-echarts applies them in a post-render flush.
  const mounted = useRef(false)
  useLayoutEffect(() => {
    if (!mounted.current) {
      mounted.current = true
      return
    }
    const instance = chartRef.current
    if (!instance || state.current.initDeferred || option === state.current.lastOption)
      return
    applyOption(instance, option)
  }, [option])

  return (
    <x-vue-echarts class={className ? `echarts ${className}` : 'echarts'} style={style}>
      <div ref={hostRef} className="echarts-host" />
    </x-vue-echarts>
  )
}
