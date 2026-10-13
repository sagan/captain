import type { CSSProperties } from 'react'
import { useCallback, useEffect, useRef, useState } from 'react'
import defaultBackground from '../../public/images/default-background-v2.webp'
import '@/scoped/background.css'
import { scope } from '@/scoped/scope'
import { scopes } from '@/scoped/scopes'
import Transition from '@/components/Transition'
import { useApp } from '@/stores/app'

const s = scope('background')
const defaultBackgroundImage = `url(${defaultBackground})`
// Vue's useCssVars sets the v-bind() variable on the component root.
const cssVars = { [scopes.background.vars.defaultBackgroundImage]: defaultBackgroundImage } as CSSProperties

export default function Background() {
  const st = useApp(a => ({
    backgroundEnabled: a.backgroundEnabled,
    currentBackgroundUrl: a.currentBackgroundUrl,
    backgroundType: a.backgroundType,
    backgroundOverlay: a.backgroundOverlay,
    backgroundBlur: a.backgroundBlur,
  }))
  const [isLoaded, setIsLoaded] = useState(false)
  const [hasError, setHasError] = useState(false)
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const imageLoader = useRef<HTMLImageElement | null>(null)

  const showBackground = st.backgroundEnabled
  const currentUrl = showBackground ? st.currentBackgroundUrl : ''
  const backgroundType = st.backgroundType
  const hasCustomBackground = showBackground && !!currentUrl
  const showBackgroundOverlay = st.backgroundOverlay > 0

  const backgroundStyle: CSSProperties = {
    filter: st.backgroundBlur > 0 ? `blur(${st.backgroundBlur}px)` : 'none',
    opacity: st.backgroundType === 'video' && !isLoaded ? 0 : 1,
  }
  const backgroundContainerStyle: CSSProperties = st.backgroundOverlay >= 0 ? {} : { opacity: 1 - Math.abs(st.backgroundOverlay) / 100 }
  const overlayStyle: CSSProperties = st.backgroundOverlay <= 0 ? {} : { backgroundColor: `rgba(0, 0, 0, ${st.backgroundOverlay / 100})` }

  const showLoadedBackground = hasCustomBackground && isLoaded && !hasError
  const showMediaBackground = hasCustomBackground && !hasError && (backgroundType === 'video' || showLoadedBackground)
  const showDefaultBackground = !hasCustomBackground || !showMediaBackground || hasError
  const showLoadingBackground = hasCustomBackground && backgroundType === 'video' && !isLoaded && !hasError
  const showFallbackBackground = hasCustomBackground && backgroundType === 'video' && hasError

  const clearImageLoader = useCallback(() => {
    if (imageLoader.current) {
      imageLoader.current.onload = null
      imageLoader.current.onerror = null
      imageLoader.current = null
    }
  }, [])

  const resetBackgroundState = useCallback(() => {
    clearImageLoader()
    const video = videoRef.current
    if (video) {
      video.pause()
      video.removeAttribute('src')
      video.load()
    }
    setIsLoaded(false)
    setHasError(false)
  }, [clearImageLoader])

  useEffect(() => {
    if (!showBackground || !currentUrl) {
      resetBackgroundState()
      return
    }
    if (backgroundType === 'image') {
      setIsLoaded(false)
      setHasError(false)
      clearImageLoader()
      const img = new Image()
      imageLoader.current = img
      img.onload = () => {
        setIsLoaded(true)
        setHasError(false)
      }
      img.onerror = () => {
        setIsLoaded(false)
        setHasError(true)
      }
      img.src = currentUrl
    }
    else if (backgroundType === 'video') {
      clearImageLoader()
      setIsLoaded(false)
      setHasError(false)
    }
  }, [showBackground, currentUrl, backgroundType, clearImageLoader, resetBackgroundState])

  useEffect(() => () => resetBackgroundState(), [resetBackgroundState])

  const handleVideoLoaded = () => {
    setIsLoaded(true)
    setHasError(false)
  }
  const handleVideoError = () => {
    setIsLoaded(false)
    setHasError(true)
  }

  return (
    <div {...s} className="background-container" style={{ ...backgroundContainerStyle, ...cssVars }}>
      <Transition name="fade">
        {showDefaultBackground && <div {...s} className="default-background" />}
      </Transition>
      <Transition name="fade">
        {showLoadingBackground && <div {...s} className="background-loading" />}
      </Transition>
      <Transition name="fade">
        {showFallbackBackground && <div {...s} className="background-loading" />}
      </Transition>
      <Transition name="fade">
        {showMediaBackground && (
          <div {...s} className="background-media" style={backgroundStyle}>
            {backgroundType === 'image'
              ? <div {...s} className="background-image" style={{ backgroundImage: `url(${currentUrl})` }} />
              : backgroundType === 'video'
                ? (
                    <video
                      {...s}
                      ref={videoRef}
                      className="background-video"
                      src={currentUrl ?? undefined}
                      autoPlay
                      loop
                      muted
                      preload="auto"
                      playsInline
                      onLoadedData={handleVideoLoaded}
                      onCanPlay={handleVideoLoaded}
                      onError={handleVideoError}
                    />
                  )
                : null}
          </div>
        )}
      </Transition>
      {showBackgroundOverlay && <div {...s} className="background-overlay" style={overlayStyle} />}
    </div>
  )
}
