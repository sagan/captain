import '@/scoped/loading-cover.css'
import { scope } from '@/scoped/scope'
import { useApp } from '@/stores/app'

const s = scope('loading-cover')

export default function LoadingCover() {
  const hasCustomBackground = useApp(st => st.backgroundEnabled && Boolean(st.currentBackgroundUrl))
  return (
    <div
      {...s}
      className={`loading-cover flex items-center inset-0 justify-center fixed z-20 ${hasCustomBackground ? 'loading-cover--custom-background' : ''}`}
    >
      <div
        {...s}
        className={`loading-cover__indicator flex flex-col items-center gap-2 ${hasCustomBackground ? 'text-white/75 dark:text-white/75' : 'text-foreground'}`}
      >
        <span
          {...s}
          className={`inline-block animate-spin rounded-full border-2 ${hasCustomBackground ? 'size-5' : 'size-7'}`}
          style={{ borderColor: 'color-mix(in srgb, currentColor 18%, transparent)', borderTopColor: 'currentColor' }}
        />
        {!hasCustomBackground && <span {...s} className="text-sm text-muted-foreground">Loading...</span>}
      </div>
    </div>
  )
}
