// Pure parts of the glass PingChart (data loading, record merging, ECharts
// option), shared by the Vue and React components so both feed ECharts the
// same options. Bodies are moved from PingChart.vue with reactive reads turned
// into parameters.
import dayjs from 'dayjs'
import { escapeHTML } from '@shared/utils/html'
import type { MetricSeries, PingMetricTaskStats, PingRecord, PingTaskInfo } from '@shared/utils/rpc'
import { PING_RECORD_MAX_COUNT } from '@shared/constants/load'
import { loadPingMetricStats, loadPublicPingTasks, queryMetrics } from '@shared/services/metrics.service'
import { ACCESSIBLE_LINE_TYPES } from '@shared/utils/chartPalette'
import { isPingMetric, normalizeMetricSeriesList, orderPingTasksByBackend, PING_LATENCY_METRIC, pingTaskId, pingTaskName } from '@shared/utils/metricSeries'

export interface CustomRange {
  start: dayjs.Dayjs
  end: dayjs.Dayjs
  hours: number
}

export const PING_CHART_MARGIN = { top: 30, right: 24, bottom: 52, left: 56 }

export function pingChartThemeColors(isDark: boolean) {
  return {
  text: isDark ? 'rgba(255, 255, 255, 0.85)' : 'rgba(0, 0, 0, 0.85)',
  textSecondary: isDark ? 'rgba(255, 255, 255, 0.55)' : 'rgba(0, 0, 0, 0.55)',
  textTertiary: isDark ? 'rgba(255, 255, 255, 0.35)' : 'rgba(0, 0, 0, 0.35)',
  borderColor: isDark ? 'rgba(255, 255, 255, 0.1)' : 'rgba(0, 0, 0, 0.06)',
  splitLineColor: isDark ? 'rgba(255, 255, 255, 0.06)' : 'rgba(0, 0, 0, 0.06)',
  tooltipBg: isDark ? 'rgba(40, 40, 40, 0.95)' : 'rgba(255, 255, 255, 0.8)',
  tooltipShadow: isDark ? 'rgba(0, 0, 0, 0.4)' : 'rgba(0, 0, 0, 0.06)',
  crosshairColor: isDark ? 'rgba(255, 255, 255, 0.15)' : 'rgba(0, 0, 0, 0.1)',
}
}

export function pingBaseTooltipConfig(chartThemeColors: ReturnType<typeof pingChartThemeColors>) {
  return {
  trigger: 'axis' as const,
  confine: false,
  backgroundColor: chartThemeColors.tooltipBg,
  borderColor: 'transparent',
  borderWidth: 0,
  borderRadius: 6,
  textStyle: {
    color: chartThemeColors.text,
    fontSize: 12,
    lineHeight: 20,
  },
  extraCssText: `backdrop-filter: blur(5px);z-index:9;box-shadow:0 0 0 1px ${chartThemeColors.tooltipShadow}, 0 0 16px ${chartThemeColors.tooltipShadow}`,
  axisPointer: {
    type: 'cross' as const,
    crossStyle: {
      color: chartThemeColors.textTertiary,
    },
    lineStyle: {
      color: chartThemeColors.crosshairColor,
      width: 1,
      type: 'dashed' as const,
    },
    shadowStyle: {
      color: chartThemeColors.crosshairColor,
    },
  },
}
}

export function normalizeMetricTaskId(taskId: string): number {
  if (!taskId.trim())
    return Number.NaN

  const numericTaskId = Number(taskId)
  if (Number.isFinite(numericTaskId))
    return numericTaskId

  let hash = 0
  for (let index = 0; index < taskId.length; index++)
    hash = (hash * 31 + taskId.charCodeAt(index)) | 0
  return Math.abs(hash)
}


export function normalizeMetricTask(stat: PingMetricTaskStats): PingTaskInfo {
  return {
    id: normalizeMetricTaskId(stat.task_id),
    name: stat.name?.trim() || pingTaskName(stat) || `Task ${stat.task_id}`,
    interval: stat.interval ?? 0,
    loss: stat.loss,
    min: stat.min,
    max: stat.max,
    avg: stat.avg,
    latest: stat.latest,
    p50: stat.p50,
    p99: stat.p99,
    p99_p50_ratio: stat.p99_p50_ratio,
    stddev: stat.stddev,
    total: stat.total,
    valid: stat.valid,
    loss_approximate: stat.loss_approximate,
    type: stat.type,
  }
}


export function buildMetricRecords(seriesList: MetricSeries[]): PingRecord[] {
  const records: PingRecord[] = []
  const normalizedSeriesList = normalizeMetricSeriesList(seriesList).filter(isPingMetric)

  for (const series of normalizedSeriesList) {
    const taskId = normalizeMetricTaskId(pingTaskId(series))
    if (!Number.isFinite(taskId))
      continue

    for (const point of series.points) {

      records.push({
        client: series.entity_id,
        task_id: taskId,
        time: point.time,
        value: point.value ?? Number.NaN,
      })
    }
  }

  return records.sort((a, b) => dayjs(a.time).valueOf() - dayjs(b.time).valueOf())
}


export async function loadMetricPingPayload(nodeUuid: string, range: CustomRange | null, isCustomRange: boolean, selectedHours: number): Promise<{ records: PingRecord[], tasks: PingTaskInfo[] } | null> {
  const metricRangeParams = isCustomRange && range
    ? { start: range.start.toDate().toISOString(), end: range.end.toDate().toISOString() }
    : { hours: selectedHours }

  const [statsResult, metricsResult, backendTasksResult] = await Promise.allSettled([
    loadPingMetricStats({ entity_id: nodeUuid, ...metricRangeParams, max_points: PING_RECORD_MAX_COUNT }),
    queryMetrics({
      metric_keys: [PING_LATENCY_METRIC],
      entity_id: nodeUuid,
      ...metricRangeParams,
      downsample: true,
      fill_empty: true,
      max_points: PING_RECORD_MAX_COUNT,
      aggregation: 'avg',
    }),
    loadPublicPingTasks(),
  ])

  const metricStats = statsResult.status === 'fulfilled'
    ? (statsResult.value.stats ?? []).filter(stat => stat.entity_id === nodeUuid)
    : []
  const metricRecords = metricsResult.status === 'fulfilled'
    ? buildMetricRecords(metricsResult.value.series)
    : []

  const metricTaskIds = new Set(metricRecords.map(record => record.task_id))
  const exactStatTaskIds = new Set(
    metricStats
      .filter(stat => stat.total > 0 && !stat.loss_approximate && Number.isFinite(stat.loss))
      .map(stat => normalizeMetricTaskId(stat.task_id)),
  )
  if (!metricRecords.length || [...metricTaskIds].some(taskId => !exactStatTaskIds.has(taskId)))
    return null

  const taskMap = new Map<number, PingTaskInfo>()
  for (const stat of metricStats) {
    const task = normalizeMetricTask(stat)
    taskMap.set(task.id, task)
  }

  for (const series of normalizeMetricSeriesList(
    metricsResult.status === 'fulfilled' ? metricsResult.value.series : [],
  ).filter(isPingMetric)) {
    const taskId = normalizeMetricTaskId(pingTaskId(series))
    if (!taskId || taskMap.has(taskId))
      continue

    taskMap.set(taskId, {
      id: taskId,
      name: pingTaskName(series) || `Task ${taskId}`,
      interval: series.interval_seconds ?? 0,
      loss: 0,
    })
  }

  return {
    records: metricRecords,
    tasks: orderPingTasksByBackend(
      [...taskMap.values()],
      backendTasksResult.status === 'fulfilled' ? backendTasksResult.value : [],
    ),
  }
}


export function mergePingRecords(remoteData: PingRecord[], tasks: PingTaskInfo[], range: CustomRange | null, isCustomRange: boolean, selectedHours: number): Array<Record<string, unknown>> {
  const data = remoteData
  if (!data.length)
    return []

  const taskList = tasks

  const taskIntervals = taskList
    .map(t => t.interval)
    .filter((v): v is number => typeof v === 'number' && v > 0)

  const fallbackIntervalSec = taskIntervals.length ? Math.min(...taskIntervals) : 60
  const toleranceMs = Math.min(
    6000,
    Math.max(800, Math.floor(fallbackIntervalSec * 1000 * 0.25)),
  )

  const grouped: Map<number, Record<string, unknown>> = new Map()
  const anchors: number[] = []

  for (const rec of data) {
    const ts = dayjs(rec.time).valueOf()
    let anchor: number | null = null

    for (let index = anchors.length - 1; index >= 0; index--) {
      const a = anchors[index]
      if (a === undefined || ts - a > toleranceMs)
        break
      if (Math.abs(a - ts) <= toleranceMs) {
        anchor = a
        break
      }
    }

    const useTs = anchor ?? ts
    if (!grouped.has(useTs)) {
      grouped.set(useTs, { time: dayjs(useTs).toISOString() })
      if (anchor === null) {
        anchors.push(useTs)
      }
    }

    const group = grouped.get(useTs)!
    group[rec.task_id] = Number.isFinite(rec.value) && rec.value >= 0 ? rec.value : null
  }

  const merged = Array.from(grouped.values()).sort(
    (a, b) => dayjs(a.time as string).valueOf() - dayjs(b.time as string).valueOf(),
  )

  if (isCustomRange && range) {
    const fromTs = range.start.valueOf()
    const toTs = range.end.valueOf()
    return merged.filter((item) => {
      const timestamp = dayjs(item.time as string).valueOf()
      return timestamp >= fromTs && timestamp <= toTs
    })
  }

  const hours = selectedHours
  const lastItem = merged.at(-1)
  const lastTs = lastItem ? dayjs(lastItem.time as string).valueOf() : dayjs().valueOf()
  const fromTs = lastTs - hours * 3600_000

  let startIdx = 0
  for (let i = 0; i < merged.length; i++) {
    const item = merged[i]
    if (!item)
      continue
    const ts = dayjs(item.time as string).valueOf()
    if (ts >= fromTs) {
      startIdx = Math.max(0, i - 1)
      break
    }
  }

  return merged.slice(startIdx)
}

export function buildPingViews(maxPingRecordPreserveTime: number, presetViews: Array<{ label: string, hours: number }>, customViewLabel: string): Array<{ label: string, hours?: number }> {
  const views: { label: string, hours?: number }[] = []
  const maxHours = maxPingRecordPreserveTime

  for (const v of presetViews) {
    if (maxHours >= v.hours) {
      views.push(v)
    }
  }

  const maxPreset = presetViews.at(-1)
  if (maxPreset && maxHours > maxPreset.hours) {
    const label = maxHours % 24 === 0
      ? `${Math.floor(maxHours / 24)} 天`
      : `${maxHours} 小时`
    views.push({ label, hours: maxHours })
  }
  else if (maxHours > 1 && !presetViews.some(v => v.hours === maxHours)) {
    const label = maxHours % 24 === 0
      ? `${Math.floor(maxHours / 24)} 天`
      : `${maxHours} 小时`
    views.push({ label, hours: maxHours })
  }

  views.push({ label: customViewLabel })
  return views
}

export function buildLatestValues(tasks: PingTaskInfo[], remoteData: PingRecord[], chartColors: string[]) {
  if (!tasks.length)
    return []

  const latestMap = new Map<number, number | null>()
  for (const task of tasks) {
    for (let i = remoteData.length - 1; i >= 0; i--) {
      const rec = remoteData[i]
      if (rec && rec.task_id === task.id && rec.value >= 0) {
        latestMap.set(task.id, rec.value)
        break
      }
    }
  }

  return tasks.map((task, idx) => {
    const safeIdx = Math.max(0, idx % chartColors.length)
    return {
      ...task,
      latestValue: latestMap.get(task.id) ?? null,
      color: chartColors[safeIdx]!,
    }
  })
}

export function formatPingTime(time: string, showDate: boolean): string {
  const date = dayjs(time)
  if (showDate) {
    return date.format('M/D HH:mm')
  }
  return date.format('HH:mm')
}

export function formatPingTimeForTooltip(time: string, hours: number): string {
  const date = dayjs(time)
  if (hours < 24) {
    return date.format('HH:mm:ss')
  }
  return date.format('MM/DD HH:mm')
}


export function getTaskColor(tasks: PingTaskInfo[], chartColors: string[], taskId: number): string {
  const taskIndex = tasks.findIndex(t => t.id === taskId)
  const safeIndex = Math.max(0, taskIndex % chartColors.length)
  return chartColors[safeIndex]!
}

export interface PingChartOptionInput {
  selectedTasks: PingTaskInfo[]
  chartData: Array<Record<string, unknown>>
  selectedHours: number
  colorVisionFriendly: boolean
  cutPeak: boolean
  tasks: PingTaskInfo[]
  chartColors: string[]
  isDark: boolean
}

export function buildPingChartOption(input: PingChartOptionInput) {
  const chartColors = input.chartColors
  const theme = pingChartThemeColors(input.isDark)
  const taskList = input.selectedTasks
  const data = input.chartData
  const hours = input.selectedHours

  // 构建 series，确保颜色与卡片一致
  const series = taskList.map((task, index) => {
    const color = getTaskColor(input.tasks, chartColors, task.id)
    const lineType = input.colorVisionFriendly
      ? (ACCESSIBLE_LINE_TYPES[index % ACCESSIBLE_LINE_TYPES.length] ?? 'solid')
      : 'solid'
    return {
      name: task.name,
      type: 'line' as const,
      data: data.map(d => d[task.id] as number | null ?? null),
      smooth: input.cutPeak ? 0.6 : 0.1,
      showSymbol: false,
      connectNulls: false,
      lineStyle: { width: 1.5, color, cap: 'round' as const, type: lineType },
      itemStyle: { color }, // 确保 symbol 颜色一致
    }
  })

  // 颜色映射表（用于 Tooltip）
  const colorMap = new Map<number, string>()
  input.tasks.forEach((task, idx) => {
    const safeIdx = Math.max(0, idx % chartColors.length)
    colorMap.set(task.id, chartColors[safeIdx]!)
  })

  return {
    animation: false,
    // 全局颜色设置（用于图例等）
    color: input.tasks.map((_, idx) => {
      const safeIdx = Math.max(0, idx % chartColors.length)
      return chartColors[safeIdx]!
    }),
    tooltip: {
      ...pingBaseTooltipConfig(theme),
      formatter: (params: unknown) => {
        const p = params as Array<{ seriesName: string, value: number | null, dataIndex: number }>
        if (!p.length)
          return ''
        const firstParam = p[0]
        if (!firstParam)
          return ''
        const rowData = data[firstParam.dataIndex]
        if (!rowData)
          return ''

        const time = rowData.time as string
        const timeStr = formatPingTimeForTooltip(time, hours)
        let html = `<div style="font-weight:600;margin-bottom:6px;color:${theme.textSecondary}">${timeStr}</div>`
        html += '<div style="display:flex;flex-direction:column;gap:4px">'

        // 按延迟值排序显示
        const sortedParams = [...p].sort((a, b) => (a.value ?? 0) - (b.value ?? 0))

        for (const item of sortedParams) {
          if (item.value !== null && item.value !== undefined) {
            // 通过任务名找到对应的任务ID，再获取颜色
            const task = input.tasks.find(t => t.name === item.seriesName)
            const color = task ? colorMap.get(task.id) || chartColors[0] : chartColors[0]
            const colorDot = `<span style="display:inline-block;width:8px;height:8px;border-radius:50%;background:${color};margin-right:8px;flex-shrink:0"></span>`
            html += `<div style="display:flex;align-items:center">${colorDot}<span style="flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${escapeHTML(item.seriesName)}</span><span style="margin-left:auto;font-weight:600;margin-left:16px;font-variant-numeric:tabular-nums">${Math.round(item.value)} ms</span></div>`
          }
        }
        html += '</div>'
        return html
      },
    },
    legend: {
      type: 'scroll',
      bottom: 0,
      itemWidth: 12,
      itemHeight: 12,
      itemGap: 16,
      icon: 'roundRect',
      textStyle: { fontSize: 11, color: theme.textSecondary },
      data: taskList.map(t => t.name),
    },
    grid: PING_CHART_MARGIN,
    xAxis: {
      type: 'category',
      data: data.map(d => formatPingTime(d.time as string, (input.selectedHours >= 24))),
      axisLabel: {
        fontSize: 11,
        color: theme.textSecondary,
        margin: 12,
      },
      axisLine: {
        show: true,
        lineStyle: { color: theme.borderColor, width: 1 },
      },
      axisTick: { show: false },
      boundaryGap: false,
    },
    yAxis: {
      type: 'value',
      name: '延迟 (ms)',
      nameTextStyle: { color: theme.textSecondary },
      axisLabel: { fontSize: 11, color: theme.textSecondary, formatter: '{value}' },
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: {
        lineStyle: {
          color: theme.splitLineColor,
          type: 'dashed' as const,
        },
      },
    },
    series,
  }
}
