import '@mantine/core/styles.css'
import '@mantine/charts/styles.css'
import React from 'react'
import ReactDOM from 'react-dom/client'
import { MantineProvider } from '@mantine/core'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { buildTheme } from './theme'
import { useQuery } from '@tanstack/react-query'
import { getJSON, type Snapshot } from './lib/api'
import { resolveAppearance, type SiteTheme } from './appearance'
import './i18n'
import App from './App'
import './site.css'

const queryClient = new QueryClient({ defaultOptions: { queries: { retry: 1, staleTime: 60_000 } } })


// The status page follows the operator's theme (colour, radius, scheme).
function Themed() {
  const site = useQuery({ queryKey: ['site'], queryFn: () => getJSON<{ theme?: SiteTheme }>('/api/site') })
  const snap = useQuery({ queryKey: ['probe'], queryFn: () => getJSON<Snapshot>('/api/probe'), refetchInterval: (q) => Math.max(3, q.state.data?.beat_seconds ?? 10) * 1000, retry: false })
  const a = resolveAppearance(snap.data?.appearance, site.data?.theme)
  return <MantineProvider key={a.scheme} theme={buildTheme(a.primary, a.radius, a.font)} forceColorScheme={a.scheme === 'auto' ? undefined : a.scheme} defaultColorScheme={a.scheme} colorSchemeManager={{ get: () => a.scheme, set: () => {}, clear: () => {}, subscribe: () => {}, unsubscribe: () => {} }}><div className="status-surface" data-probe-preset={a.preset}><App snap={snap} /></div></MantineProvider>
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <Themed />
    </QueryClientProvider>
  </React.StrictMode>,
)
