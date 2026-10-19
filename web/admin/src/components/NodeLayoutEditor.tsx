import { ActionIcon, Alert, Button, Card, Checkbox, Group, Modal, Select, Stack, Text, TextInput } from '@mantine/core'
import { DndContext, PointerSensor, KeyboardSensor, closestCenter, useSensor, useSensors } from '@dnd-kit/core'
import { SortableContext, useSortable, sortableKeyboardCoordinates, verticalListSortingStrategy, arrayMove } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { IconArrowUp, IconArrowDown, IconGripVertical, IconPencil, IconTrash } from '@tabler/icons-react'
import { useState, type ReactNode } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api, ApiError, type Node } from '../lib/api'
import { toast } from '../lib/notify'

import { type NodeLayout } from '../lib/node-layout'

function SortRow({ id, children, index, count, move }: { id: string; children: ReactNode; index: number; count: number; move: (from: number, to: number) => void }) {
  const { t } = useTranslation()
  const { attributes, listeners, setNodeRef, transform, transition } = useSortable({ id })
  return <Card ref={setNodeRef} p="xs" style={{ transform: CSS.Transform.toString(transform), transition }}>
    <Group wrap="nowrap" gap="xs">
      <ActionIcon variant="subtle" style={{ touchAction: 'none', flexShrink: 0 }} aria-label={t('nodeLayout.drag')} {...attributes} {...listeners}><IconGripVertical size={18} /></ActionIcon>
      <div style={{ flex: 1, minWidth: 0 }}>{children}</div>
      <Stack gap={2}><ActionIcon variant="subtle" size="sm" aria-label={t('nodeLayout.up')} disabled={index === 0} onClick={() => move(index, index - 1)}><IconArrowUp size={16} /></ActionIcon><ActionIcon variant="subtle" size="sm" aria-label={t('nodeLayout.down')} disabled={index === count - 1} onClick={() => move(index, index + 1)}><IconArrowDown size={16} /></ActionIcon></Stack>
    </Group>
  </Card>
}
function SortList({ ids, move, children }: { ids: string[]; move: (from: number, to: number) => void; children: ReactNode }) {
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }), useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }))
  return <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={({ active, over }) => { if (over && active.id !== over.id) move(ids.indexOf(String(active.id)), ids.indexOf(String(over.id))) }}><SortableContext items={ids} strategy={verticalListSortingStrategy}><Stack gap="xs">{children}</Stack></SortableContext></DndContext>
}

export function NodeLayoutEditor({ initial, nodes, close, onSaved }: { initial: NodeLayout; nodes: Node[]; close: () => void; onSaved: () => void }) {
  const { t } = useTranslation(), qc = useQueryClient()
  const [draft, setDraft] = useState<NodeLayout>(() => structuredClone(initial))
  const [group, setGroup] = useState('')
  const [selected, setSelected] = useState<number[]>([])
  const [target, setTarget] = useState<string | null>(null)
  const [rename, setRename] = useState<string | null>(null), [name, setName] = useState('')
  const [conflict, setConflict] = useState(false)
  const save = useMutation({ mutationFn: () => api.put<NodeLayout>('/api/admin/nodes/layout', draft), onSuccess: () => { qc.invalidateQueries({ queryKey: ['nodes'] }); qc.invalidateQueries({ queryKey: ['node'] }); qc.invalidateQueries({ queryKey: ['monitoring'] }); qc.invalidateQueries({ queryKey: ['node-layout'] }); toast.ok(t('common.saved')); onSaved(); close() }, onError: e => { if (e instanceof ApiError && e.status === 409) setConflict(true); else toast.err(e) } })
  const options = [{ value: '', label: t('nodeLayout.ungrouped') }, ...draft.groups.map(g => ({ value: g, label: g }))]
  const members = draft.nodes.filter(n => n.group === group)
  const moveGroups = (from: number, to: number) => setDraft(d => ({ ...d, groups: arrayMove(d.groups, from, to) }))
  const moveNodes = (from: number, to: number) => {
    const reordered = arrayMove(members, from, to)
    let i = 0
    setDraft(d => ({ ...d, nodes: d.nodes.map(n => n.group === group ? reordered[i++] : n) }))
  }
  const applyName = () => {
    const value = name.trim()
    if (!value || new TextEncoder().encode(value).length > 128 || /[\p{Cc}<>&"'`]/u.test(value) || draft.groups.some(g => g === value && g !== rename)) { toast.err(t('nodeLayout.invalidName')); return }
    setDraft(d => ({ ...d, groups: rename === '' ? [...d.groups, value] : d.groups.map(g => g === rename ? value : g), nodes: d.nodes.map(n => n.group === rename && rename !== '' ? { ...n, group: value } : n) }))
    if (rename === group && rename !== '') setGroup(value)
    if (rename === target && rename !== '') setTarget(value)
    setRename(null)
  }
  const removeGroup = (g: string) => {
    setDraft(d => ({ ...d, groups: d.groups.filter(x => x !== g), nodes: d.nodes.map(n => n.group === g ? { ...n, group: '' } : n) }))
    if (group === g) { setGroup(''); setSelected([]) }
    if (target === g) setTarget(null)
  }
  return <Modal opened onClose={() => { if (!save.isPending) close() }} closeOnClickOutside={false} title={t('nodeLayout.title')} size="lg">
    <Stack>
      <Alert>{t('nodeLayout.hint')}</Alert>
      {conflict && <Alert color="orange">{t('nodeLayout.conflict')}</Alert>}
      <fieldset disabled={save.isPending} style={{ border: 0, padding: 0, margin: 0, minWidth: 0 }}><Stack>
      <Group justify="space-between"><Text fw={600}>{t('monitoring.group')}</Text><Button size="xs" variant="light" onClick={() => { setRename(''); setName('') }}>{t('nodeLayout.addGroup')}</Button></Group>
      <SortList ids={draft.groups} move={moveGroups}>{draft.groups.map((g, i) => <SortRow key={g} id={g} index={i} count={draft.groups.length} move={moveGroups}><Group justify="space-between" wrap="nowrap"><Text style={{ overflowWrap: 'anywhere' }}>{g}</Text><Group gap={4} wrap="nowrap"><ActionIcon variant="subtle" aria-label={t('nodeLayout.rename')} onClick={() => { setRename(g); setName(g) }}><IconPencil size={16} /></ActionIcon><ActionIcon variant="subtle" color="red" aria-label={t('nodeLayout.deleteGroup')} onClick={() => removeGroup(g)}><IconTrash size={16} /></ActionIcon></Group></Group></SortRow>)}</SortList>
      <Text size="xs" c="dimmed">{t('nodeLayout.deleteHint')}</Text>
      <Select label={t('nodeLayout.groupNodes')} value={group} onChange={v => { setGroup(v ?? ''); setSelected([]) }} data={options} allowDeselect={false} />
      <Checkbox label={t('nodeLayout.selectAll')} checked={members.length > 0 && selected.length === members.length} indeterminate={selected.length > 0 && selected.length < members.length} onChange={e => setSelected(e.currentTarget.checked ? members.map(n => n.id) : [])} />
      {selected.length > 0 && <Group align="flex-end"><Select style={{ flex: 1 }} label={t('nodeLayout.moveTo')} data={options} value={target} onChange={setTarget} /><Button disabled={target === null || target === group} onClick={() => { if (target === null) return; const chosen = new Set(selected); setDraft(d => ({ ...d, nodes: [...d.nodes.filter(n => !chosen.has(n.id)), ...d.nodes.filter(n => chosen.has(n.id)).map(n => ({ ...n, group: target }))] })); setSelected([]) }}>{t('nodeLayout.move')}</Button></Group>}
      <SortList ids={members.map(n => String(n.id))} move={moveNodes}>{members.map((n, i) => <SortRow key={n.id} id={String(n.id)} index={i} count={members.length} move={moveNodes}><Checkbox label={nodes.find(x => x.id === n.id)?.name ?? String(n.id)} checked={selected.includes(n.id)} onChange={e => setSelected(e.currentTarget.checked ? [...selected, n.id] : selected.filter(id => id !== n.id))} /></SortRow>)}</SortList>
      {members.length === 0 && <Text c="dimmed" ta="center">{t('common.empty')}</Text>}
      </Stack></fieldset>
      <Group justify="flex-end"><Button variant="default" disabled={save.isPending} onClick={close}>{t('common.cancel')}</Button><Button loading={save.isPending} disabled={conflict} onClick={() => save.mutate()}>{t('common.save')}</Button></Group>
    </Stack>
    <Modal opened={rename !== null} onClose={() => setRename(null)} title={t(rename === '' ? 'nodeLayout.addGroup' : 'nodeLayout.rename')}><form onSubmit={e => { e.preventDefault(); applyName() }}><Stack><TextInput data-autofocus label={t('monitoring.group')} value={name} onChange={e => setName(e.currentTarget.value)} /><Button type="submit">{t('common.save')}</Button></Stack></form></Modal>
  </Modal>
}
