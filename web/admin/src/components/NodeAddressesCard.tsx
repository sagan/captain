import { Alert, Badge, Card, Group, SimpleGrid, Stack, Switch, Text, Title } from '@mantine/core'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '../lib/api'
import { toast } from '../lib/notify'

type Family = { address: string; last_success: number; confirmed_at: number }
type Addresses = { auto_ipv4: boolean; auto_ipv6: boolean; entry_ipv4: string; entry_ipv6: string; observed: { received_at: number; ipv4: Family; ipv6: Family }; dns_pending: boolean }
export function NodeAddressesCard({ nodeID, canWrite }: { nodeID: number; canWrite: boolean }) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const q = useQuery({ queryKey: ['node-addresses', nodeID], queryFn: () => api.get<Addresses>(`/api/admin/nodes/${nodeID}/addresses`), refetchInterval: 30_000 })
  const save = useMutation({ mutationFn: (value: Partial<Pick<Addresses, 'auto_ipv4' | 'auto_ipv6'>>) => api.put(`/api/admin/settings/nodes/${nodeID}/addresses`, value), onSuccess: () => { qc.invalidateQueries({ queryKey: ['node-addresses', nodeID] }); qc.invalidateQueries({ queryKey: ['node'] }); toast.ok(t('common.saved')) }, onError: toast.err })
  const fresh = !q.isError && !!q.data?.observed.received_at && q.dataUpdatedAt / 1000 - q.data.observed.received_at < 900
  return <Card mb="lg"><Stack gap="sm">
    <Title order={5}>{t('nodeAddresses.title')}</Title>
    <Text size="sm" c="dimmed">{t('nodeAddresses.hint')}</Text>
    {q.isError && <Alert color="red">{t('dnsHealth.loadError')}</Alert>}
    {!fresh && <Text size="sm" c="dimmed">{t('nodeAddresses.waiting')}</Text>}
    {q.data && <>
      <SimpleGrid cols={{ base: 1, sm: 2 }}>{(['ipv4', 'ipv6'] as const).map(family => {
        const value = q.data!.observed[family]
        const following = family === 'ipv4' ? q.data!.auto_ipv4 : q.data!.auto_ipv6
        const current = fresh && value.last_success === q.data!.observed.received_at
        return <Card key={family} withBorder p="sm"><Stack gap="xs">
          <Group justify="space-between"><Text fw={600}>{family === 'ipv4' ? 'IPv4' : 'IPv6'}</Text><Badge color={current ? 'teal' : 'gray'}>{t(current ? 'nodeAddresses.detected' : 'nodeAddresses.unknown')}</Badge></Group>
          <Text size="sm" style={{ overflowWrap: 'anywhere' }}>{t('nodeAddresses.machine')}: {value.address || '—'}</Text>
          <Text size="xs" c="dimmed">{t('nodeAddresses.lastSuccess')}: {value.last_success ? new Date(value.last_success * 1000).toLocaleString() : '—'}</Text>
          <Text size="sm" style={{ overflowWrap: 'anywhere' }}>{t('nodeAddresses.entry')}: {(family === 'ipv4' ? q.data!.entry_ipv4 : q.data!.entry_ipv6) || '—'}</Text>
          {canWrite && <Switch label={t('nodeAddresses.follow')} checked={following} disabled={save.isPending || (!following && !fresh)} onChange={e => save.mutate(family === 'ipv4' ? { auto_ipv4: e.currentTarget.checked } : { auto_ipv6: e.currentTarget.checked })} />}
        </Stack></Card>
      })}</SimpleGrid>
      <Text size="xs" c="dimmed">{t('nodeAddresses.behavior')}</Text>
      {q.data.dns_pending && <Text size="sm" c="orange">{t('nodeAddresses.dnsPending')}</Text>}
    </>}
  </Stack></Card>
}
