import { Alert, Badge, Button, Card, Code, Group, Stack, Text } from '@mantine/core'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api, runNodeJob } from '../lib/api'
import { useAuth } from '../lib/auth'
import { when } from '../lib/format'
import { toast } from '../lib/notify'

export type Readiness = { effective_mode?: string; checked_at: number; ready: boolean; checks: { id: string; ok: boolean }[] }
export function PrivateAccessReadiness({ nodeID, compact = false }: { nodeID: number; compact?: boolean }) {
  const { t } = useTranslation()
  const { me } = useAuth()
  const qc = useQueryClient()
  const [now, setNow] = useState(() => Date.now() / 1000)
  useEffect(() => { const timer = setInterval(() => setNow(Date.now() / 1000), 1000); return () => clearInterval(timer) }, [])
  const controller = useRef<AbortController | null>(null)
  useEffect(() => () => controller.current?.abort(), [nodeID])
  const endpoint = `/api/admin/nodes/${nodeID}/core-options`
  const q = useQuery({ queryKey: ['core-options', endpoint, 'readiness'], queryFn: () => api.get<{ private_access_readiness?: Readiness }>(endpoint), refetchInterval: 15_000 })
  const refresh = useMutation({ mutationFn: async () => {
    controller.current?.abort()
    controller.current = new AbortController()
    return runNodeJob<Readiness>(nodeID, 'private_access_check', {}, 90_000, controller.current.signal)
  }, onSuccess: () => { qc.invalidateQueries({ queryKey: ['core-options'] }); qc.invalidateQueries({ queryKey: ['node'] }) }, onError: error => { if (!controller.current?.signal.aborted) toast.err(error) } })
  const report = q.data?.private_access_readiness
  const detail = report && refresh.data && refresh.data.checked_at >= report.checked_at ? refresh.data : report
  const stale = !detail || now - detail.checked_at >= 180 || detail.checked_at - now > 30
  const effectiveCoreOnly = !stale && detail?.effective_mode === 'core_only'
  const checks = stale ? [] : detail.checks.map(c => effectiveCoreOnly && c.id === 'guard' ? {id:'guardDisabled',ok:!c.ok} : c)
  const ready = checks.length > 0 && checks.every(c => c.ok)
  return <Card withBorder mb={compact ? 0 : 'lg'}>
    <Stack gap="xs">
      <Group justify="space-between"><Text fw={600}>{t('privateReady.title')}</Text><Button size="xs" variant="light" loading={refresh.isPending || q.isFetching} disabled={refresh.isPending} onClick={() => { if (report && !stale && me?.role === 'admin') refresh.mutate(); else void q.refetch() }}>{t('privateReady.refresh')}</Button></Group>
      <Text size="xs" c="dimmed">{t('privateReady.hint')}</Text>
      <Text size="sm">{t(stale || !detail?.effective_mode ? 'privateAccess.runtimeUnknown' : effectiveCoreOnly ? 'privateAccess.coreOnly' : 'privateAccess.dual')}</Text>
      {stale ? <Alert color="orange">{t('privateReady.unknown')}</Alert> : <>
        <Text size="xs" c="dimmed">{when(new Date(detail.checked_at * 1000).toISOString())}</Text>
        {checks.filter(c => !compact || !c.ok).map(c => <Stack key={c.id} gap={3}>
          <Group gap="xs"><Badge color={c.ok ? 'teal' : 'orange'}>{c.ok ? '✓' : '!'}</Badge><Text size="sm">{t(`privateReady.checks.${c.id}`)}</Text></Group>
          {!c.ok && <Text size="sm" c="dimmed">{t(`privateReady.fixes.${c.id}`)}</Text>}
        </Stack>)}
        {!checks.some(c => c.id === 'kernel') && <Text size="xs" c="dimmed">{t('privateReady.kernelPending')}</Text>}
        {ready && <Text size="sm" c="teal">{t('privateReady.ready')}</Text>}
        {!ready && <><Code block>bosun prepare-private-access -c /etc/bosun/config.yaml</Code><Text size="xs" c="dimmed">{t('privateReady.prepare')}</Text></>}
      </>}
    </Stack>
  </Card>
}
