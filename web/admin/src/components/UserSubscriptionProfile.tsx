import { Link } from 'react-router-dom'
import { Button, Group, Select, Stack, Text } from '@mantine/core'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '../lib/api'
import { useAuth } from '../lib/auth'
import { toast } from '../lib/notify'
export function UserSubscriptionProfile({ userID, subURL }: { userID: number; subURL: string }) {
 const { t } = useTranslation()
 const { me } = useAuth()
 const qc = useQueryClient()
 const path = `/api/admin/users/${userID}/subscription-profile`
 const q = useQuery({ queryKey: ['user-sub-profile', userID], queryFn: () => api.get<{ profile_id: number | null; effective: { name: string; page: boolean } | null }>(path) })
 const choices = useQuery({ queryKey: ['user-profile-choices'], queryFn: () => api.get<{ id: number; name: string; default: boolean }[]>('/api/admin/users/subscription-profiles') })
 const save = useMutation({ mutationFn: (id: string | null) => api.put(path, { profile_id: id ? Number(id) : null }), onSuccess: () => { qc.invalidateQueries({ queryKey: ['user-sub-profile', userID] }); toast.ok(t('common.saved')) }, onError: toast.err })
 let pageURL = ''
 if (subURL) { const u = new URL(subURL, window.location.origin); u.searchParams.set('client', 'page'); pageURL = u.toString() }
 return <Stack gap="xs"><Group align="flex-start"><Select label={t('subProfiles.assignment')} value={q.data?.profile_id ? String(q.data.profile_id) : ''} disabled={me?.role === 'support' || save.isPending || !q.data} allowDeselect={false} data={[{ value: '', label: t('subProfiles.inherit') }, ...(choices.data ?? []).map(p => ({ value: String(p.id), label: p.name }))]} onChange={v => save.mutate(v)} flex={1} />{q.data?.effective?.page && pageURL && me?.role !== 'support' && <Button component="a" href={pageURL} target="_blank" rel="noreferrer" mt={25} variant="default">{t('subProfiles.openPage')}</Button>}</Group><Text size="xs" c="dimmed">{t('subProfiles.effective')}: {q.data?.effective?.name || t('subProfiles.legacy')}</Text>{me?.role === 'admin' && <Button component={Link} to="/sub-templates?mode=profiles" variant="subtle" size="xs" style={{ alignSelf: 'flex-start' }}>{t('workspace.templates')}</Button>}</Stack>
}
