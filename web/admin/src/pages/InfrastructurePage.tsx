import { Alert, Badge, Button, Card, Checkbox, Group, Modal, NumberInput, Pagination, Select, SimpleGrid, Stack, Switch, Table, Tabs, Text, Textarea, TextInput } from '@mantine/core'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useSearchParams } from 'react-router-dom'
import { api, type Node } from '../lib/api'
import { useAuth } from '../lib/auth'
import { toast } from '../lib/notify'
import { PageHeader } from '../components/PageHeader'
import { useURLChoice } from '../lib/use-url-choice'

type Supplier = { id: number; name: string; url: string; contact: string; notes: string }
type Asset = { id: number; supplier_id: number; supplier_name: string; node_id: number | null; name: string; external_ref: string; currency: string; amount_minor: number; period_months: number; anchor_day: number; next_due: string; remind_days: number; active: boolean; notes: string; revision: number }
type Payment = { id: number; asset_id: number; supplier_name: string; asset_name: string; currency: string; amount_minor: number; paid_date: string; period_start: string; next_due: string; reference: string; notes: string }
type Overview = { suppliers: Supplier[]; assets: Asset[]; paid_totals: { currency: string; amount_minor: number }[] }
type PaymentDraft = { asset: Asset; request_key: string; amount_minor: number; paid_date: string; reference: string; notes: string; confirm: boolean }
const base = '/api/admin/settings/infrastructure'
const today = () => new Date().toISOString().slice(0, 10)
function money(amount: number, currency: string) { try { const f = new Intl.NumberFormat(undefined, { style: 'currency', currency }); return f.format(amount / 10 ** (f.resolvedOptions().maximumFractionDigits ?? 2)) } catch { return `${currency} ${amount}` } }
function afterRenewal(a: Asset) { if (!a.period_months) return a.next_due; const [y, m] = a.next_due.split('-').map(Number); const d = new Date(Date.UTC(y, m - 1 + a.period_months, 1)); const last = new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + 1, 0)).getUTCDate(); d.setUTCDate(Math.min(a.anchor_day, last)); return d.toISOString().slice(0, 10) }
const newAsset = (supplier: number, node: number | null): Asset => ({ id: 0, supplier_id: supplier, supplier_name: '', node_id: node, name: '', external_ref: '', currency: 'USD', amount_minor: 0, period_months: 1, anchor_day: new Date().getUTCDate(), next_due: today(), remind_days: 7, active: true, notes: '', revision: 0 })

export default function InfrastructurePage() {
  const { t } = useTranslation()
  const { me } = useAuth()
  const allowed = me?.role === 'admin'
  const [params] = useSearchParams()
  const qc = useQueryClient()
  const q = useQuery({ queryKey: ['infrastructure'], queryFn: () => api.get<Overview>(base), enabled: allowed })
  const nodes = useQuery({ queryKey: ['nodes'], queryFn: () => api.get<Node[]>('/api/admin/nodes'), enabled: allowed })
  const [supplier, setSupplier] = useState<Supplier | null>(null)
  const [asset, setAsset] = useState<Asset | null>(null)
  const [pay, setPay] = useState<PaymentDraft | null>(null)
  const [remove, setRemove] = useState<{ kind: 'suppliers' | 'assets'; id: number; name: string; revision?: number } | null>(null)
  const [search, setSearch] = useState('')
  const [filter, setFilter] = useURLChoice('filter', ['all', 'active', 'due', 'overdue'], 'active')
  const [historyAsset, setHistoryAsset] = useState<string | null>(null)
  const [page, setPage] = useState(1)
  const history = useQuery({ queryKey: ['infra-payments', historyAsset, page], queryFn: () => api.get<{ items: Payment[]; total: number }>(`${base}/payments?offset=${(page - 1) * 100}${historyAsset ? `&asset_id=${historyAsset}` : ''}`), enabled: allowed })
  const refresh = () => { qc.invalidateQueries({ queryKey: ['infrastructure'] }); qc.invalidateQueries({ queryKey: ['infra-payments'] }) }
  const saveSupplier = useMutation({ mutationFn: () => supplier!.id ? api.put(`${base}/suppliers/${supplier!.id}`, supplier) : api.post(`${base}/suppliers`, supplier), onSuccess: () => { setSupplier(null); refresh(); toast.ok(t('common.saved')) }, onError: toast.err })
  const saveAsset = useMutation({ mutationFn: () => asset!.id ? api.put(`${base}/assets/${asset!.id}`, asset) : api.post(`${base}/assets`, asset), onSuccess: () => { setAsset(null); refresh(); toast.ok(t('common.saved')) }, onError: toast.err })
  const record = useMutation({ mutationFn: () => api.post(`${base}/assets/${pay!.asset.id}/payments`, { request_key: pay!.request_key, revision: pay!.asset.revision, amount_minor: pay!.amount_minor, paid_date: pay!.paid_date, reference: pay!.reference, notes: pay!.notes }), onSuccess: () => { setPay(null); refresh(); toast.ok(t('common.saved')) }, onError: e => { toast.err(e); refresh() } })
  const del = useMutation({ mutationFn: () => api.del(`${base}/${remove!.kind}/${remove!.id}${remove!.revision ? `?revision=${remove!.revision}` : ''}`), onSuccess: () => { setRemove(null); refresh() }, onError: toast.err })
  const importInfo = useMutation({ mutationFn: () => api.get<{ probe: { info: { provider?: string; price?: string; expires_at?: string; note?: string } } }>(`/api/admin/nodes/${asset!.node_id}/probe`), onSuccess: data => { const info = data.probe.info; setAsset(a => a && ({ ...a, name: a.name || nodes.data?.find(n => n.id === a.node_id)?.name || '', supplier_id: q.data?.suppliers.find(s => s.name === info.provider)?.id ?? a.supplier_id, next_due: info.expires_at || a.next_due, anchor_day: Number(info.expires_at?.slice(8)) || a.anchor_day, notes: [a.notes, info.provider, info.price, info.note].filter(Boolean).join('\n') })); toast.ok(t('infra.imported')) }, onError: toast.err })
  const editAsset = <K extends keyof Asset>(k: K, value: Asset[K]) => setAsset(a => a && ({ ...a, [k]: value }))
  const suppliers = q.data?.suppliers ?? []
  const assets = q.data?.assets ?? []
  const due = (a: Asset) => a.active && new Date(`${a.next_due}T00:00:00Z`).getTime() <= new Date(`${today()}T00:00:00Z`).getTime() + a.remind_days * 86400e3
  const shown = assets.filter(a => (!params.get('node') || String(a.node_id) === params.get('node')) && `${a.name} ${a.supplier_name} ${a.external_ref}`.toLowerCase().includes(search.toLowerCase()) && (filter === 'all' || filter === 'active' && a.active || filter === 'due' && due(a) || filter === 'overdue' && a.active && a.next_due < today()))
  if (!allowed) return <Alert>{t('infra.adminOnly')}</Alert>
  return <>
    <PageHeader title={t('infra.title')} subtitle={t('infra.hint')} />
    <Stack>
      {q.error && <Alert color="red">{String(q.error)}</Alert>}
      <SimpleGrid cols={{ base: 1, sm: 3 }}><Card><Text c="dimmed" size="sm">{t('infra.active')}</Text><Text size="xl" fw={600}>{assets.filter(a => a.active).length}</Text></Card><Card><Text c="dimmed" size="sm">{t('infra.due')}</Text><Text size="xl" fw={600}>{assets.filter(due).length}</Text></Card><Card><Text c="dimmed" size="sm">{t('infra.totalPaid')}</Text>{q.data?.paid_totals.length ? q.data.paid_totals.map(c => <Text key={c.currency} fw={600}>{money(c.amount_minor, c.currency)}</Text>) : <Text>—</Text>}</Card></SimpleGrid>
      <Tabs defaultValue="assets"><Tabs.List><Tabs.Tab value="assets">{t('infra.assets')}</Tabs.Tab><Tabs.Tab value="suppliers">{t('infra.suppliers')}</Tabs.Tab><Tabs.Tab value="history">{t('infra.history')}</Tabs.Tab></Tabs.List>
        <Tabs.Panel value="assets" pt="md"><Card><Stack>
          <Group align="flex-end"><TextInput label={t('infra.search')} value={search} onChange={e => setSearch(e.target.value)} style={{ flex: 1 }} /><Select label={t('infra.status')} value={filter} onChange={value => setFilter(value ?? 'active')} allowDeselect={false} data={['active', 'due', 'overdue', 'all'].map(v => ({ value: v, label: t(`infra.${v}`) }))} /><Button disabled={!suppliers.length} onClick={() => setAsset(newAsset(suppliers[0]?.id ?? 0, Number(params.get('node')) || null))}>{t('infra.addAsset')}</Button></Group>
          {!suppliers.length && <Text size="sm" c="dimmed">{t('infra.needSupplier')}</Text>}
          {params.get('node') && <Button component={Link} to="/infrastructure" size="xs" variant="subtle">{t('infra.all')}</Button>}
          <Table.ScrollContainer minWidth={760}><Table striped><Table.Thead><Table.Tr>{['name', 'supplier', 'amount', 'nextDue', 'status', 'actions'].map(k => <Table.Th key={k}>{t(`infra.${k}`)}</Table.Th>)}</Table.Tr></Table.Thead><Table.Tbody>{shown.map(a => <Table.Tr key={a.id}><Table.Td><Text fw={500}>{a.name}</Text>{a.node_id && <Button size="compact-xs" variant="subtle" component={Link} to={`/nodes/${a.node_id}`}>{nodes.data?.find(n => n.id === a.node_id)?.name || `#${a.node_id}`}</Button>}<Text size="xs" c="dimmed">{a.external_ref}</Text></Table.Td><Table.Td>{a.supplier_name}</Table.Td><Table.Td>{money(a.amount_minor, a.currency)}<Text size="xs" c="dimmed">{a.period_months ? t('infra.everyMonths', { count: a.period_months }) : t('infra.oneTime')}</Text></Table.Td><Table.Td>{a.next_due}</Table.Td><Table.Td><Badge variant="light" color={!a.active ? 'gray' : a.next_due < today() ? 'red' : due(a) ? 'orange' : 'teal'}>{t(!a.active ? 'infra.archived' : a.next_due < today() ? 'infra.overdue' : due(a) ? 'infra.due' : 'infra.active')}</Badge></Table.Td><Table.Td><Group gap={4}><Button size="compact-xs" variant="subtle" onClick={() => setAsset({ ...a })}>{t('infra.edit')}</Button><Button size="compact-xs" variant="light" disabled={!a.active} onClick={() => setPay({ asset: a, request_key: crypto.randomUUID(), amount_minor: a.amount_minor, paid_date: today(), reference: '', notes: '', confirm: false })}>{t('infra.record')}</Button><Button size="compact-xs" color="red" variant="subtle" onClick={() => setRemove({ kind: 'assets', id: a.id, name: a.name, revision: a.revision })}>{t('common.delete')}</Button></Group></Table.Td></Table.Tr>)}</Table.Tbody></Table></Table.ScrollContainer>
          <Text size="xs" c="dimmed">{t('infra.remindersHint')}</Text>
        </Stack></Card></Tabs.Panel>
        <Tabs.Panel value="suppliers" pt="md"><Card><Stack><Group justify="flex-end"><Button onClick={() => setSupplier({ id: 0, name: '', url: '', contact: '', notes: '' })}>{t('infra.addSupplier')}</Button></Group><Table.ScrollContainer minWidth={500}><Table><Table.Thead><Table.Tr>{['name', 'url', 'contact', 'actions'].map(k => <Table.Th key={k}>{t(`infra.${k}`)}</Table.Th>)}</Table.Tr></Table.Thead><Table.Tbody>{suppliers.map(s => <Table.Tr key={s.id}><Table.Td>{s.name}</Table.Td><Text component="td" size="sm" maw={300} style={{ overflowWrap: 'anywhere' }}>{s.url}</Text><Table.Td>{s.contact}</Table.Td><Table.Td><Group gap={4}><Button size="compact-xs" variant="subtle" onClick={() => setSupplier({ ...s })}>{t('infra.edit')}</Button><Button size="compact-xs" color="red" variant="subtle" onClick={() => setRemove({ kind: 'suppliers', id: s.id, name: s.name })}>{t('common.delete')}</Button></Group></Table.Td></Table.Tr>)}</Table.Tbody></Table></Table.ScrollContainer></Stack></Card></Tabs.Panel>
        <Tabs.Panel value="history" pt="md"><Card><Stack><Text size="sm" c="dimmed">{t('infra.historyHint')}</Text><Select label={t('infra.asset')} clearable searchable data={assets.map(a => ({ value: String(a.id), label: a.name }))} value={historyAsset} onChange={v => { setHistoryAsset(v); setPage(1) }} />{history.error && <Alert color="red">{String(history.error)}</Alert>}<Table.ScrollContainer minWidth={700}><Table striped><Table.Thead><Table.Tr>{['paidDate', 'asset', 'supplier', 'amount', 'period', 'reference'].map(k => <Table.Th key={k}>{t(`infra.${k}`)}</Table.Th>)}</Table.Tr></Table.Thead><Table.Tbody>{history.data?.items.map(p => <Table.Tr key={p.id}><Table.Td>{p.paid_date}</Table.Td><Table.Td>{p.asset_name}</Table.Td><Table.Td>{p.supplier_name}</Table.Td><Table.Td>{money(p.amount_minor, p.currency)}</Table.Td><Table.Td>{p.period_start} → {p.next_due}</Table.Td><Table.Td><Text size="sm">{p.reference}</Text><Text size="xs" c="dimmed">{p.notes}</Text></Table.Td></Table.Tr>)}</Table.Tbody></Table></Table.ScrollContainer><Pagination total={Math.max(1, Math.ceil((history.data?.total ?? 0) / 100))} value={page} onChange={setPage} /></Stack></Card></Tabs.Panel>
      </Tabs>
    </Stack>
    <Modal opened={!!supplier} onClose={() => setSupplier(null)} title={t('infra.supplier')} size="lg"><Stack>
      {(['name', 'url', 'contact'] as const).map(k => <TextInput key={k} label={t(`infra.${k}`)} value={supplier?.[k] ?? ''} onChange={e => setSupplier(s => s && ({ ...s, [k]: e.target.value }))} />)}<Textarea label={t('infra.notes')} value={supplier?.notes ?? ''} onChange={e => setSupplier(s => s && ({ ...s, notes: e.target.value }))} /><Button disabled={!supplier?.name.trim()} loading={saveSupplier.isPending} onClick={() => saveSupplier.mutate()}>{t('common.save')}</Button>
    </Stack></Modal>
    <Modal opened={!!asset} onClose={() => setAsset(null)} title={t('infra.asset')} size="lg"><Stack>{asset && <>
      <SimpleGrid cols={{ base: 1, sm: 2 }}><TextInput label={t('infra.name')} value={asset.name} onChange={e => editAsset('name', e.target.value)} /><Select label={t('infra.supplier')} data={suppliers.map(s => ({ value: String(s.id), label: s.name }))} value={String(asset.supplier_id)} onChange={v => editAsset('supplier_id', Number(v))} allowDeselect={false} searchable /></SimpleGrid>
      <Group align="flex-start"><Select label={t('infra.node')} description={t('infra.nodeHint')} style={{ flex: '1 1 240px', minWidth: 0 }} data={(nodes.data ?? []).map(n => ({ value: String(n.id), label: n.name }))} clearable searchable value={asset.node_id ? String(asset.node_id) : null} onChange={v => editAsset('node_id', v ? Number(v) : null)} /><Button mt={{ base: 0, sm: 25 }} variant="light" disabled={!asset.node_id} loading={importInfo.isPending} onClick={() => importInfo.mutate()}>{t('infra.importInfo')}</Button></Group>
      <TextInput label={t('infra.externalRef')} value={asset.external_ref} onChange={e => editAsset('external_ref', e.target.value)} />
      <SimpleGrid cols={{ base: 1, sm: 2 }}><TextInput label={t('infra.currency')} maxLength={3} value={asset.currency} onChange={e => editAsset('currency', e.target.value.toUpperCase())} /><NumberInput label={t('infra.amountMinor')} description={t('infra.minorHint')} allowDecimal={false} min={0} max={1e12} value={asset.amount_minor} onChange={v => editAsset('amount_minor', Number(v))} /></SimpleGrid>
      <Text size="sm">{money(asset.amount_minor, asset.currency)}</Text>
      <SimpleGrid cols={{ base: 1, sm: 2 }}><NumberInput label={t('infra.periodMonths')} description={t('infra.periodHint')} allowDecimal={false} min={0} max={120} value={asset.period_months} onChange={v => editAsset('period_months', Number(v))} /><NumberInput label={t('infra.anchorDay')} description={t('infra.anchorHint')} allowDecimal={false} min={1} max={31} value={asset.anchor_day} onChange={v => editAsset('anchor_day', Number(v))} /><TextInput type="date" label={t('infra.nextDue')} value={asset.next_due} onChange={e => editAsset('next_due', e.target.value)} /><NumberInput label={t('infra.remindDays')} allowDecimal={false} min={0} max={90} value={asset.remind_days} onChange={v => editAsset('remind_days', Number(v))} /></SimpleGrid>
      <Switch label={t('infra.active')} checked={asset.active} onChange={e => editAsset('active', e.target.checked)} /><Textarea label={t('infra.notes')} value={asset.notes} onChange={e => editAsset('notes', e.target.value)} /><Button disabled={!asset.name.trim() || !asset.supplier_id || !asset.next_due} loading={saveAsset.isPending} onClick={() => saveAsset.mutate()}>{t('common.save')}</Button>
    </>}</Stack></Modal>
    <Modal opened={!!pay} onClose={() => setPay(null)} title={t('infra.record')}><Stack>{pay && <>
      <Text fw={600}>{pay.asset.name} · {pay.asset.supplier_name}</Text><Text size="sm">{t('infra.nextDue')}: {pay.asset.next_due} → {afterRenewal(pay.asset)}{!pay.asset.period_months && ` (${t('infra.archived')})`}</Text>
      <NumberInput label={`${t('infra.amountMinor')} (${pay.asset.currency})`} allowDecimal={false} min={0} max={1e12} value={pay.amount_minor} onChange={v => setPay(p => p && ({ ...p, amount_minor: Number(v) }))} /><Text>{money(pay.amount_minor, pay.asset.currency)}</Text><TextInput type="date" label={t('infra.paidDate')} value={pay.paid_date} onChange={e => setPay(p => p && ({ ...p, paid_date: e.target.value }))} /><TextInput label={t('infra.reference')} value={pay.reference} onChange={e => setPay(p => p && ({ ...p, reference: e.target.value }))} /><Textarea label={t('infra.notes')} value={pay.notes} onChange={e => setPay(p => p && ({ ...p, notes: e.target.value }))} /><Checkbox label={t('infra.confirmPayment')} checked={pay.confirm} onChange={e => setPay(p => p && ({ ...p, confirm: e.target.checked }))} /><Button disabled={!pay.confirm || !pay.paid_date} loading={record.isPending} onClick={() => record.mutate()}>{t('infra.record')}</Button>
    </>}</Stack></Modal>
    <Modal opened={!!remove} onClose={() => setRemove(null)} title={t('common.delete')}><Stack><Text>{remove?.name}</Text><Text size="sm" c="dimmed">{t('infra.deleteHint')}</Text><Button color="red" loading={del.isPending} onClick={() => del.mutate()}>{t('common.delete')}</Button></Stack></Modal>
  </>
}
