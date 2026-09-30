import { Box, Group, SimpleGrid, Stack, Text, Tooltip } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import type { Availability } from '../lib/availability'

export function AvailabilityView({ data }: { data: Availability }) {
 const { t, i18n } = useTranslation()
 const percent = (v: number | null) => v == null ? '—' : `${v.toFixed(2)}%`
 const duration = (v: number) => new Intl.NumberFormat(i18n.language, { style: 'unit', unit: 'minute', maximumFractionDigits: 1 }).format(v / 60)
 const categories = ['online', 'offline', 'maintenance', 'unknown'] as const
 const colors = ['teal', 'red', 'blue', 'gray']
 return <Stack gap="sm">
  <Text size="xs" c="dimmed">{t('availability.hint')}</Text>
  <SimpleGrid cols={{ base: 2, sm: 4 }}>{[
   ['percent', percent(data.percent)], ['coverage', percent(data.coverage_percent)], ['offline', duration(data.offline_seconds)], ['maintenance', duration(data.maintenance_seconds)],
  ].map(([key, value]) => <Box key={key}><Text size="xs" c="dimmed">{t(`availability.${key}`)}</Text><Text fw={600}>{value}</Text></Box>)}</SimpleGrid>
  <Group gap={2} wrap="nowrap" style={{ overflowX: 'auto' }}>{data.buckets.map((b) => <Tooltip key={b.from} multiline label={<Stack gap={0}><Text size="xs">{new Date(b.from * 1000).toLocaleString()} — {new Date(b.to * 1000).toLocaleString()}</Text><Text size="xs">{t('availability.percent')}: {percent(b.percent)}</Text>{categories.map((k) => <Text size="xs" key={k}>{t(`availability.${k}`)}: {duration(b[`${k}_seconds`])}</Text>)}</Stack>}><Stack gap={0} style={{ flex: 1, minWidth: 4, height: 30, overflow: 'hidden', borderRadius: 2 }} aria-label={`${percent(b.percent)} / ${percent(b.coverage_percent)}`}>{categories.map((k, i) => <Box key={k} bg={`${colors[i]}.5`} style={{ height: `${100 * b[`${k}_seconds`] / (b.to - b.from)}%` }} />)}</Stack></Tooltip>)}</Group>
  <Group justify="space-between"><Text size="xs" c="dimmed">{new Date(data.from * 1000).toLocaleString()}</Text><Text size="xs" c="dimmed">{new Date(data.to * 1000).toLocaleString()}</Text></Group>
  <Group gap="md">{categories.map((k, i) => <Text key={k} size="xs" c={`${colors[i]}.6`}>{t(`availability.${k}`)}</Text>)}</Group>
  <Text size="xs" c="dimmed">{t('availability.unknown')}: {duration(data.unknown_seconds)}. {t('availability.observationHint')}</Text>
 </Stack>
}
