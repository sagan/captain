import { Alert, Autocomplete, Stack, Switch, Text } from '@mantine/core'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '../lib/api'
import type { NodeDomainCheck } from '../lib/use-node-domain-check'

export function NodeDomainFields({ domain, shared, onDomainChange, onSharedChange, check }: {
  domain: string; shared: boolean; onDomainChange: (value: string) => void; onSharedChange: (value: boolean) => void; check: NodeDomainCheck
}) {
  const { t } = useTranslation()
  const zones = useQuery({ queryKey: ['domains'], queryFn: () => api.get<{ domains: { name: string }[] }>('/api/admin/domains') })
  const conflicts = check.data?.conflicts ?? []
  const names = conflicts.map(n => `${n.name} (#${n.id})`).join(', ')
  return <Stack gap="sm">
    <Autocomplete label={t('nodes.domain')} description={t('nodes.domainHint')} placeholder="jp1.example.com"
      data={(zones.data?.domains ?? []).map(d => domain.includes('.') && !domain.endsWith('.' + d.name) ? `${domain.split('.')[0]}.${d.name}` : d.name)}
      value={domain} onChange={onDomainChange} error={check.data?.valid === false ? t('nodes.domainInvalid') : undefined} />
    <Switch label={t('nodes.domainShared')} description={t('nodes.domainSharedHint')} checked={shared} onChange={e => onSharedChange(e.currentTarget.checked)} />
    {conflicts.length > 0 && <Alert color={shared ? 'blue' : 'orange'} title={t('nodes.domainUsed', { names })}>
      {t(shared ? 'nodes.domainSharedConflictHint' : check.data?.unchanged ? 'nodes.domainLegacyHint' : 'nodes.domainConflictHint')}
    </Alert>}
    {check.unavailable && <Text size="xs" c="dimmed">{t('nodes.domainCheckUnavailable')}</Text>}
  </Stack>
}
