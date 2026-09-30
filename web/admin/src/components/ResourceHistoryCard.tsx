import { Alert, Card, Group, Loader, SegmentedControl, Select, Stack, Text, Title } from '@mantine/core'
import { LineChart } from '@mantine/charts'
import { useQueries } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../lib/api'
import { bytes } from '../lib/format'
import { historyRanges, hostMetrics, metricUnit, type ResourceHistory } from '../lib/monitoring'

// The node API supplies a complete timestamp grid with nulls for missing data.
// Comparing nodes never interpolates across a gap or treats absence as zero.
export function ResourceHistoryCard({ nodes, detail = false }: { nodes: { id: number; name: string }[]; detail?: boolean }) {
  const { t } = useTranslation()
  const [range, setRange] = useState('24h')
  const [series, setSeries] = useState('host::cpu')
  const [stat, setStat] = useState('average')
  const queries = useQueries({ queries: nodes.slice(0, 4).map((n) => ({ queryKey: ['resource-history', n.id, range, series], queryFn: () => api.get<ResourceHistory>(`/api/admin/nodes/${n.id}/resource-history?range=${range}&series=${encodeURIComponent(series)}`), refetchInterval: 60_000 })) })
  const catalog = queries[0]?.data?.series ?? []
  const metric = series.split(':')[2]
  const unit = metricUnit(metric)
  const options = new Map(hostMetrics.map((m) => [`host::${m}`, { value: `host::${m}`, label: `${t('monitoring.kinds.host')} · ${t(`monitoring.metrics.${m}`)}` }]))
  if (detail) for (const s of catalog) options.set(s.key, { value: s.key, label: `${t(`monitoring.kinds.${s.kind}`)}${s.device ? ` ${s.device}` : ''} · ${t(`monitoring.metrics.${s.metric}`)}` })
  if (!options.has(series)) options.set(series, { value: series, label: series })
  const byTS = new Map<number, Record<string, string | number | null>>()
  queries.forEach((q, i) => { for (const p of q.data?.points ?? []) { const row = byTS.get(p.ts) ?? { ts: p.ts }; row[`node${i}`] = stat === 'average' ? p.average : p.peak; byTS.set(p.ts, row) } })
  const data = [...byTS.entries()].sort(([a], [b]) => a - b).map(([ts, row]) => ({ ...row, time: new Date(ts * 1000).toLocaleString([], range === '1h' ? { hour: '2-digit', minute: '2-digit' } : { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' }) }))
  const colors = ['indigo.6', 'teal.6', 'orange.6', 'grape.6']
  const hasData = queries.some((q) => q.data?.points.some((p) => p.samples > 0))
  return <Card mb="lg"><Stack gap="sm">
    <Title order={5}>{t(detail ? 'monitoring.history' : 'monitoring.compare')}</Title>
    <Text size="xs" c="dimmed">{t('monitoring.historyHint')}</Text>
    <Group align="flex-start" grow>
      <Select label={t('monitoring.series')} searchable data={[...options.values()]} value={series} onChange={(v) => v && setSeries(v)} allowDeselect={false} />
      <Select label={t('monitoring.range')} data={historyRanges} value={range} onChange={(v) => v && setRange(v)} allowDeselect={false} />
    </Group>
    <Group justify="space-between"><SegmentedControl size="xs" value={stat} onChange={setStat} data={[{ value: 'average', label: t('monitoring.average') }, { value: 'peak', label: t('monitoring.peak') }]} /><Text size="xs" c="dimmed">{unit}</Text></Group>
    {queries.some((q) => q.isError) && <Alert color="red">{t('monitoring.loadError')}</Alert>}
    {queries.some((q) => q.isLoading) ? <Loader size="sm" /> : !hasData ? <Text size="sm" c="dimmed" py="lg">{t('monitoring.noHistory')}</Text> : <LineChart h={260} data={data} dataKey="time" series={nodes.slice(0, 4).map((n, i) => ({ name: `node${i}`, label: n.name, color: colors[i] }))} connectNulls={false} curveType="linear" withDots={data.length <= 90} withLegend valueFormatter={(v) => unit === 'B/s' ? `${bytes(v)}/s` : unit === 'B' ? bytes(v) : `${v.toFixed(1)}${unit}`} />}
    {queries.some((q) => q.data?.truncated) && <Alert color="yellow">{t('monitoring.truncated')}</Alert>}
  </Stack></Card>
}
