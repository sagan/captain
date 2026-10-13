import { t } from '@shared/i18n'
import { AppDialog } from '@/components/ui/app-dialog'
import { asyncComponent } from '@/lib/asyncComponent'

const PingChart = asyncComponent(() => import('@/components/PingChart'))

export default function PingMonitorDialog({ open, uuid, nodeName, onOpenChange }: { open: boolean, uuid: string, nodeName: string, onOpenChange?: (open: boolean) => void }) {
  return (
    <AppDialog
      open={open}
      title={`${nodeName} ${t('延迟')} / ${t('丢包')}`}
      description={t('探测任务延迟、丢包率与波动统计')}
      contentClass="max-w-6xl"
      onOpenChange={onOpenChange}
    >
      {open && uuid ? <PingChart uuid={uuid} /> : null}
    </AppDialog>
  )
}
