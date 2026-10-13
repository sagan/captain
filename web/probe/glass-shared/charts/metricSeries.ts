// Pure parts of the glass MetricSeriesChartCard, shared by both builds.
import { formatBytes } from '@shared/utils/helper'
import { escapeHTML } from '@shared/utils/html'
import type { MetricChartSeriesData } from '@shared/charts/load'

type MetricValueKind = MetricChartSeriesData['kind']

export function formatMetricValue(value: number | null | undefined, kind: MetricValueKind): string {
  if (value === null || value === undefined || !Number.isFinite(value))
    return '-'

  switch (kind) {
    case 'bytes':
      return formatBytes(value)
    case 'bytesPerSecond':
      return `${formatBytes(value)}/s`
    case 'milliseconds':
      return `${value.toFixed(value >= 100 ? 0 : 1)} ms`
    case 'percent':
      return `${value.toFixed(1)}%`
    case 'temperature':
      return `${value.toFixed(1)} °C`
    default:
      return Math.round(value).toLocaleString('zh-CN')
  }
}

export function metricSeriesLatestText(p: { latest: string, series: MetricChartSeriesData[] }): string {
  if (p.latest)
    return p.latest

  for (const series of p.series) {
    const point = [...series.data].reverse().find(item => item[1] !== null && Number.isFinite(item[1]))
    if (point)
      return `${series.name} ${formatMetricValue(point[1], series.kind)}`
  }
  return '-'
}

export function buildMetricSeriesChartOption(p: { series: MetricChartSeriesData[], percentScale: boolean }) {
  const primaryKind: MetricValueKind = p.series[0]?.kind ?? 'count'
  return {
  animation: false,
  color: p.series.map(item => item.color),
  tooltip: {
    trigger: 'axis',
    confine: true,
    backgroundColor: 'var(--color-popover)',
    borderColor: 'var(--color-border)',
    borderWidth: 1,
    textStyle: { color: 'var(--color-popover-foreground)', fontSize: 12 },
    formatter: (params: unknown) => {
      const items = params as Array<{ axisValueLabel?: string, color: string, data: [string, number | null], seriesIndex: number, seriesName: string }>
      if (!items.length)
        return ''
      const rows = items.map((item) => {
        const source = p.series[item.seriesIndex]
        const kind = source?.kind ?? primaryKind
        return `<div style="display:flex;align-items:center;gap:8px"><span style="width:8px;height:8px;border-radius:2px;background:${item.color};flex:none"></span><span>${escapeHTML(item.seriesName)}</span><strong style="margin-left:auto;padding-left:12px">${formatMetricValue(item.data?.[1], kind)}</strong></div>`
      }).join('')
      return `<div style="margin-bottom:6px;color:var(--color-muted-foreground)">${escapeHTML(items[0]?.axisValueLabel ?? '')}</div><div style="display:flex;flex-direction:column;gap:4px">${rows}</div>`
    },
  },
  legend: {
    type: 'scroll',
    bottom: 2,
    itemWidth: 10,
    itemHeight: 8,
    textStyle: { color: 'var(--color-muted-foreground)', fontSize: 10 },
  },
  grid: { top: 20, right: 18, bottom: 48, left: 58 },
  xAxis: {
    type: 'time',
    axisLine: { lineStyle: { color: 'var(--color-border)' } },
    axisTick: { show: false },
    axisLabel: { color: 'var(--color-muted-foreground)', fontSize: 10, hideOverlap: true },
    splitLine: { show: false },
  },
  yAxis: {
    type: 'value',
    min: p.percentScale ? 0 : undefined,
    max: p.percentScale ? 100 : undefined,
    axisLine: { show: false },
    axisTick: { show: false },
    axisLabel: {
      color: 'var(--color-muted-foreground)',
      fontSize: 10,
      formatter: (value: number) => formatMetricValue(value, primaryKind),
    },
    splitLine: { lineStyle: { color: 'var(--color-border)', opacity: 0.45 } },
  },
  series: p.series.map(item => ({
    name: item.name,
    type: 'line',
    data: item.data,
    connectNulls: false,
    showSymbol: false,
    smooth: false,
    lineStyle: { width: 1.6, type: item.dashed ? 'dashed' : 'solid', color: item.color },
  })),
}
}
