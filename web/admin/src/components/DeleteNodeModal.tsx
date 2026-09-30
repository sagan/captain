import { Alert, Button, Checkbox, Code, Group, Modal, Radio, Stack, Text, TextInput } from '@mantine/core'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api, type Node, type NodeJob } from '../lib/api'
import { useAuth } from '../lib/auth'
import { toast } from '../lib/notify'

type Mode = 'standalone' | 'uninstall' | 'record'
type RemovalJob = Omit<NodeJob, 'params' | 'result'> & { params: { mode: 'standalone' | 'uninstall'; keep_data?: boolean }; result?: { phase?: string; listen?: string; login_setup?: boolean } }

export function DeleteNodeModal({ node, opened, onClose, onDeleted }: { node: Node; opened: boolean; onClose: () => void; onDeleted: () => void }) {
  const { t } = useTranslation()
  const { me } = useAuth()
  const remoteAllowed = me?.role === 'admin' && node.paired && node.online
  const [mode, setMode] = useState<Mode>('standalone')
  const [keepData, setKeepData] = useState(false)
  const [confirmation, setConfirmation] = useState('')
  const latest = useQuery({ queryKey: ['node-removal', node.id], queryFn: () => api.get<RemovalJob | null>(`/api/admin/nodes/${node.id}/removal`), enabled: opened, refetchInterval: opened ? 2000 : false })
  const existing = latest.data
  const pending = !!existing && !existing.done_at
  const finished = !!existing?.done_at && !existing.error && existing.result?.phase === 'complete'
  const selectedMode = (pending || finished) && existing ? existing.params.mode : mode
  const remove = useMutation({
    mutationFn: async () => {
      if (selectedMode === 'record') {
        await api.del(`/api/admin/nodes/${node.id}`)
        return
      }
      let job = await api.get<RemovalJob | null>(`/api/admin/nodes/${node.id}/removal`)
      if (!job || (job.done_at && job.error)) {
        await api.post(`/api/admin/nodes/${node.id}/removal`, { mode: selectedMode, keep_data: keepData })
      }
      const deadline = Date.now() + 240_000
      while (Date.now() < deadline) {
        job = await api.get<RemovalJob | null>(`/api/admin/nodes/${node.id}/removal`)
        if (job?.done_at) {
          if (job.error) throw new Error(job.error)
          if (job.result?.phase !== 'complete') throw new Error(t('nodeRemoval.incomplete'))
          await api.del(`/api/admin/nodes/${node.id}?removal_job=${encodeURIComponent(job.id)}`)
          return
        }
        await new Promise((resolve) => setTimeout(resolve, 2000))
      }
      throw new Error(t('nodeRemoval.timeout'))
    },
    onSuccess: () => { toast.ok(t('common.deleted')); onDeleted() },
    onError: toast.err,
  })
  return <Modal opened={opened} onClose={() => { if (!remove.isPending) onClose() }} title={t('nodeRemoval.title', { name: node.name })} closeOnClickOutside={!remove.isPending} closeOnEscape={!remove.isPending} withCloseButton={!remove.isPending}>
    <Stack>
      <Text size="sm">{t('nodeRemoval.intro')}</Text>
      <Radio.Group value={selectedMode} onChange={(v) => setMode(v as Mode)} label={t('nodeRemoval.action')}>
        <Stack gap="xs" mt="xs">
          <Radio value="standalone" label={t('nodeRemoval.standalone')} disabled={!remoteAllowed || remove.isPending || pending || finished} />
          <Radio value="uninstall" label={t('nodeRemoval.uninstall')} disabled={!remoteAllowed || remove.isPending || pending || finished} />
          <Radio value="record" label={t('nodeRemoval.record')} disabled={remove.isPending || pending || finished} />
        </Stack>
      </Radio.Group>
      <Text size="xs" c="dimmed">{t('nodeRemoval.requires')}</Text>
      {selectedMode === 'standalone' && <Alert color="blue"><Stack gap="xs"><Text size="sm">{t('nodeRemoval.standaloneHint')}</Text><Text size="sm">{t('nodeRemoval.loginHint')}</Text><Code>bosun admin set -user admin -password 'NEW_PASSWORD'</Code></Stack></Alert>}
      {selectedMode === 'uninstall' && <><Alert color="red">{t('nodeRemoval.uninstallHint')}</Alert><Checkbox checked={(pending || finished) && existing ? !!existing.params.keep_data : keepData} disabled={remove.isPending || pending || finished} onChange={(e) => setKeepData(e.currentTarget.checked)} label={t('nodeRemoval.keepData')} /></>}
      {selectedMode === 'record' && <Alert color="orange">{t('nodeRemoval.recordHint')}</Alert>}
      {(pending || remove.isPending) && <Alert color="blue">{t('nodeRemoval.waiting')}</Alert>}
      {finished && <Alert color="teal">{t('nodeRemoval.finished')}</Alert>}
      {existing?.error && <Alert color="red">{existing.error}</Alert>}
      <TextInput label={t('nodeRemoval.confirm', { name: node.name })} value={confirmation} onChange={(e) => setConfirmation(e.currentTarget.value)} disabled={remove.isPending} />
      <Group justify="flex-end"><Button variant="default" onClick={onClose} disabled={remove.isPending}>{t('common.cancel')}</Button><Button color="red" loading={remove.isPending} disabled={confirmation !== node.name || latest.isLoading || latest.isError || (selectedMode !== 'record' && !remoteAllowed && !pending && !finished)} onClick={() => remove.mutate()}>{pending || finished ? t('nodeRemoval.resume') : t('common.delete')}</Button></Group>
    </Stack>
  </Modal>
}
