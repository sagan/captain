import type { ComponentProps } from 'react'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { cn } from '@/lib/utils'

type ModelValue = string | number

export interface InputProps extends Omit<ComponentProps<'input'>, 'value' | 'defaultValue'> {
  /** 受控值（对应 Vue 的 modelValue） */
  value?: ModelValue
  /** 非受控初始值 */
  defaultValue?: ModelValue
  /** 对应 Vue 的 update:modelValue；type="number" 时与 v-model 一样转换为数字 */
  onValueChange?: (value: ModelValue) => void
}

// 与 Vue 的 looseToNumber 一致：解析失败时保留原字符串
function looseToNumber(val: string): ModelValue {
  const n = Number.parseFloat(val)
  return Number.isNaN(n) ? val : n
}

type ComposingInput = HTMLInputElement & { composing?: boolean }

// 复刻 vModelText：值只写入 DOM 的 value 属性（不产生 value 特性），
// 监听原生 input / change / composition 事件。
export function Input({ className, value, defaultValue, onValueChange, ref, ...props }: InputProps) {
  const el = useRef<ComposingInput | null>(null)
  const [inner, setInner] = useState<ModelValue | undefined>(defaultValue)
  const model = value !== undefined ? value : inner
  const castToNumber = props.type === 'number'

  const assignRef = useRef<(v: ModelValue) => void>(() => {})
  assignRef.current = (v) => {
    if (value === undefined)
      setInner(v)
    onValueChange?.(v)
  }

  useEffect(() => {
    const node = el.current
    if (!node)
      return
    const cast = (raw: string) => (castToNumber ? looseToNumber(raw) : raw)
    const onInput = (e: Event) => {
      if ((e.target as ComposingInput).composing)
        return
      assignRef.current(cast(node.value))
    }
    const onCompositionStart = (e: Event) => {
      (e.target as ComposingInput).composing = true
    }
    const onCompositionEnd = (e: Event) => {
      const target = e.target as ComposingInput
      if (target.composing) {
        target.composing = false
        target.dispatchEvent(new Event('input'))
      }
    }
    const onNumberChange = () => {
      node.value = String(cast(node.value))
    }
    node.addEventListener('input', onInput)
    if (castToNumber)
      node.addEventListener('change', onNumberChange)
    node.addEventListener('compositionstart', onCompositionStart)
    node.addEventListener('compositionend', onCompositionEnd)
    node.addEventListener('change', onCompositionEnd)
    return () => {
      node.removeEventListener('input', onInput)
      node.removeEventListener('change', onNumberChange)
      node.removeEventListener('compositionstart', onCompositionStart)
      node.removeEventListener('compositionend', onCompositionEnd)
      node.removeEventListener('change', onCompositionEnd)
    }
  }, [castToNumber])

  useLayoutEffect(() => {
    const node = el.current
    if (!node || node.composing)
      return
    const elValue = castToNumber && !/^0\d/.test(node.value) ? looseToNumber(node.value) : node.value
    const next = model == null ? '' : model
    if (elValue === next)
      return
    node.value = String(next)
  })

  return (
    <input
      ref={(node) => {
        el.current = node
        if (typeof ref === 'function')
          return ref(node)
        if (ref)
          ref.current = node
      }}
      data-slot="input"
      className={cn(
        'bg-transparent border-input focus-visible:border-ring focus-visible:ring-ring/50 aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive dark:aria-invalid:border-destructive/50 h-9 rounded-md border px-3 py-1 text-base shadow-xs transition-colors file:h-7 file:text-sm file:font-medium focus-visible:ring-[3px] aria-invalid:ring-[3px] md:text-sm w-full min-w-0 outline-none file:inline-flex file:border-0 file:bg-transparent file:text-foreground placeholder:text-muted-foreground disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30',
        className,
      )}
      {...props}
    />
  )
}
