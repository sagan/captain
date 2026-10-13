// Counterpart of Vue's defineAsyncComponent(() => import(...)): render nothing
// until the chunk has loaded, then mount synchronously. React.lazy is not used
// because React throttles Suspense reveals by up to 300 ms of performance.now()
// time, which would mount components (and start the globe's rotation) at a
// different virtual instant than the Vue build under the pixel gate's clock.
import type { ComponentType } from 'react'
import { useLayoutEffect, useReducer } from 'react'
import { flushSync } from 'react-dom'

export function asyncComponent<P extends object>(loader: () => Promise<{ default: ComponentType<P> }>): ComponentType<P> {
  let resolved: ComponentType<P> | undefined
  let pending: Promise<ComponentType<P>> | undefined
  const load = () => {
    pending ??= loader().then((mod) => {
      resolved = mod.default
      return mod.default
    }, (err: unknown) => {
      // Like defineAsyncComponent: forget the failed request so a later mount retries.
      pending = undefined
      throw err
    })
    return pending
  }
  function AsyncComponent(props: P) {
    const [, rerender] = useReducer((n: number) => n + 1, 0)
    useLayoutEffect(() => {
      if (resolved)
        return
      let active = true
      load().then(() => {
        if (active)
          flushSync(rerender)
      }, (err: unknown) => {
        console.error(err)
      })
      return () => {
        active = false
      }
    }, [])
    const C = resolved
    return C ? <C {...props} /> : null
  }
  return AsyncComponent
}
