import { Alert, Card, Loader, Select, Stack, Title } from '@mantine/core'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../lib/api'
import type { NetworkHistory } from '../lib/network'
import { NetworkQualityTable } from './NetworkQualityTable'
import { NetworkHistoryChart } from './NetworkHistoryChart'
export function NetworkQualityCard({ nodeID }: { nodeID: number }) {
 const { t } = useTranslation()
 const [range, setRange] = useState('24h')
 const q = useQuery({ queryKey: ['network-quality', nodeID, range], queryFn: () => api.get<NetworkHistory>(`/api/admin/nodes/${nodeID}/network-quality?range=${range}`), refetchInterval: 30_000 })
 return <Card mb="lg"><Stack gap="sm"><Title order={5}>{t('networkQuality.title')}</Title>
  {q.isError && <Alert color="red">{t('networkQuality.error')}</Alert>}
  {q.isLoading ? <Loader size="sm" /> : <NetworkQualityTable samples={q.data?.current ?? []} />}
  <Select label={t('networkQuality.range')} w={180} value={range} onChange={(v) => v && setRange(v)} allowDeselect={false} data={['1h', '24h', '7d', '14d', '30d', '90d']} />
  <NetworkHistoryChart history={q.data} />
 </Stack></Card>
}
