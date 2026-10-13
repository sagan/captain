import { useEffect, useState } from 'react'
import type { VersionInfo } from '@shared/utils/api'
import { getSharedApi } from '@shared/utils/api'
import { DataTooltip } from '@/components/ui/data-tooltip'

const api = getSharedApi()

const buildVersion = __BUILD_VERSION__
const buildGitHash = __BUILD_GIT_HASH__

export default function Footer() {
  const [serverVersion, setServerVersion] = useState<VersionInfo | null>(null)

  useEffect(() => {
    let cancelled = false
    api.getVersion()
      .then((v) => {
        if (!cancelled)
          setServerVersion(v)
      })
      .catch(() => {
        // 静默失败
      })
    return () => {
      cancelled = true
    }
  }, [])

  const formattedServerVersion = serverVersion?.version ?? ''

  return (
    <footer className="w-full max-w-[1280px] mx-auto p-4">
      <div className="flex w-full flex-row justify-between gap-4 text-xs text-muted-foreground">
        <div className="flex gap-1 items-center">
          {' Powered by '}
          <DataTooltip as="span" placement="top" content={formattedServerVersion}>
            <a
              href="https://github.com/zeptop-dev/captain"
              target="_blank"
              rel="noopener noreferrer"
              className="transition-opacity hover:opacity-80"
            >
              <span className="font-medium text-foreground">Captain</span>
            </a>
          </DataTooltip>
        </div>
        <div className="flex flex-wrap gap-1 items-center justify-end text-right">
          {' Theme by '}
          <DataTooltip as="span" placement="top" content={`v${buildVersion}\n${buildGitHash}`}>
            <a
              href="https://github.com/sanrokamlan-prog/komari-theme-Glassmorphism"
              target="_blank"
              rel="noopener noreferrer"
              className="transition-opacity hover:opacity-80"
            >
              <span className="font-medium text-foreground">Komari Glassmorphism</span>
            </a>
          </DataTooltip>
        </div>
      </div>
    </footer>
  )
}
