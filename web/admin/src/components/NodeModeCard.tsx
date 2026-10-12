import { Alert, Badge, Button, Card, Group, Stack, Text } from '@mantine/core'
import { modals } from '@mantine/modals'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api, type Node } from '../lib/api'
import { toast } from '../lib/notify'

export function NodeModeCard({ node, canWrite }: { node: Node; canWrite: boolean }) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const mode = node.mode ?? 'full'
  const monitor = mode === 'monitor'
  const pending = node.paired && !!node.reported_mode && (node.reported_mode !== mode || node.reported_mode_revision !== node.mode_revision)
  const supported = node.mode_switch_available === true
  const change = useMutation({ mutationFn: () => api.put(`/api/admin/nodes/${node.id}/mode`, { mode: monitor ? 'full' : 'monitor', expected: mode, expected_revision: node.mode_revision ?? 0 }), onSuccess: () => { toast.ok(t('nodeMode.requested')); qc.invalidateQueries({ queryKey: ['node'] }); qc.invalidateQueries({ queryKey: ['nodes'] }) }, onError: toast.err })
  return <Card mb="lg"><Stack gap="sm">
    <Group justify="space-between" align="flex-start">
      <div><Text fw={600}>{t('nodeMode.title')}</Text><Text size="sm" c="dimmed" mt={4}>{(monitor ? t('nodeMode.monitorHint') : t('nodeMode.fullHint'))}</Text></div>
      <Badge color={monitor ? 'cyan' : 'indigo'}>{(monitor ? t('nodeMode.monitor') : t('nodeMode.full'))}</Badge>
    </Group>
    {pending && <Alert color="blue">{t('nodeMode.pending', { mode: (node.reported_mode === 'monitor' ? t('nodeMode.monitor') : t('nodeMode.full')) })}</Alert>}
    {node.paired && !supported && <Text size="sm" c="dimmed">{t('nodeMode.upgradeHint')}</Text>}
    {node.mode_error && <Alert color="red" title={t('nodeMode.failed')}>{node.mode_error}</Alert>}
    {canWrite && <Group justify="flex-end"><Button variant="light" loading={change.isPending} disabled={!supported} onClick={() => modals.openConfirmModal({ title: (monitor ? t('nodeMode.toFull') : t('nodeMode.toMonitor')), children: <Text size="sm">{(monitor ? t('nodeMode.confirmFull') : t('nodeMode.confirmMonitor'))}</Text>, labels: { confirm: t('common.confirm'), cancel: t('common.cancel') }, onConfirm: () => change.mutate() })}>{(monitor ? t('nodeMode.toFull') : t('nodeMode.toMonitor'))}</Button></Group>}
  </Stack></Card>
}
