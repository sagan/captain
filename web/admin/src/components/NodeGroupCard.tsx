import { Button, Card, Group, Select, Stack, Text } from '@mantine/core'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api, type Node } from '../lib/api'
import { toast } from '../lib/notify'
import { nodeLayoutQuery } from '../lib/node-layout'

export function NodeGroupCard({ node }: { node: Node }) {
  const { t } = useTranslation(), qc = useQueryClient()
  const q = useQuery(nodeLayoutQuery)
  const [group, setGroup] = useState(node.monitor_group ?? '')
  const save = useMutation({ mutationFn: () => api.put(`/api/admin/nodes/${node.id}/monitor-group`, { group }), onSuccess: () => { for (const key of ['nodes', 'node', 'monitoring', 'node-layout']) qc.invalidateQueries({ queryKey: [key] }); toast.ok(t('common.saved')) }, onError: toast.err })
  return <Card mb="lg"><Stack gap="xs"><Group align="flex-end"><Select style={{ flex: 1 }} label={t('monitoring.group')} value={group} allowDeselect={false} onChange={v => setGroup(v ?? '')} disabled={!q.data || q.isError || save.isPending} data={[{ value: '', label: t('nodeLayout.ungrouped') }, ...[...new Set([...(q.data?.groups ?? []), ...(group ? [group] : [])])].map(g => ({ value: g, label: g }))]} /><Button disabled={!q.data || q.isError || group === (node.monitor_group ?? '')} loading={save.isPending} onClick={() => save.mutate()}>{t('common.save')}</Button></Group><Text size="xs" c="dimmed">{t('nodeLayout.publicHint')}</Text></Stack></Card>
}
