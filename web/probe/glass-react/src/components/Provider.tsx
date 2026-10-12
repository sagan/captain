import type { ReactNode } from 'react'
import { createContext, useLayoutEffect, useState } from 'react'
import { BackTop } from '@/components/ui/back-top'
import { useApp } from '@/stores/app'
import { buildGlassThemeTokens } from '@shared/utils/glassTheme'

/** Whether the page has scrolled past the back-to-top threshold (Vue: provide('isScrolled')). */
export const ScrolledContext = createContext(false)

export default function Provider({ children }: { children?: ReactNode }) {
  const isDark = useApp(s => s.isDark)
  const glassColorPreset = useApp(s => s.glassColorPreset)
  const glassCustomColors = useApp(s => s.glassCustomColors)
  const colorVisionFriendly = useApp(s => s.colorVisionFriendly)
  const [isScrolled, setIsScrolled] = useState(false)

  // Layout effects so the root classes are in place before the first paint,
  // as Vue's immediate watchers set them during setup.
  useLayoutEffect(() => {
    const root = document.documentElement
    if (isDark)
      root.classList.add('dark')
    else root.classList.remove('dark')
    root.style.colorScheme = isDark ? 'dark' : 'light'
  }, [isDark])

  useLayoutEffect(() => {
    const tokens = buildGlassThemeTokens(glassColorPreset, glassCustomColors)
    const root = document.documentElement
    root.style.setProperty('--glass-light-card', tokens.lightCard)
    root.style.setProperty('--glass-light-card-hover', tokens.lightCardHover)
    root.style.setProperty('--glass-light-control', tokens.lightControl)
    root.style.setProperty('--glass-light-header', tokens.lightHeader)
    root.style.setProperty('--glass-light-text', tokens.lightText)
    root.style.setProperty('--glass-light-muted-text', tokens.lightMutedText)
    root.style.setProperty('--glass-light-border', tokens.lightBorder)
    root.style.setProperty('--glass-light-shadow', tokens.lightShadow)
    root.style.setProperty('--glass-dark-card', tokens.darkCard)
    root.style.setProperty('--glass-dark-card-hover', tokens.darkCardHover)
    root.style.setProperty('--glass-dark-control', tokens.darkControl)
    root.style.setProperty('--glass-dark-header', tokens.darkHeader)
    root.style.setProperty('--glass-dark-text', tokens.darkText)
    root.style.setProperty('--glass-dark-muted-text', tokens.darkMutedText)
    root.style.setProperty('--glass-dark-border', tokens.darkBorder)
    root.style.setProperty('--glass-dark-shadow', tokens.darkShadow)
  }, [glassColorPreset, glassCustomColors])

  useLayoutEffect(() => {
    document.documentElement.classList.toggle('color-vision-friendly', colorVisionFriendly)
  }, [colorVisionFriendly])

  return (
    <ScrolledContext.Provider value={isScrolled}>
      {children}
      <BackTop visibilityHeight={320} onScrolled={setIsScrolled} />
    </ScrolledContext.Provider>
  )
}
