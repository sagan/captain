// Port of @vueuse/core useVirtualList (vertical, fixed itemHeight) as used by
// NodeList.vue: same range maths, container/wrapper styles and recalculation
// triggers (container size, list, total height).
import { useCallback, useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { CSSProperties } from 'react'

export function useVirtualList<T>(source: T[], { itemHeight, overscan = 5 }: { itemHeight: number, overscan?: number }) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const [state, setState] = useState({ start: 0, end: 10 })
  const [currentList, setCurrentList] = useState<Array<{ data: T, index: number }>>([])
  const sourceRef = useRef(source)
  sourceRef.current = source

  const calculateRange = useCallback(() => {
    const element = containerRef.current
    if (!element)
      return
    const offset = Math.floor(element.scrollTop / itemHeight) + 1
    const viewCapacity = Math.ceil(element.clientHeight / itemHeight)
    const from = offset - overscan
    const to = offset + viewCapacity + overscan
    const list = sourceRef.current
    const next = { start: from < 0 ? 0 : from, end: to > list.length ? list.length : to }
    setState(next)
    setCurrentList(list.slice(next.start, next.end).map((data, index) => ({ data, index: index + next.start })))
  }, [itemHeight, overscan])

  // useElementSize + watch([width, height, list, totalSize, containerRef]).
  const totalHeight = source.length * itemHeight
  useLayoutEffect(() => {
    calculateRange()
  }, [source, totalHeight, calculateRange])
  useLayoutEffect(() => {
    const el = containerRef.current
    if (!el)
      return
    const observer = new ResizeObserver(() => calculateRange())
    observer.observe(el)
    return () => observer.disconnect()
  }, [calculateRange])

  const offsetTop = state.start * itemHeight
  const wrapperStyle = useMemo<CSSProperties>(() => ({
    width: '100%',
    height: `${totalHeight - offsetTop}px`,
    marginTop: `${offsetTop}px`,
  }), [totalHeight, offsetTop])

  return {
    list: currentList,
    containerProps: { ref: containerRef, onScroll: calculateRange, style: { overflowY: 'auto' } as CSSProperties },
    wrapperProps: { style: wrapperStyle },
  }
}
