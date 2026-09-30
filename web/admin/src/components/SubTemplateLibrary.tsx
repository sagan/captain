import { ActionIcon, Button, Card, Code, Group, Modal, Select, Stack, Table, Textarea, TextInput } from '@mantine/core'
import { modals } from '@mantine/modals'
import { IconCopy, IconPencil, IconTrash } from '@tabler/icons-react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api, ApiError } from '../lib/api'
import { toast } from '../lib/notify'
export type NamedSubTemplate = { id: number; name: string; format: string; body: string }
export type SubLibrary = { items: NamedSubTemplate[]; defaults: Record<string, string> }
export function SubTemplateLibrary() {
 const { t } = useTranslation()
 const qc = useQueryClient()
 const q = useQuery({ queryKey: ['sub-library'], queryFn: () => api.get<SubLibrary>('/api/admin/settings/sub-library/templates') })
 const [draft, setDraft] = useState<NamedSubTemplate | null>(null)
 const [rendered, setRendered] = useState('')
 const save = useMutation({ mutationFn: () => draft!.id ? api.put(`/api/admin/settings/sub-library/templates/${draft!.id}`, draft) : api.post('/api/admin/settings/sub-library/templates', draft), onSuccess: () => { setDraft(null); qc.invalidateQueries({ queryKey: ['sub-library'] }); toast.ok(t('common.saved')) }, onError: toast.err })
 const preview = useMutation({ mutationFn: () => api.post<{ body: string }>('/api/admin/settings/sub-library/templates/preview', draft), onSuccess: r => setRendered(r.body), onError: toast.err })
 const del = useMutation({ mutationFn: (id: number) => api.del(`/api/admin/settings/sub-library/templates/${id}`), onSuccess: () => qc.invalidateQueries({ queryKey: ['sub-library'] }), onError: e => toast.err(e instanceof ApiError && e.status === 409 ? t('subProfiles.inUse') : e) })
 const edit = (value: NamedSubTemplate) => { setDraft(value); setRendered('') }
 return <Card><Stack>
  <Group justify="flex-end"><Button onClick={() => edit({ id: 0, name: '', format: 'clash', body: q.data?.defaults.clash ?? '' })}>{t('subProfiles.newTemplate')}</Button></Group>
  <Table.ScrollContainer minWidth={480}><Table><Table.Thead><Table.Tr><Table.Th>{t('subProfiles.name')}</Table.Th><Table.Th>{t('subTemplates.format')}</Table.Th><Table.Th /></Table.Tr></Table.Thead><Table.Tbody>{q.data?.items.map(v => <Table.Tr key={v.id}><Table.Td>{v.name}</Table.Td><Table.Td>{v.format}</Table.Td><Table.Td><Group gap="xs"><ActionIcon variant="subtle" aria-label={t('common.edit')} onClick={() => edit(v)}><IconPencil size={16} /></ActionIcon><ActionIcon variant="subtle" aria-label={t('subProfiles.duplicate')} onClick={() => edit({ ...v, id: 0, name: `${v.name} (${t('subProfiles.copy')})` })}><IconCopy size={16} /></ActionIcon><ActionIcon color="red" variant="subtle" aria-label={t('common.delete')} onClick={() => modals.openConfirmModal({ title: t('common.delete'), children: v.name, labels: { confirm: t('common.delete'), cancel: t('common.cancel') }, onConfirm: () => del.mutate(v.id) })}><IconTrash size={16} /></ActionIcon></Group></Table.Td></Table.Tr>)}</Table.Tbody></Table></Table.ScrollContainer>
  <Modal opened={!!draft} onClose={() => setDraft(null)} title={t('subProfiles.library')} size="xl">
   {draft && <Stack>
    <Group grow align="flex-start"><TextInput label={t('subProfiles.name')} maxLength={100} value={draft.name} onChange={e => setDraft({ ...draft, name: e.currentTarget.value })} /><Select label={t('subTemplates.format')} disabled={draft.id > 0} allowDeselect={false} value={draft.format} data={Object.keys(q.data?.defaults ?? {})} onChange={f => f && setDraft({ ...draft, format: f, body: q.data?.defaults[f] ?? '' })} /></Group>
    <Textarea label={t('subProfiles.templateBody')} autosize minRows={14} maxRows={30} ff="monospace" value={draft.body} onChange={e => { setDraft({ ...draft, body: e.currentTarget.value }); setRendered('') }} />
    <Group justify="flex-end"><Button variant="default" loading={preview.isPending} onClick={() => preview.mutate()}>{t('subProfiles.previewSample')}</Button><Button loading={save.isPending} disabled={!draft.name.trim() || !draft.body.trim()} onClick={() => save.mutate()}>{t('common.save')}</Button></Group>
    {rendered && <Code block style={{ maxHeight: 320, overflow: 'auto' }}>{rendered}</Code>}
   </Stack>}
  </Modal>
 </Stack></Card>
}
