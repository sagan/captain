import { Alert, Card, Group, Loader, Select, Stack, Title } from '@mantine/core'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../lib/api'
import type { Availability } from '../lib/availability'
import { AvailabilityView } from './AvailabilityView'
export function AvailabilityCard({ nodeID }: { nodeID: number }) {
 const { t } = useTranslation()
 const [range, setRange] = useState('24h')
 const q = useQuery({ queryKey: ['availability', nodeID, range], queryFn: () => api.get<Availability>(`/api/admin/nodes/${nodeID}/availability?range=${range}`), refetchInterval: 30_000 })
 return <Card mb="lg"><Stack gap="sm"><Group justify="space-between"><Title order={5}>{t('availability.title')}</Title><Select w={120} aria-label={t('availability.range')} value={range} onChange={(v) => v && setRange(v)} allowDeselect={false} data={['1h', '24h', '7d', '30d', '90d']} /></Group>{q.isError && <Alert color="red">{t('availability.error')}</Alert>}{q.isLoading && <Loader size="sm" />}{q.data && <AvailabilityView data={q.data} />}</Stack></Card>
}
