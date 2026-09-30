import { Button, Card, Group, Table, Text, TextInput, Title } from '@mantine/core'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api, type Group as UserGroup } from '../lib/api'
import { toast } from '../lib/notify'

export function UserGroupsPanel() {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [name, setName] = useState('')
  const groups = useQuery({ queryKey: ['groups'], queryFn: () => api.get<UserGroup[]>('/api/admin/groups') })
  const create = useMutation({
    mutationFn: (groupName: string) => api.post('/api/admin/groups', { Name: groupName }),
    onSuccess: () => {
      toast.ok(t('common.saved'))
      setName('')
      qc.invalidateQueries({ queryKey: ['groups'] })
    },
    onError: toast.err,
  })
  return <Card>
    <Title order={5} mb="sm">{t('users.groups')}</Title>
    <Table mb="md"><Table.Tbody>
      {(groups.data ?? []).map((g) => <Table.Tr key={g.ID}><Table.Td w={60}><Text c="dimmed">#{g.ID}</Text></Table.Td><Table.Td>{g.Name}</Table.Td></Table.Tr>)}
      {groups.data?.length === 0 && <Table.Tr><Table.Td colSpan={2}><Text c="dimmed" ta="center" py="md">{t('common.empty')}</Text></Table.Td></Table.Tr>}
    </Table.Tbody></Table>
    <form onSubmit={(e) => { e.preventDefault(); if (name.trim() && !create.isPending) create.mutate(name.trim()) }}>
      <Group align="flex-end">
        <TextInput label={t('users.groupName')} value={name} onChange={(e) => setName(e.currentTarget.value)} required disabled={create.isPending} />
        <Button type="submit" disabled={!name.trim()} loading={create.isPending}>{t('users.createGroup')}</Button>
      </Group>
    </form>
  </Card>
}
