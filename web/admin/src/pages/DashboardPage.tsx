import { Alert, Anchor, Badge, Button, Card, Group, SimpleGrid, Skeleton, Stack, Text, Title } from '@mantine/core'
import { AreaChart } from '@mantine/charts'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { IconArrowRight, IconDevices, IconArrowsExchange, IconCoin, IconRefresh } from '@tabler/icons-react'
import { api, type Dashboard } from '../lib/api'
import { bytes, money } from '../lib/format'
import { useAuth } from '../lib/auth'
import { canVisit } from '../lib/navigation'
import { PageHeader } from '../components/PageHeader'
import { SelfCheckCard } from '../components/SelfCheckCard'
import { Stat } from '../components/Stat'
import classes from './DashboardPage.module.css'

function TaskRow({ label, count, to, warning = false }: { label: string; count: number | undefined; to: string; warning?: boolean }) {
  return <Anchor component={Link} to={to} className={classes.task} underline="never">
    <Text component="span" size="sm" fw={500}>{label}</Text>
    <Group gap="sm" wrap="nowrap"><Badge color={warning && count ? 'orange' : 'gray'} variant="light">{count ?? '—'}</Badge><IconArrowRight size={15} aria-hidden /></Group>
  </Anchor>
}

export default function DashboardPage() {
  const { t, i18n } = useTranslation()
  const { me } = useAuth()
  const q = useQuery({ queryKey: ['dashboard'], queryFn: () => api.get<Dashboard>('/api/admin/dashboard'), refetchInterval: 30_000 })
  const s = q.data?.stats
  const attention = q.data?.attention
  const showNodes = canVisit('/nodes', me?.role ?? '')
  const series = (q.data?.traffic ?? []).map(d => ({ day: new Date(d.day * 1000).toLocaleDateString(i18n.language, { month: 'short', day: 'numeric', timeZone: 'UTC' }), GiB: +((d.up + d.down) / 2 ** 30).toFixed(2) }))
  return <Stack gap="lg">
    <PageHeader title={t('dashboard.title')} subtitle={t('dashboard.workspaceHint')} actions={<Button variant="default" size="xs" leftSection={<IconRefresh size={15} />} loading={q.isFetching} onClick={() => q.refetch()}>{t('common.refresh')}</Button>} />
    {q.isError && <Alert color="red">{t('dashboard.loadFailed')}{q.data && ` ${t('dashboard.stale')}`}</Alert>}
    {!s ? !q.isError && <Skeleton h={240} /> : <>
      <SimpleGrid cols={{ base: 1, md: showNodes ? 2 : 1 }}>
        {showNodes && <Card className={classes.tasks}>
          <Group justify="space-between" mb="sm"><Title order={4}>{t('dashboard.operations')}</Title><Text size="xs" c="dimmed">{t('dashboard.nodesOnline')}: {s.nodes_online} / {s.nodes}</Text></Group>
          <TaskRow label={t('dashboard.offlineNodes')} count={attention?.nodes?.offline} to="/nodes?status=offline" warning />
          <TaskRow label={t('dashboard.lastCheckFailed')} count={attention?.nodes?.doctor_fail} to="/nodes?status=doctor" warning />
          <TaskRow label={t('dashboard.openIncidents')} count={attention?.open_incidents} to="/monitoring?tab=alerts" warning />
          <Text size="xs" c="dimmed" mt="sm">{t('dashboard.operationsHint')}</Text>
        </Card>}
        <Card className={classes.tasks}>
          <Title order={4} mb="sm">{t('dashboard.tasks')}</Title>
          <TaskRow label={t('dashboard.ticketsToReply')} count={q.data?.open_tickets} to="/tickets" warning />
          <TaskRow label={t('dashboard.ordersToPay')} count={s.orders_pending} to="/orders?status=pending" />
          {showNodes && <TaskRow label={t('nodes.unpaired')} count={attention?.nodes?.unpaired} to="/nodes?status=unpaired" />}
          {me?.role === 'admin' && <TaskRow label={t('dashboard.assetsDue')} count={attention?.assets_due} to="/infrastructure?filter=due" warning />}
        </Card>
      </SimpleGrid>
      <section aria-labelledby="business-summary"><Title id="business-summary" order={4} mb="sm">{t('dashboard.businessSummary')}</Title>
        <SimpleGrid cols={{ base: 1, sm: 2, lg: 4 }}>
          <Stat label={t('dashboard.activeSubs')} value={s.active_subs} hint={t('dashboard.ofUsers', { count: s.users })} icon={<IconArrowsExchange size={18} opacity={0.6} />} />
          <Stat label={t('dashboard.onlineDevices')} value={s.online_devices} icon={<IconDevices size={18} opacity={0.6} />} />
          <Stat label={t('dashboard.trafficToday')} value={bytes(s.traffic_today_bytes)} icon={<IconArrowsExchange size={18} opacity={0.6} />} />
          <Stat label={t('dashboard.revenueToday')} value={money(s.revenue_today_cents)} hint={t('dashboard.revenueMonth', { amount: money(s.revenue_month_cents) })} icon={<IconCoin size={18} opacity={0.6} />} />
        </SimpleGrid>
      </section>
      <Card>
        <Text fw={600}>{t('dashboard.trafficChart')}</Text>
        <Text size="xs" c="dimmed" mb="md">{t('dashboard.utcHint')}</Text>
        <AreaChart h={240} data={series} dataKey="day" series={[{ name: 'GiB', color: 'brand.5' }]} curveType="linear" withDots={false} gridAxis="x" />
      </Card>
    </>}
    {me?.role === 'admin' && <section aria-labelledby="system-checks"><Title id="system-checks" order={4} mb="sm">{t('dashboard.systemChecks')}</Title><SelfCheckCard /></section>}
  </Stack>
}
