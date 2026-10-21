import { NumberInput, Select, SimpleGrid, Stack, Switch, Text } from '@mantine/core'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

export interface NodeBandwidth { inbound_mbps: number; outbound_mbps: number; public: boolean }
export const emptyBandwidth: NodeBandwidth = { inbound_mbps: 0, outbound_mbps: 0, public: false }

export function NodeBandwidthFields({ value, onChange }: { value: NodeBandwidth; onChange: (value: NodeBandwidth) => void }) {
  const { t } = useTranslation()
  const [unit, setUnit] = useState('Mbps')
  const [expanded, setExpanded] = useState(false)
  const separate = expanded || value.inbound_mbps !== value.outbound_mbps
  const factor = unit === 'Gbps' ? 1000 : 1
  const rate = (v: number | string) => Math.round(Number(v || 0) * factor * 1e6) / 1e6
  return <Stack gap="xs">
    <Text size="sm" fw={500}>{t('nodeBandwidth.title')}</Text>
    <Text size="xs" c="dimmed">{t('nodeBandwidth.hint')}</Text>
    <Switch label={t('nodeBandwidth.separate')} checked={separate} onChange={e => {
      setExpanded(e.currentTarget.checked)
      if (!e.currentTarget.checked) onChange({ ...value, outbound_mbps: value.inbound_mbps })
    }} />
    <SimpleGrid cols={{ base: 1, sm: separate ? 3 : 2 }}>
      <NumberInput label={t(separate ? 'nodeBandwidth.inbound' : 'nodeBandwidth.symmetric')} placeholder={t('nodeBandwidth.unknown')} min={0} max={1000000 / factor} decimalScale={6} allowNegative={false} value={value.inbound_mbps / factor || ''} onChange={v => onChange({ ...value, inbound_mbps: rate(v), ...(!separate ? { outbound_mbps: rate(v) } : {}) })} />
      {separate && <NumberInput label={t('nodeBandwidth.outbound')} placeholder={t('nodeBandwidth.unknown')} min={0} max={1000000 / factor} decimalScale={6} allowNegative={false} value={value.outbound_mbps / factor || ''} onChange={v => onChange({ ...value, outbound_mbps: rate(v) })} />}
      <Select label={t('nodeBandwidth.unit')} data={['Mbps', 'Gbps']} value={unit} allowDeselect={false} onChange={v => setUnit(v ?? 'Mbps')} />
    </SimpleGrid>
    <Switch label={t('nodeBandwidth.public')} checked={value.public} onChange={e => onChange({ ...value, public: e.currentTarget.checked })} />
  </Stack>
}
