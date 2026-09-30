import { Button, Card, Group, Modal, PasswordInput, Stack, Table, Text, TextInput, Title } from '@mantine/core'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../lib/api'
import { useAuth } from '../lib/auth'
import { when } from '../lib/format'
import { toast } from '../lib/notify'
import { passkeysAvailable, registerPasskey } from '../lib/passkeys'

type Key = { id: string; name: string; created_at: string; last_used_at: string | null }
export function PasskeysCard() {
  const { t } = useTranslation()
  const { me } = useAuth()
  const qc = useQueryClient()
  const q = useQuery({ queryKey: ['passkeys'], queryFn: () => api.get<{ available: boolean; items: Key[] }>('/api/admin/passkeys') })
  const [editing, setEditing] = useState<Key | 'new' | null>(null)
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [code, setCode] = useState('')
  const close = () => { setEditing(null); setPassword(''); setCode(''); setName('') }
  const save = useMutation({ mutationFn: () => editing === 'new' ? registerPasskey(name, password, code) : api.del(`/api/admin/passkeys/${(editing as Key).id}`, { password, code }), onSuccess: () => { close(); qc.invalidateQueries({ queryKey: ['passkeys'] }); toast.ok(t('common.saved')) }, onError: () => toast.err(t('passkeys.failed')) })
  return <Card>
    <Group justify="space-between"><Title order={5}>{t('passkeys.title')}</Title><Button size="xs" disabled={!q.data?.available || !passkeysAvailable() || (q.data?.items.length ?? 0) >= 10} onClick={() => setEditing('new')}>{t('passkeys.add')}</Button></Group>
    <Text size="sm" c="dimmed" my="sm">{t('passkeys.hint')}</Text>
    {q.data && !q.data.available && <Text size="sm" c="orange">{t('passkeys.unavailable')}</Text>}
    <Table.ScrollContainer minWidth={440}><Table><Table.Thead><Table.Tr><Table.Th>{t('passkeys.name')}</Table.Th><Table.Th>{t('passkeys.lastUsed')}</Table.Th><Table.Th /></Table.Tr></Table.Thead><Table.Tbody>{q.data?.items.map(key => <Table.Tr key={key.id}><Table.Td>{key.name}<Text size="xs" c="dimmed">{when(key.created_at)}</Text></Table.Td><Table.Td>{key.last_used_at ? when(key.last_used_at) : '—'}</Table.Td><Table.Td><Button size="compact-xs" color="red" variant="subtle" onClick={() => setEditing(key)}>{t('common.delete')}</Button></Table.Td></Table.Tr>)}</Table.Tbody></Table></Table.ScrollContainer>
    <Modal opened={editing !== null} onClose={() => { if (!save.isPending) close() }} title={editing === 'new' ? t('passkeys.add') : t('passkeys.revoke')}>
      <Stack>
        {editing === 'new' ? <TextInput label={t('passkeys.name')} maxLength={100} value={name} onChange={e => setName(e.currentTarget.value)} /> : <Text>{editing?.name}</Text>}
        <PasswordInput label={t('login.password')} autoComplete="current-password" value={password} onChange={e => setPassword(e.currentTarget.value)} />
        {me?.totp && <TextInput label={t('login.code')} value={code} onChange={e => setCode(e.currentTarget.value)} autoComplete="one-time-code" />}
        <Button color={editing === 'new' ? undefined : 'red'} disabled={!password || (editing === 'new' && !name.trim()) || (me?.totp && !code)} loading={save.isPending} onClick={() => save.mutate()}>{editing === 'new' ? t('passkeys.add') : t('passkeys.revoke')}</Button>
      </Stack>
    </Modal>
  </Card>
}
