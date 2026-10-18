import { Alert, Badge, Button, Card, Checkbox, Code, Divider, Group, Modal, NumberInput, Select, SimpleGrid, Stack, Tabs, Text, Textarea, TextInput, Title } from '@mantine/core'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { api, type NodeJob, type Node } from '../lib/api'
import { useAuth } from '../lib/auth'
import { toast } from '../lib/notify'
import { PageHeader } from '../components/PageHeader'

interface Item { id: string; state: string; values: Record<string, string>; editable: boolean; reason?: string }
interface Pending { id: string; section: string; deadline: number; phase: string }
interface Snapshot { checked_at: number; environment: string; revision: string; items: Item[]; pending?: Pending }
interface Request { action: string; section?: string; values?: Record<string, string>; revision?: string; transaction?: string; verified?: boolean }
interface Result { state?: Snapshot; outcome: string; message?: string }
type Job = Omit<NodeJob, 'params' | 'result'> & { params: Request; result?: Result }
interface Data { supported: boolean; online: boolean; minimum: string; state?: Snapshot; history: Job[] }
const groups = [['admin', 'ssh', 'port', 'firewall', 'fail2ban'], ['hostname', 'updates', 'time', 'swap'], ['dns', 'network']]

export default function HostManagementPage() {
  const { id } = useParams(); const { t } = useTranslation(); const admin = useAuth().me?.role === 'admin'
  const qc = useQueryClient(); const key = ['host-management', id]
  const node = useQuery({ queryKey: ['host-node', id], queryFn: () => api.get<{ node: Node }>(`/api/admin/nodes/${id}`), enabled: admin })
  const q = useQuery({ queryKey: key, queryFn: () => api.get<Data>(`/api/admin/settings/nodes/${id}/host`), enabled: admin, refetchInterval: query => query.state.data?.history.some(j => !j.done_at) ? 2000 : 15000 })
  const [editing, setEditing] = useState<Item | null>(null); const [values, setValues] = useState<Record<string, string>>({})
  const [revision, setRevision] = useState(''); const [review, setReview] = useState(false); const [verified, setVerified] = useState(false)
  const [tab, setTab] = useState<string | null>('state')
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => { const timer = window.setInterval(() => setNow(Date.now()), 1000); return () => window.clearInterval(timer) }, [])
  const run = useMutation({ mutationFn: (r: Request) => api.post(`/api/admin/settings/nodes/${id}/host`, r), onSuccess: async () => { setEditing(null); setReview(false); setVerified(false); await qc.invalidateQueries({ queryKey: key }) }, onError: toast.err })
  const busy = run.isPending || !!q.data?.history.some(j => !j.done_at)
  const snapshot = q.data?.state; const pending = snapshot?.pending
  const stale = !!snapshot && now / 1000 - snapshot.checked_at > 300
  const online = !!q.data?.online && !!q.data?.supported
  const auto = useRef('')
  useEffect(() => {
    if (q.data?.supported && q.data.online && !busy && (!q.data.state || Date.now() / 1000 - q.data.state.checked_at > (q.data.state.pending ? 20 : 300))) {
      const marker = `${id}:${Math.floor(Date.now() / (q.data.state?.pending ? 20000 : 300000))}`
      if (auto.current !== marker) { auto.current = marker; run.mutate({ action: 'inspect' }) }
    }
  }, [id, q.data, busy, run])
  const label = (s: string) => t('host.fields.' + s, { defaultValue: s })
  const open = (item: Item) => {
    let defaults: Record<string, string> = {}
    switch (item.id) {
      case 'hostname': defaults = { hostname: item.values.hostname }; break
      case 'time': defaults = { timezone: item.values.timezone || 'UTC', ntp: item.values.ntp === 'yes' ? 'true' : 'false' }; break
      case 'port': defaults = { port: '' }; break
      case 'ssh': defaults = { root: item.values.root === 'prohibit-password' ? 'prohibit-password' : 'no', password: item.values.password || 'no' }; break
      case 'admin': defaults = { user: '', public_key: '' }; break
      case 'firewall': defaults = { port: '', protocol: 'tcp', source: '' }; break
      case 'fail2ban': defaults = { enabled: 'true', bantime: '3600', findtime: '600', maxretry: '5' }; break
      case 'updates': defaults = { enabled: 'true' }; break
      case 'dns': defaults = { servers: '' }; break
      case 'network': defaults = { bbr: item.values.bbr === 'bbr' ? 'bbr' : 'cubic', security: 'false' }; break
      case 'swap': defaults = { size_mb: '1024' }; break
    }
    setValues(defaults); setRevision(snapshot?.revision || ''); setEditing(item); setReview(false)
  }
  const field = (name: string, value: string) => {
    const set = (v: string) => setValues(old => ({ ...old, [name]: v }))
    let options: string[] | undefined
    if (['enabled', 'ntp', 'security'].includes(name)) options = ['true', 'false']
    if (name === 'root') options = ['no', 'prohibit-password']
    if (name === 'password') options = ['no', 'yes']
    if (name === 'protocol') options = ['tcp', 'udp']
    if (name === 'bbr') options = ['bbr', 'cubic']
    if (options) return <Select key={name} label={label(name)} value={value} allowDeselect={false} data={options.map(v => ({ value: v, label: name === 'security' && v === 'false' ? t('host.keep') : t('host.values.' + v, { defaultValue: v }) }))} onChange={v => v && set(v)} />
    if (name === 'public_key') return <Textarea key={name} label={label(name)} value={value} onChange={e => set(e.currentTarget.value)} minRows={3} autosize required placeholder="ssh-ed25519 …" />
    if (['port', 'bantime', 'findtime', 'maxretry', 'size_mb'].includes(name)) return <NumberInput key={name} label={label(name)} value={value ? Number(value) : ''} min={name === 'size_mb' ? 256 : 1} max={name === 'size_mb' ? 16384 : name === 'port' ? 65535 : name === 'maxretry' ? 100 : 604800} allowDecimal={false} allowNegative={false} onChange={v => set(String(v))} required />
    return <TextInput key={name} label={label(name)} value={value} onChange={e => set(e.currentTarget.value)} required placeholder={name === 'timezone' ? 'Asia/Shanghai' : name === 'servers' ? '1.1.1.1, 2606:4700:4700::1111' : name === 'source' ? '192.0.2.0/24' : undefined} />
  }
  if (!admin) return <Alert color="orange">{t('host.adminOnly')}</Alert>
  const latest = q.data?.history[0]
  return <>
    <PageHeader title={t('host.title')} subtitle={node.data?.node.name || ''} actions={<Button component={Link} to={`/nodes/${id}`} variant="default">{t('host.back')}</Button>} />
    <Tabs value={tab} onChange={setTab}><Tabs.List mb="lg"><Tabs.Tab value="state">{t('host.current')}</Tabs.Tab><Tabs.Tab value="history">{t('host.history')}</Tabs.Tab></Tabs.List>
      <Tabs.Panel value="state">
        <Group justify="space-between" mb="md"><Text size="sm" c="dimmed">{snapshot ? t('host.checked', { time: new Date(snapshot.checked_at * 1000).toLocaleString() }) : t('host.noData')}{stale && ' · ' + t('host.stale')}</Text><Button variant="light" loading={busy} disabled={!online} onClick={() => run.mutate({ action: 'inspect' })}>{t('host.refresh')}</Button></Group>
        {q.isError && <Alert color="red" mb="md">{q.error.message}</Alert>}
        {q.data && !q.data.supported && <Alert mb="md">{t('host.minimum', { version: q.data.minimum })}</Alert>}
        {q.data && !q.data.online && <Alert color="gray" mb="md">{t('host.offline')}</Alert>}
        {snapshot?.environment === 'restricted' && <Alert mb="md">{t('host.restricted')}</Alert>}
        {busy && <Alert mb="md">{t('host.running')}</Alert>}
        {(latest?.error || latest?.result?.message) && <Alert color="orange" mb="md">{latest.error || latest.result?.message}</Alert>}
        {pending && <Card mb="lg" withBorder><Stack gap="sm"><Group justify="space-between"><Title order={5}>{t('host.items.' + pending.section)} · {t('host.pending')}</Title><Badge color="orange">{pending.phase}</Badge></Group><Text size="sm">{t('host.deadline', { time: new Date(pending.deadline * 1000).toLocaleString() })}</Text><Text size="sm">{t('host.verifyHint')}</Text><Checkbox checked={verified} onChange={e => setVerified(e.currentTarget.checked)} label={t('host.verified')} /><Group><Button disabled={!online || busy || !verified || pending.phase !== 'waiting' || now >= pending.deadline * 1000} onClick={() => run.mutate({ action: 'confirm', transaction: pending.id, verified: true })}>{t('host.finish')}</Button><Button variant="default" disabled={!online || busy || !pending.id} onClick={() => run.mutate({ action: 'rollback', transaction: pending.id })}>{t('host.restore')}</Button></Group></Stack></Card>}
        {groups.map((ids, index) => <Card key={index} mb="lg" withBorder><Title order={5} mb="md">{t('host.groups.' + index)}</Title><Stack gap="md">{ids.map((itemID, i) => {
          const item = snapshot?.items.find(x => x.id === itemID)
          return <div key={itemID}>{i > 0 && <Divider mb="md" />}<Group justify="space-between" align="flex-start" wrap="nowrap"><Stack gap={4} style={{ minWidth: 0, flex: 1 }}><Group gap="xs"><Text fw={600}>{t('host.items.' + itemID)}</Text><Badge color={item?.state === 'known' ? 'gray' : 'orange'} variant="light">{t(item?.state === 'known' ? 'host.observed' : 'host.unknown')}</Badge></Group>
            {item && Object.entries(item.values).map(([name, value]) => <Text key={name} size="sm" style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}><Text span c="dimmed">{label(name)}: </Text>{value || '—'}</Text>)}
            {item && !item.editable && <Text size="xs" c="dimmed">{t('host.reasons.' + item.reason, { defaultValue: item.reason })}</Text>}
          </Stack><Button variant="default" size="xs" disabled={!item?.editable || !online || busy || !!pending || stale} onClick={() => item && open(item)}>{t('common.edit')}</Button></Group></div>
        })}</Stack></Card>)}
      </Tabs.Panel>
      <Tabs.Panel value="history"><Text size="sm" c="dimmed" mb="md">{t('host.historyHint')}</Text><Stack>{q.data?.history.filter(j => j.params.action !== 'inspect').map(j => <Card key={j.id} withBorder><Group justify="space-between"><Text fw={600}>{j.params.section ? t('host.items.' + j.params.section) : t('host.actions.' + j.params.action)}</Text><Badge color={j.error || j.result?.outcome === 'failed' ? 'orange' : 'gray'}>{j.done_at ? t('host.outcomes.' + (j.result?.outcome || 'failed'), { defaultValue: j.result?.outcome || 'failed' }) : t('host.running')}</Badge></Group><Text size="xs" c="dimmed">{new Date(j.created_at).toLocaleString()}</Text>{j.error || j.result?.message ? <Text size="sm">{j.error || j.result?.message}</Text> : null}</Card>)}{!q.data?.history.some(j => j.params.action !== 'inspect') && <Text c="dimmed">{t('common.empty')}</Text>}</Stack></Tabs.Panel>
    </Tabs>
    <Modal opened={!!editing} onClose={() => !run.isPending && setEditing(null)} title={editing && t('host.items.' + editing.id)} size="lg" closeOnClickOutside={!run.isPending}>
      {editing && <Stack><Alert>{t('host.hints.' + editing.id)}</Alert>{review ? <><Title order={5}>{t('host.review')}</Title>{Object.entries(values).map(([name, value]) => <div key={name}><Text size="sm" c="dimmed">{label(name)}</Text><Code block style={{ overflowWrap: 'anywhere', whiteSpace: 'pre-wrap' }}>{value}</Code></div>)}<Text size="sm">{t('host.applyHint')}</Text><Group justify="flex-end"><Button variant="default" disabled={run.isPending} onClick={() => setReview(false)}>{t('host.previous')}</Button><Button loading={run.isPending} disabled={!online || busy || !!pending} onClick={() => run.mutate({ action: 'apply', section: editing.id, values: Object.fromEntries(Object.entries(values).map(([k, v]) => [k, v.trim()])), revision })}>{t('host.apply')}</Button></Group></> : <><SimpleGrid cols={{ base: 1, sm: editing.id === 'admin' ? 1 : 2 }}>{Object.entries(values).map(([k, v]) => field(k, v))}</SimpleGrid><Group justify="flex-end"><Button disabled={Object.values(values).some(v => !v.trim())} onClick={() => setReview(true)}>{t('host.review')}</Button></Group></>}</Stack>}
    </Modal>
  </>
}
