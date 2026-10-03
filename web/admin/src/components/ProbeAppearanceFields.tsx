import { Badge, Box, Group, Progress, Select, SimpleGrid, Stack, Text } from '@mantine/core'
import { useTranslation } from 'react-i18next'

export function ProbeAppearanceFields({ value, onChange }: { value?: { preset: string; scheme: string }; onChange: (v: { preset: string; scheme: string }) => void }) {
 const { t } = useTranslation()
 const v = value ?? { preset: 'inherit', scheme: 'inherit' }
 const color = v.preset === 'terminal' ? 'teal' : v.preset === 'aurora' ? 'violet' : v.preset === 'paper' ? 'blue' : 'cyan'
 const glass = v.preset === 'glass'
 const background = v.scheme === 'dark' ? 'var(--mantine-color-dark-7)' : v.scheme === 'light' ? 'var(--mantine-color-white)' : 'var(--mantine-color-body)'
 return <Stack gap="xs">
  <SimpleGrid cols={{ base: 1, sm: 2 }}>
   <Select label={t('probeAppearance.preset')} allowDeselect={false} value={v.preset} onChange={(preset) => preset && onChange({ ...v, preset })} data={['inherit', 'aurora', 'paper', 'terminal', 'glass'].map((key) => ({ value: key, label: t(`probeAppearance.presets.${key}`) }))} />
   <Select label={t('probeAppearance.scheme')} allowDeselect={false} value={v.scheme} onChange={(scheme) => scheme && onChange({ ...v, scheme })} data={['inherit', 'auto', 'light', 'dark'].map((key) => ({ value: key, label: t(`probeAppearance.schemes.${key}`) }))} />
  </SimpleGrid>
  <Text size="xs" c="dimmed">{t('probeAppearance.hint')}</Text>
  {glass && <Text size="xs" c="dimmed">{t('probeAppearance.glassHint')}</Text>}
  <Box p="md" style={{ border: `1px ${v.preset === 'terminal' ? 'dashed' : 'solid'} var(--mantine-color-${color}-4)`, borderRadius: v.preset === 'terminal' ? 2 : 16, fontFamily: v.preset === 'terminal' ? 'monospace' : undefined, background: glass ? `linear-gradient(120deg, color-mix(in srgb, var(--mantine-color-cyan-3) 35%, transparent), color-mix(in srgb, var(--mantine-color-violet-3) 30%, transparent)), ${background}` : background, color: v.scheme === 'dark' ? 'var(--mantine-color-gray-1)' : v.scheme === 'light' ? 'var(--mantine-color-gray-9)' : 'var(--mantine-color-text)' }}>
   <Box p={glass ? 'md' : 0} style={glass ? { background: `color-mix(in srgb, ${background} 65%, transparent)`, border: '1px solid color-mix(in srgb, var(--mantine-color-cyan-2) 35%, transparent)', borderRadius: 14, boxShadow: '0 8px 24px color-mix(in srgb, var(--mantine-color-cyan-9) 8%, transparent)' } : undefined}>
    <Group justify="space-between"><Text fw={700}>{t('probeAppearance.preview')}</Text><Badge color={color}>CPU 24%</Badge></Group><Progress value={24} color={color} size="xs" mt="sm" />
   </Box>
  </Box>
 </Stack>
}
