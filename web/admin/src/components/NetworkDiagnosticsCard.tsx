import { Alert, Card, Select, Stack, Text, Title } from '@mantine/core'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { useLocation } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { api, type NodeJob } from '../lib/api'
import { useAuth } from '../lib/auth'
import type { DiagnosticRequest, DiagnosticResult } from '../lib/diagnostics'
import { DiagnosticForm, DiagnosticResultView } from './NetworkDiagnostic'

type DiagnosticJob = Omit<NodeJob, 'params' | 'result'> & { params: DiagnosticRequest; result?: DiagnosticResult }
export function NetworkDiagnosticsCard({ nodeID }: { nodeID: number }) {
 const { t } = useTranslation()
 const { me } = useAuth()
 const qc = useQueryClient()
 const { hash } = useLocation()
 const anchor = useRef<HTMLDivElement>(null)
 useEffect(() => {
  if (hash !== '#diagnostics') return
  const frame = requestAnimationFrame(() => anchor.current?.scrollIntoView({ block: 'start' }))
  return () => cancelAnimationFrame(frame)
 }, [hash, nodeID])
 const [selected, setSelected] = useState<string | null>(null)
 const key = ['network-diagnostics', nodeID]
 const q = useQuery({ queryKey: key, queryFn: () => api.get<DiagnosticJob[]>(`/api/admin/nodes/${nodeID}/network-diagnostics`), enabled: me?.role === 'admin', refetchInterval: (query) => query.state.data?.some((j) => !j.done_at) ? 2000 : 15_000 })
 const run = useMutation({ mutationFn: (params: DiagnosticRequest) => api.post<{ id: string }>(`/api/admin/nodes/${nodeID}/jobs`, { kind: 'network_diagnostic', params }), onSuccess: async ({ id }) => { setSelected(id); await qc.invalidateQueries({ queryKey: key }) } })
 if (me?.role !== 'admin') return null
 const job = q.data?.find((j) => j.id === selected) ?? q.data?.[0]
 const pending = run.isPending || !!q.data?.some((j) => !j.done_at)
 return <Card ref={anchor} id="diagnostics" mb="lg" style={{ scrollMarginTop: 80 }}><Stack gap="sm"><Title order={5}>{t('diagnostics.title')}</Title>
  <Text size="xs" c="dimmed">{t('diagnostics.requirement')}</Text>
  {q.isError && <Alert color="red">{q.error.message}</Alert>}
  <DiagnosticForm onRun={(v) => run.mutate(v)} pending={pending} />
  {run.isError && <Alert color="red">{run.error.message}</Alert>}
  <Select label={t('diagnostics.history')} placeholder={t('diagnostics.empty')} value={job?.id ?? null} onChange={setSelected} allowDeselect={false} data={(q.data ?? []).map((j) => ({ value: j.id, label: `${new Date(j.created_at).toLocaleString()} · ${t(`diagnostics.types.${j.params.type}`)} · ${j.params.target}` }))} />
  {job && <Text size="xs" c="dimmed" style={{ overflowWrap: 'anywhere' }}>{job.params.target}{job.params.source_ip ? ` · ${t('diagnostics.source')}: ${job.params.source_ip}` : ''}{job.params.resolver ? ` · DNS: ${job.params.resolver}` : ''}</Text>}
  {job?.error && <Alert color="orange">{job.error}</Alert>}
  {job?.result && <DiagnosticResultView result={job.result} />}
 </Stack></Card>
}
