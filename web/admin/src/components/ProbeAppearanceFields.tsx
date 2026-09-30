import { Badge, Box, Group, Progress, Select, SimpleGrid, Stack, Text } from '@mantine/core'
import { useTranslation } from 'react-i18next'

export function ProbeAppearanceFields({ value, onChange }: { value?: { preset: string; scheme: string }; onChange: (v: { preset: string; scheme: string }) => void }) {
 const { t } = useTranslation()
 const v = value ?? { preset: 'inherit', scheme: 'inherit' }
 const color = v.preset === 'terminal' ? 'teal' : v.preset === 'aurora' ? 'violet' : v.preset === 'paper' ? 'blue' : 'cyan'
 return <Stack gap="xs">
  <SimpleGrid cols={{ base: 1, sm: 2 }}>
   <Select label={t('probeAppearance.preset')} allowDeselect={false} value={v.preset} onChange={(preset) => preset && onChange({ ...v, preset })} data={['inherit', 'aurora', 'paper', 'terminal'].map((key) => ({ value: key, label: t(`probeAppearance.presets.${key}`) }))} />
   <Select label={t('probeAppearance.scheme')} allowDeselect={false} value={v.scheme} onChange={(scheme) => scheme && onChange({ ...v, scheme })} data={['inherit', 'auto', 'light', 'dark'].map((key) => ({ value: key, label: t(`probeAppearance.schemes.${key}`) }))} />
  </SimpleGrid>
  <Text size="xs" c="dimmed">{t('probeAppearance.hint')}</Text>
  <Box p="md" style={{ border: `1px ${v.preset === 'terminal' ? 'dashed' : 'solid'} var(--mantine-color-${color}-4)`, borderRadius: v.preset === 'terminal' ? 2 : 12, fontFamily: v.preset === 'terminal' ? 'monospace' : undefined, background: v.scheme === 'dark' ? '#1a1b1e' : v.scheme === 'light' ? '#ffffff' : 'var(--mantine-color-body)', color: v.scheme === 'dark' ? '#e9ecef' : v.scheme === 'light' ? '#212529' : 'var(--mantine-color-text)' }}>
   <Group justify="space-between"><Text fw={700}>{t('probeAppearance.preview')}</Text><Badge color={color}>CPU 24%</Badge></Group><Progress value={24} color={color} size="xs" mt="sm" />
  </Box>
 </Stack>
}
