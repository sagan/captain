import { useMemo } from 'react'
import type { MetricChartSeriesData } from '@shared/charts/load'
import { buildMetricSeriesChartOption, metricSeriesLatestText } from '@shared/charts/metricSeries'
import '@shared/utils/echarts'
import EChart from '@/components/EChart'
import MetricChartHeader from '@/components/MetricChartHeader'
import { CardX } from '@/components/ui/card-x'

type MetricChartTone = 'rose' | 'amber' | 'emerald' | 'cyan' | 'sky' | 'violet' | 'orange' | 'slate'

export default function MetricSeriesChartCard({ title, icon, tone = 'slate', series, order = 99, subtitle = '', latest = '', percentScale = false }: {
  title: string
  icon: string
  tone?: MetricChartTone
  series: MetricChartSeriesData[]
  order?: number
  subtitle?: string
  latest?: string
  percentScale?: boolean
}) {
  const latestText = useMemo(() => metricSeriesLatestText({ latest, series }), [latest, series])
  const chartOption = useMemo(() => buildMetricSeriesChartOption({ series, percentScale }), [series, percentScale])
  return (
    <CardX
      size="small"
      className="bg-background/50 border-none hover:bg-background transition-all rounded-md"
      style={{ order }}
      header={(
        <MetricChartHeader title={title} icon={icon} tone={tone} subtitle={subtitle}>
          {latestText}
        </MetricChartHeader>
      )}
    >
      <div className="h-48">
        <EChart option={chartOption} autoresize />
      </div>
    </CardX>
  )
}
