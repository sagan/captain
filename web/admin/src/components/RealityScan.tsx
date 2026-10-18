import { Badge, Button, Group, Paper, Stack, Text, Tooltip } from '@mantine/core'
import { IconCheck, IconX } from '@tabler/icons-react'
import { useLayoutEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from '../lib/notify'

// One probed REALITY target as the node reports it.
export interface RealityResult {
  host: string; port: number; ip?: string; feasible: boolean; reason?: string
  tls13: boolean; h2: boolean; x25519: boolean; cert_valid: boolean; cdn?: string
  cert_subject?: string; cert_issuer?: string; server_names?: string[]; latency_ms: number
  tls_record_bytes?: number; tls_record_limit?: number; tls_record_ok?: boolean | null; reason_code?: string
}

// RealityScan runs the node-side probe over the built-in pool ("auto":
// picks the fastest feasible target) or over the host currently typed in,
// and lets the operator pick a row. `scan` is the transport: a direct call
// on the standalone panel, a node job on Captain.
export function RealityScan({ current, scan, onPick, disabled = false }: { disabled?: boolean; current: string; scan: (hosts: string[], signal?: AbortSignal) => Promise<RealityResult[]>; onPick: (host: string) => void }) {
  const { t } = useTranslation()
  const pending = useRef<AbortController | null>(null)
  const [request, setRequest] = useState<{ mode: 'auto' | 'one'; current: string; controller: AbortController } | null>(null)
  const [result, setResult] = useState<{ rows: RealityResult[]; current: string; picked?: string } | null>(null)
  // A result belongs to this mounted node and the input that started it.
  // Closing/reordering a wizard or typing another target cannot apply it.
  useLayoutEffect(() => () => { pending.current?.abort(); pending.current = null }, [current])
  const busy = request && request.current === current && !request.controller.signal.aborted ? request.mode : null
  const rows = result && (result.current === current || result.picked === current) ? result.rows : null
  const selectable = (r: RealityResult) => r.feasible && r.tls_record_ok !== false
  const pick = (host: string) => { setResult(r => r ? { ...r, picked: host } : r); onPick(host) }
  const run = async (mode: 'auto' | 'one') => {
    if (disabled) return
    pending.current?.abort()
    const controller = new AbortController()
    pending.current = controller
    setRequest({ mode, current, controller })
    try {
      const out = await scan(mode === 'one' ? [current.trim()] : [], controller.signal)
      if (pending.current !== controller || controller.signal.aborted) return
      const best = mode === 'auto' ? out.find(selectable) : undefined
      setResult({ rows: out, current, picked: best?.host })
      if (mode === 'auto') {
        if (best) { onPick(best.host); toast.ok(t('inbounds.realityPicked', { host: best.host, ms: best.latency_ms })) } else toast.err(new Error(t('inbounds.realityNone')))
      }
    } catch (e) {
      if (pending.current === controller && !controller.signal.aborted) toast.err(e)
    } finally {
      if (pending.current === controller) { pending.current = null; setRequest(null) }
    }
  }
  const mark = (v: boolean) => (v ? <IconCheck size={14} color="var(--mantine-color-teal-6)" /> : <IconX size={14} color="var(--mantine-color-red-6)" />)
  return (
    <Stack gap="xs">
      <Group gap="xs">
        <Button size="xs" variant="light" loading={busy === 'auto'} disabled={disabled || busy !== null} onClick={() => run('auto')}>{t('inbounds.realityAuto')}</Button>
        <Button size="xs" variant="default" loading={busy === 'one'} disabled={disabled || busy !== null || !current.trim()} onClick={() => run('one')}>{t('inbounds.realityProbe')}</Button>
        {busy && <Text size="xs" c="dimmed">{t('inbounds.realityScanning')}</Text>}
      </Group>
      <Text size="xs" c="dimmed">{t('inbounds.realityScanHint')}</Text>
      {rows && (
        <Stack gap="xs" style={{ minWidth: 0 }}>
          {rows.map(r => (
            <Paper key={r.host + r.port} withBorder p="sm" style={{ minWidth: 0 }}>
              <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) auto', gap: 'var(--mantine-spacing-sm)', alignItems: 'start' }}>
                <Stack gap={2} style={{ minWidth: 0 }}>
                  <Text size="sm" fw={600} style={{ overflowWrap: 'anywhere' }}>{r.host}{r.port !== 443 ? `:${r.port}` : ''}</Text>
                  {r.feasible && r.cert_issuer && <Text size="xs" c="dimmed" style={{ overflowWrap: 'anywhere' }}>{r.cert_issuer}</Text>}
                </Stack>
                {selectable(r) ? <Button size="compact-xs" variant="light" onClick={() => pick(r.host)}>{t('inbounds.realityUse')}</Button> : <Text size="xs" c="dimmed">{t('inbounds.realityBlocked')}</Text>}
              </div>
              <Group gap="sm" mt="xs" wrap="wrap">
                <Text size="xs" c="dimmed">{t('inbounds.realityLatency')}: {r.latency_ms >= 0 ? `${r.latency_ms} ms` : '—'}</Text>
                {([['TLS 1.3', r.tls13], ['h2', r.h2], ['X25519', r.x25519], [t('inbounds.realityCert'), r.cert_valid]] as const).map(([label, passed]) => (
                  <Group key={label} gap={4} wrap="nowrap"><Text size="xs">{label}</Text>{mark(passed)}</Group>
                ))}
                <Group gap={4} wrap="nowrap">
                  <Text size="xs">{t('inbounds.realityRecord')}</Text>
                  {typeof r.tls_record_ok === 'boolean' ? <Tooltip label={t('inbounds.realityRecordHint', { limit: r.tls_record_limit })}><Badge miw="max-content" color={r.tls_record_ok ? 'teal' : 'red'} size="xs">{r.tls_record_bytes} B</Badge></Tooltip> : <Tooltip label={t('inbounds.realityRecordUnknownHint')}><Badge miw="max-content" color="yellow" size="xs">{t('inbounds.realityRecordUnknown')}</Badge></Tooltip>}
                </Group>
                <Group gap={4} wrap="nowrap">
                  <Text size="xs">CDN</Text>
                  {r.cdn ? <Tooltip label={t('inbounds.realityCdnHint')}><Badge color="red" size="xs">{r.cdn}</Badge></Tooltip> : <Text size="xs" c="dimmed">{t('inbounds.realityNoCdn')}</Text>}
                </Group>
              </Group>
              {r.reason && <Text size="xs" mt={4} c={r.feasible ? 'dimmed' : 'red'} style={{ overflowWrap: 'anywhere' }}>{r.reason_code === 'tls_record_too_large' ? t('inbounds.realityRecordTooLarge', { bytes: r.tls_record_bytes, limit: r.tls_record_limit }) : r.reason}</Text>}
            </Paper>
          ))}
          {rows.length === 0 && <Text size="xs" c="dimmed">{t('inbounds.realityNone')}</Text>}
        </Stack>
      )}
    </Stack>
  )
}
