import { Accordion, Alert, Badge, Button, Card, Group, Select, Stack, Switch, Table, Text, Title } from '@mantine/core'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api, type NodeJob } from '../lib/api'
import { useAuth } from '../lib/auth'
import { toast } from '../lib/notify'

interface Schedule { enabled: boolean; interval_hours: number; city: string; family: string }
interface Hop { reply?: string; ttl: number; ip?: string; rtt_ms?: number; asns?: number[] }
interface Path { carrier: string; target: string; outcome: string; reached: boolean; hops: Hop[]; evidence: { code: string; hops: number[] }[]; classification: string; asn_missing: number }
interface Report { city: string; family: string; protocol: string; engine: string; rules_version: string; asn_provider: string; paths?: Path[] }
interface Result { started_at: number; outcome: string; route?: Report }
type Job = Omit<NodeJob, 'params' | 'result'> & { params: { route?: { city: string; family: string } }; result?: Result }
interface Data { schedule: Schedule; history: Job[] }
const defaults: Schedule = { enabled: false, interval_hours: 6, city: 'beijing', family: 'ipv4' }
const signature = (p: Path) => [...new Set(p.evidence.map(e => e.code))].sort().join(',') || 'unknown'

export function ReturnRoutesCard({ nodeID }: { nodeID: number }) {
  const { t } = useTranslation()
  const admin = useAuth().me?.role === 'admin'
  const qc = useQueryClient()
  const key = ['return-routes', nodeID]
  const q = useQuery({ queryKey: key, queryFn: () => api.get<Data>(`/api/admin/nodes/${nodeID}/return-routes`), enabled: admin, refetchInterval: query => query.state.data?.history.some(j => !j.done_at) ? 2000 : 30_000 })
  const [draft, setDraft] = useState<Schedule | null>(null)
  const [selected, setSelected] = useState<string | null>(null)
  const form = draft ?? q.data?.schedule ?? defaults
  const edit = (patch: Partial<Schedule>) => setDraft({ ...form, ...patch })
  const save = useMutation({
    mutationFn: () => api.put<Schedule>(`/api/admin/settings/nodes/${nodeID}/return-routes`, form),
    onSuccess: async () => { await qc.invalidateQueries({ queryKey: key }); setDraft(null); toast.ok(t('common.saved')) },
    onError: toast.err,
  })
  const run = useMutation({
    mutationFn: () => api.post<{ id: string }>(`/api/admin/nodes/${nodeID}/jobs`, { kind: 'network_diagnostic', params: { type: 'return_route', route: { city: form.city, family: form.family } } }),
    onSuccess: async ({ id }) => { setSelected(id); await qc.invalidateQueries({ queryKey: key }) },
    onError: toast.err,
  })
  if (!admin) return null
  const history = q.data?.history ?? []
  const job = history.find(j => j.id === selected) ?? history[0]
  const report = job?.result?.route
  const pending = run.isPending || history.some(j => !j.done_at)
  const previous = report && history.slice(history.findIndex(j => j.id === job?.id) + 1).find(j => j.result?.route?.city === report.city && j.result?.route?.family === report.family && j.result?.route?.rules_version === report.rules_version)?.result?.route
  return <Card mb="lg"><Stack gap="sm">
    <Title order={5}>{t('returnRoutes.title')}</Title>
    <Text size="sm" c="dimmed">{t('returnRoutes.hint')}</Text>
    <Text size="xs" c="dimmed">{t('returnRoutes.requirement')}</Text>
    {q.isError && <Alert color="red">{q.error.message}</Alert>}
    <Group grow align="flex-start">
      <Select label={t('returnRoutes.city')} value={form.city} allowDeselect={false} disabled={save.isPending || pending} onChange={v => v && edit({ city: v })} data={['beijing', 'shanghai', 'guangzhou'].map(value => ({ value, label: t(`returnRoutes.cities.${value}`) }))} />
      <Select label={t('returnRoutes.family')} value={form.family} allowDeselect={false} disabled={save.isPending || pending} onChange={v => v && edit({ family: v })} data={[{ value: 'ipv4', label: 'IPv4' }, { value: 'ipv6', label: 'IPv6' }]} />
    </Group>
    <Group><Button onClick={() => run.mutate()} loading={pending} disabled={!q.data || q.isError}>{t('returnRoutes.run')}</Button><Text size="xs" c="dimmed">{t('returnRoutes.direction')}</Text></Group>
    <Switch label={t('returnRoutes.schedule')} checked={form.enabled} disabled={save.isPending || !q.data} onChange={e => edit({ enabled: e.currentTarget.checked })} />
    {form.enabled && <Select label={t('returnRoutes.interval')} value={String(form.interval_hours)} allowDeselect={false} disabled={save.isPending} onChange={v => v && edit({ interval_hours: Number(v) })} data={[6, 12, 24].map(n => ({ value: String(n), label: t('returnRoutes.hours', { count: n }) }))} />}
    <Group><Button variant="light" onClick={() => save.mutate()} loading={save.isPending} disabled={!draft || !q.data || q.isError}>{t('common.save')}</Button><Text size="xs" c="dimmed">{t('returnRoutes.scheduleHint')}</Text></Group>
    <Select label={t('returnRoutes.history')} value={job?.id ?? null} allowDeselect={false} onChange={setSelected} placeholder={t('returnRoutes.empty')} data={history.map(j => ({ value: j.id, label: `${new Date(j.created_at).toLocaleString()} · ${t('returnRoutes.cities.' + j.params.route?.city)} · ${j.params.route?.family ?? ''}` }))} />
    {job?.error && <Alert color="orange">{job.error}</Alert>}
    {job && !job.done_at && <Text size="sm">{t('diagnostics.pending')}</Text>}
    {job?.result && <Group><Badge color={job.result.outcome === 'ok' ? 'blue' : 'orange'}>{t('returnRoutes.outcomes.' + job.result.outcome, { defaultValue: job.result.outcome })}</Badge><Text size="xs">{new Date(job.result.started_at * 1000).toLocaleString()}</Text></Group>}
    {job?.result?.outcome === 'unavailable' && <Alert color="orange">{t('returnRoutes.install')}</Alert>}
    {report && <Text size="xs" c="dimmed">{report.engine} · {report.protocol?.toUpperCase() ?? "—"} · {report.asn_provider} · {t('returnRoutes.rules')}: {report.rules_version}</Text>}
    {report?.paths?.map(p => {
      const before = previous?.paths?.find(old => old.carrier === p.carrier && old.target === p.target)
      const changed = before && signature(before) !== signature(p)
      return <Card key={p.carrier} withBorder padding="sm">
        <Stack gap="xs">
          <Group justify="space-between"><Text fw={600}>{t('returnRoutes.carriers.' + p.carrier)}</Text><Badge color="blue">{t('returnRoutes.classes.' + p.classification, { defaultValue: p.classification })}</Badge></Group>
          <Text size="xs" style={{ overflowWrap: 'anywhere' }}>{t('returnRoutes.target')}: {p.target}</Text>
          <Group gap="xs">{p.evidence.map(e => <Badge key={e.code} variant="light">{e.code.toUpperCase()} · {t('returnRoutes.hop')} {e.hops.join(', ')}</Badge>)}</Group>
          <Text size="xs" c="dimmed">{t('returnRoutes.outcomes.' + p.outcome, { defaultValue: p.outcome })} · {t(p.reached ? 'returnRoutes.reached' : 'returnRoutes.notReached')} · {t('returnRoutes.asnMissing', { count: p.asn_missing })}</Text>
          {changed && <Alert color="yellow">{t('returnRoutes.changed', { before: signature(before), after: signature(p) })}</Alert>}
          <Accordion variant="contained"><Accordion.Item value="hops"><Accordion.Control>{t('returnRoutes.details')}</Accordion.Control><Accordion.Panel>
            <Table.ScrollContainer minWidth={420} type="native"><Table><Table.Thead><Table.Tr><Table.Th>{t('returnRoutes.hop')}</Table.Th><Table.Th>IP</Table.Th><Table.Th>ASN</Table.Th><Table.Th>RTT</Table.Th></Table.Tr></Table.Thead><Table.Tbody>
              {p.hops.map(h => <Table.Tr key={h.ttl}><Table.Td>{h.ttl}</Table.Td><Table.Td style={{ overflowWrap: 'anywhere' }}>{h.ip || '*'} {h.reply}</Table.Td><Table.Td>{h.asns?.map(n => 'AS' + n).join(', ') || '—'}</Table.Td><Table.Td>{h.rtt_ms == null ? '—' : h.rtt_ms.toFixed(2) + ' ms'}</Table.Td></Table.Tr>)}
            </Table.Tbody></Table></Table.ScrollContainer>
          </Accordion.Panel></Accordion.Item></Accordion>
        </Stack>
      </Card>
    })}
    <Text size="xs" c="dimmed">{t('returnRoutes.limits')}</Text>
  </Stack></Card>
}
