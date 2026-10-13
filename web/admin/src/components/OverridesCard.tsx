import { Button, Card, Group, JsonInput, SimpleGrid, Text, Title } from '@mantine/core'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from '../lib/notify'

const cores = ['xray', 'singbox', 'hysteria', 'mita'] as const
const labels: Record<string, string> = { xray: 'Xray', singbox: 'sing-box', hysteria: 'Hysteria', mita: 'mieru (mita)' }

// Per-core JSON objects deep-merged into the rendered config: the escape
// hatch for options the panel does not model. `load`/`save` differ per host
// (standalone API vs a Captain node endpoint).
export function OverridesCard({ queryKey, load, save, readOnly }: { queryKey: unknown[]; load: () => Promise<Record<string, string>>; save: (v: Record<string, string>) => Promise<unknown>; readOnly?: boolean }) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const q = useQuery({ queryKey, queryFn: load })
  // Local edits overlay the loaded values; a fresh load replaces untouched fields.
  const [edits, setEdits] = useState<Record<string, string>>({})
  const v = { ...q.data, ...edits }
  const setV = (f: (cur: Record<string, string>) => Record<string, string>) => setEdits((cur) => f({ ...q.data, ...cur }))
  const mut = useMutation({ mutationFn: () => save(v), onSuccess: () => { toast.ok(t('common.saved')); qc.invalidateQueries({ queryKey }) }, onError: toast.err })
  return (
    <Card>
      <Title order={5} mb={4}>{t('overrides.title')}</Title>
      <Text size="xs" c="dimmed" mb="sm">{t('overrides.hint')}</Text>
      <SimpleGrid cols={{ base: 1, md: 2 }} spacing="sm">
        {cores.map((c) => (
          <JsonInput key={c} label={labels[c]} placeholder={'{ }'} autosize minRows={3} maxRows={12} formatOnBlur validationError={t('overrides.invalid')} disabled={readOnly} value={v[c] ?? ''} onChange={(x) => setV((cur) => ({ ...cur, [c]: x }))} />
        ))}
      </SimpleGrid>
      {!readOnly && <Group justify="flex-end" mt="sm"><Button size="xs" loading={mut.isPending} onClick={() => mut.mutate()}>{t('common.save')}</Button></Group>}
    </Card>
  )
}
