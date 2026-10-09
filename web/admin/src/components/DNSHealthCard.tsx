import { Accordion, Alert, Badge, Button, Card, Group, Loader, SimpleGrid, Stack, Table, Text, Title } from '@mantine/core'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import { useAuth } from '../lib/auth'
import { toast } from '../lib/notify'

interface DNSRecord { id?: string; type: string; name: string; content: string; ttl: number; proxied: boolean }
interface DNSChange { id: number; node_id: number | null; source: string; actor: string; zone: string; name: string; action: string; before: DNSRecord | null; wanted: DNSRecord; after: DNSRecord | null; outcome: string; created_at: number; finished_at: number }
interface DNSTarget { key: string; host: string; label: string; mode: string; expected: string[]; status: string; answers: { source: string; addresses: string[]; error?: string }[] }
interface DNSHealth { node_id: number; node: string; checked_at: number; stale: boolean; targets: DNSTarget[]; failure_count: number }
const date = (at: number) => at ? new Date(at * 1000).toLocaleString() : '—'
const failure = (status: string) => ['mismatch', 'missing', 'conflict', 'invalid'].includes(status)

function DNSResult({ health }: { health: DNSHealth }) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const check = useMutation({ mutationFn: () => api.post<DNSHealth>(`/api/admin/nodes/${health.node_id}/dns-health/check`), onSuccess: () => { qc.invalidateQueries({ queryKey: ['dns-health'] }); qc.invalidateQueries({ queryKey: ['selfcheck'] }) }, onError: toast.err })
  return <Stack gap="sm">
    <Group justify="space-between"><Group gap="xs"><Text size="xs" c="dimmed">{t('dnsHealth.checked', { at: date(health.checked_at) })}</Text>{health.stale && <Badge color="gray">{t('dnsHealth.stale')}</Badge>}</Group><Button size="xs" variant="light" loading={check.isPending} onClick={() => check.mutate()}>{t('dnsHealth.recheck')}</Button></Group>
    {health.targets.length === 0 && <Text size="sm" c="dimmed">{t('dnsHealth.empty')}</Text>}
    {health.targets.map(target => <Card key={target.key} withBorder padding="sm">
      <Stack gap="xs">
        <Group justify="space-between" align="flex-start"><div style={{ minWidth: 0 }}><Text fw={600} style={{ overflowWrap: 'anywhere' }}>{target.host}</Text><Text size="xs" c="dimmed">{target.label} · {t(`dnsHealth.modes.${target.mode}`)}</Text></div><Badge color={health.stale ? 'gray' : failure(target.status) ? 'red' : target.status === 'ok' ? 'teal' : 'orange'}>{t(`dnsHealth.statuses.${health.stale ? 'unknown' : target.status}`)}</Badge></Group>
        <Text size="sm" style={{ overflowWrap: 'anywhere' }}>{t('dnsHealth.expected')}: {target.expected.join(', ') || t('dnsHealth.resolveOnly')}</Text>
        <SimpleGrid cols={{ base: 1, sm: 2 }}>{target.answers.map(answer => <div key={answer.source}><Text size="xs" c="dimmed">{t(`dnsHealth.sources.${answer.source}`)}</Text><Text size="sm" style={{ overflowWrap: 'anywhere' }}>{answer.error ? t(`dnsHealth.errors.${answer.error}`) : answer.addresses.join(', ') || '—'}</Text></div>)}</SimpleGrid>
        {target.status === 'inconsistent' && <Text size="xs" c="orange">{t('dnsHealth.inconsistentHint')}</Text>}
      </Stack>
    </Card>)}
  </Stack>
}

function DNSHistory({ nodeID }: { nodeID?: number }) {
  const { t } = useTranslation()
  const [before, setBefore] = useState(0)
  const q = useQuery({ queryKey: ['dns-history', nodeID, before], queryFn: () => api.get<{ items: DNSChange[]; more: boolean }>(`/api/admin/settings/dns/history?node_id=${nodeID ?? 0}&before=${before}`), refetchInterval: 30_000 })
  const record = (r: DNSRecord | null) => r ? <Stack gap={2}><Text size="sm" style={{ overflowWrap: 'anywhere' }}>{r.type} · {r.content}</Text><Text size="xs" c="dimmed">TTL: {r.ttl === 1 ? t('dnsHealth.auto') : r.ttl} · {t('dnsHealth.proxy')}: {t(r.proxied ? 'dnsHealth.on' : 'dnsHealth.off')}</Text>{r.id && <Text size="xs" c="dimmed" style={{ overflowWrap: 'anywhere' }}>ID: {r.id}</Text>}</Stack> : <Text size="sm" c="dimmed">—</Text>
  return <Stack gap="sm">
    <Text size="sm" c="dimmed">{t('dnsHealth.historyHint')}</Text>
    {q.isLoading && <Loader size="sm" />}{q.isError && <Alert color="red">{t('dnsHealth.loadError')}</Alert>}
    {q.data && q.data.items.length === 0 && <Text size="sm" c="dimmed">{t('dnsHealth.noHistory')}</Text>}
    {!!q.data?.items.length && <Table.ScrollContainer minWidth={800}><Table verticalSpacing="sm"><Table.Thead><Table.Tr>{['change', 'before', 'wanted', 'after'].map(key => <Table.Th key={key}>{t(`dnsHealth.${key}`)}</Table.Th>)}</Table.Tr></Table.Thead><Table.Tbody>{q.data.items.map(change => <Table.Tr key={change.id}>
      <Table.Td style={{ maxWidth: 260 }}><Stack gap={3}><Text size="sm" fw={600} style={{ overflowWrap: 'anywhere' }}>{change.name}</Text><Badge color={change.outcome === 'success' ? 'teal' : 'orange'}>{t(`dnsHealth.outcomes.${change.outcome}`)}</Badge><Text size="xs">{t(`dnsHealth.actions.${change.action}`)} · {date(change.created_at)}</Text><Text size="xs" c="dimmed" style={{ overflowWrap: 'anywhere' }}>{change.actor || '—'}</Text>{change.node_id && <Text component={Link} size="xs" to={`/nodes/${change.node_id}`}>{t('dnsHealth.node', { id: change.node_id })}</Text>}</Stack></Table.Td>
      <Table.Td>{record(change.before)}</Table.Td><Table.Td>{record(change.wanted)}</Table.Td><Table.Td>{record(change.after)}</Table.Td>
    </Table.Tr>)}</Table.Tbody></Table></Table.ScrollContainer>}
    <Group justify="flex-end">{before > 0 && <Button variant="subtle" size="xs" onClick={() => setBefore(0)}>{t('dnsHealth.latest')}</Button>}{q.data?.more && <Button variant="light" size="xs" onClick={() => setBefore(q.data!.items[q.data!.items.length - 1].id)}>{t('dnsHealth.older')}</Button>}</Group>
  </Stack>
}

export function DNSHealthCard({ nodeID }: { nodeID?: number }) {
  const { t } = useTranslation()
  const { me } = useAuth()
  const [history, setHistory] = useState(false)
  const q = useQuery({ queryKey: ['dns-health', nodeID], queryFn: async () => nodeID ? [await api.get<DNSHealth>(`/api/admin/nodes/${nodeID}/dns-health`)] : api.get<DNSHealth[]>('/api/admin/monitoring/dns'), refetchInterval: 30_000 })
  return <Card mb={nodeID ? 'lg' : 0} style={{ minWidth: 0 }}><Stack>
    <Title order={5}>{t('dnsHealth.title')}</Title><Text size="sm" c="dimmed">{t('dnsHealth.hint')}</Text>
    {q.isLoading && <Loader size="sm" />}{q.isError && <Alert color="red">{t('dnsHealth.loadError')}</Alert>}
    {q.data?.length === 0 && <Text c="dimmed" size="sm">{t('dnsHealth.empty')}</Text>}
    {nodeID ? q.data?.map(health => <DNSResult key={health.node_id} health={health} />) : <Accordion variant="separated">{q.data?.map(health => <Accordion.Item key={health.node_id} value={String(health.node_id)}><Accordion.Control><Group gap="xs"><Text fw={600}>{health.node}</Text><Badge color={health.stale ? 'gray' : health.targets.some(t => failure(t.status)) ? 'red' : health.targets.every(t => t.status === 'ok') ? 'teal' : 'orange'}>{t(`dnsHealth.statuses.${health.stale ? 'unknown' : health.targets.some(t => failure(t.status)) ? 'problem' : health.targets.every(t => t.status === 'ok') ? 'ok' : 'unknown'}`)}</Badge></Group></Accordion.Control><Accordion.Panel><DNSResult health={health} /></Accordion.Panel></Accordion.Item>)}</Accordion>}
    {me?.role === 'admin' && <><Button variant="subtle" size="sm" style={{ alignSelf: 'flex-start' }} onClick={() => setHistory(!history)}>{t('dnsHealth.history')}</Button>{history && <DNSHistory key={nodeID ?? "all"} nodeID={nodeID} />}</>}
  </Stack></Card>
}
