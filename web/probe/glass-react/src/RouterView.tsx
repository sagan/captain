// The routed area of App.vue:
//   <RouterView v-slot="{ Component }">
//     <Transition v-bind="routeTransition" :mode="disablePageAnimation ? 'default' : 'out-in'">
//       <KeepAlive :include="['HomeView']"><component :is="Component" /></KeepAlive>
//     </Transition>
//   </RouterView>
//
// The Transition animates the root element of whichever view is shown. With
// KeepAlive, HomeView is deactivated (kept, detached) instead of destroyed,
// and InstanceDetail is one instance reused across node ids (RouterView keys
// by component). Here HomeView lives in a React <Activity> that is hidden
// while another route shows; the enter/leave classes run on each view's root
// through the same hooks as <Transition>, in out-in order.
import { asyncComponent } from '@/lib/asyncComponent'
import { Activity, useCallback, useLayoutEffect, useRef, useState } from 'react'
import type { Ref } from 'react'
import { beforeEnter, enter, leave, resolveClasses } from '@/lib/transition'
import type { TransitionClassProps } from '@/lib/transition'
import { ActiveContext, useRoute } from '@/lib/router'
import type { RouteName } from '@/lib/router'
import { useApp } from '@/stores/app'

const HomeView = asyncComponent(() => import('@/views/HomeView'))
const InstanceDetail = asyncComponent(() => import('@/views/InstanceDetail'))

const ROUTE_TRANSITION: TransitionClassProps = {
  enterActiveClass: 'transition-all duration-300 ease-out',
  enterFromClass: 'opacity-0 translate-y-2',
  enterToClass: 'opacity-100 translate-y-0',
  leaveActiveClass: 'transition-opacity duration-150 ease-in',
  leaveFromClass: 'opacity-100',
  leaveToClass: 'opacity-0',
}

type View = RouteName | undefined

export default function RouterView() {
  const route = useRoute()
  const disablePageAnimation = useApp(s => s.disablePageAnimation)
  const css = !disablePageAnimation
  const outIn = !disablePageAnimation

  // `shown` is the view on screen; it trails `route.name` while a leave runs (out-in).
  const [shown, setShown] = useState<View>(route.name)
  const [leaving, setLeaving] = useState<View | null>(null)
  const roots = useRef(new Map<View, HTMLElement>())
  const firstMount = useRef(true)
  const pendingEnter = useRef<View | null>(null)

  const target = route.name
  if (target !== shown && leaving === null) {
    // Start the switch: the old view leaves; in default mode the new one enters at once.
    if (shown !== undefined && css)
      setLeaving(shown)
    if (!outIn || !css || shown === undefined) {
      pendingEnter.current = target
      setShown(target)
    }
  }

  const finishLeave = useCallback(() => {
    setLeaving(null)
    if (outIn) {
      pendingEnter.current = route.name
      setShown(route.name)
    }
  }, [outIn, route.name])

  const setRoot = useCallback((view: View) => (node: HTMLElement | null) => {
    if (node)
      roots.current.set(view, node)
  }, [])

  // Enter hooks: beforeEnter + enter on the newly shown view's root (not on first paint: no appear).
  useLayoutEffect(() => {
    const view = pendingEnter.current
    if (view === null)
      return
    pendingEnter.current = null
    if (firstMount.current || !css)
      return
    const el = roots.current.get(view)
    if (!el)
      return
    const classes = resolveClasses(ROUTE_TRANSITION)
    beforeEnter(el, classes, false)
    enter(el, classes, ROUTE_TRANSITION, false)
  })

  // Leave hooks on the old view's root.
  const leaveStarted = useRef<View | null>(null)
  useLayoutEffect(() => {
    if (leaving === null) {
      leaveStarted.current = null
      return
    }
    if (leaveStarted.current === leaving)
      return
    leaveStarted.current = leaving
    const el = roots.current.get(leaving)
    if (!el) {
      finishLeave()
      return
    }
    leave(el, resolveClasses(ROUTE_TRANSITION), ROUTE_TRANSITION, finishLeave)
  })

  useLayoutEffect(() => {
    firstMount.current = false
  }, [])

  const homeVisible = shown === 'home' || leaving === 'home'
  const detailVisible = shown === 'instance-detail' || leaving === 'instance-detail'

  return (
    <>
      <Activity mode={homeVisible ? 'visible' : 'hidden'}>
        <ActiveContext.Provider value={shown === 'home' && leaving !== 'home'}>
          <HomeView ref={setRoot('home') as Ref<HTMLDivElement>} />
        </ActiveContext.Provider>
      </Activity>
      {detailVisible && <InstanceDetail ref={setRoot('instance-detail') as Ref<HTMLDivElement>} />}
    </>
  )
}
