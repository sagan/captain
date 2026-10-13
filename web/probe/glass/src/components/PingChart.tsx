import { Fragment, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { HTMLAttributes } from 'react'
import { Icon } from '@iconify/react'
import dayjs from 'dayjs'
import { t } from '@shared/i18n'
import type { CustomRange } from '@shared/charts/ping'
import { buildLatestValues, buildPingChartOption, buildPingViews, loadMetricPingPayload, mergePingRecords } from '@shared/charts/ping'
import { PING_RECORD_MAX_COUNT } from '@shared/constants/load'
import { loadPingRecordsWithTasks } from '@shared/services/history.service'
import { getChartSeriesPalette } from '@shared/utils/chartPalette'
import { cutPeakValues } from '@shared/utils/recordHelper'
import type { PingRecord, PingTaskInfo } from '@shared/utils/rpc'
import '@shared/utils/echarts' // 共享 ECharts 配置
import EChart from '@/components/EChart'
import { Button } from '@/components/ui/button'
import { Empty } from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { vclass } from '@/lib/vclass'
import { useApp } from '@/stores/app'

const DEFAULT_CUSTOM_RANGE_HOURS = 24

function presetViews() {
  return [
    { label: t('1 小时'), hours: 1 },
    { label: t('6 小时'), hours: 6 },
    { label: t('12 小时'), hours: 12 },
    { label: t('1 天'), hours: 24 },
  ]
}

function parseRange(startInput: string, endInput: string): CustomRange | null {
  if (!startInput || !endInput)
    return null
  const start = dayjs(startInput)
  const end = dayjs(endInput)
  if (!start.isValid() || !end.isValid() || !end.isAfter(start))
    return null
  return { start, end, hours: Math.max(1, Math.ceil(end.diff(start, 'hour', true))) }
}

const STAT_ROWS: Array<[keyof PingTaskInfo, string, (v: number | string) => string]> = [
  ['min', '最小', v => `${Math.round(v as number)} ms`],
  ['max', '最大', v => `${Math.round(v as number)} ms`],
  ['avg', '平均', v => `${Math.round(v as number)} ms`],
  ['latest', '最新', v => `${Math.round(v as number)} ms`],
  ['p50', 'P50', v => `${Math.round(v as number)} ms`],
  ['p99', 'P99', v => `${Math.round(v as number)} ms`],
  ['p99_p50_ratio', '波动率', v => (v as number).toFixed(2)],
  ['interval', '间隔', v => `${v}s`],
  ['type', '类型', v => String(v).toUpperCase()],
  ['stddev', '标准差', v => (v as number).toFixed(1)],
  ['total', '总数', v => String(v)],
  ['valid', '有效', v => String(v)],
]

export default function PingChart({ uuid, className, ...attrs }: { uuid: string } & HTMLAttributes<HTMLDivElement>) {
  const app = useApp(a => ({ isDark: a.isDark, colorVisionFriendly: a.colorVisionFriendly, publicSettings: a.publicSettings }))
  const chartColors = useMemo(() => getChartSeriesPalette(app.colorVisionFriendly), [app.colorVisionFriendly])
  const maxPingRecordPreserveTime = app.publicSettings?.ping_record_preserve_time || 168
  const CUSTOM_VIEW_LABEL = t('自定义')
  const availableViews = useMemo(() => buildPingViews(maxPingRecordPreserveTime, presetViews(), CUSTOM_VIEW_LABEL), [maxPingRecordPreserveTime, CUSTOM_VIEW_LABEL])

  // watch(availableViews, …, { immediate: true }): the first view is selected before the first render.
  const [selectedView, setSelectedView] = useState<string>(() => availableViews[0]?.label ?? '')
  if (!selectedView && availableViews[0])
    setSelectedView(availableViews[0].label)
  const [customStartInput, setCustomStartInput] = useState('')
  const [customEndInput, setCustomEndInput] = useState('')
  const [appliedCustomRange, setAppliedCustomRange] = useState<CustomRange | null>(null)
  const isCustomRange = selectedView === CUSTOM_VIEW_LABEL
  const customRange = useMemo(() => parseRange(customStartInput, customEndInput), [customStartInput, customEndInput])
  const customRangeError = !isCustomRange || (!customStartInput && !customEndInput)
    ? ''
    : !customStartInput || !customEndInput ? '请选择开始和结束时间' : customRange ? '' : '结束时间必须晚于开始时间'
  const selectedHours = isCustomRange
    ? appliedCustomRange?.hours ?? customRange?.hours ?? DEFAULT_CUSTOM_RANGE_HOURS
    : availableViews.find(v => v.label === selectedView)?.hours || 1

  const [remoteData, setRemoteData] = useState<PingRecord[]>([])
  const [tasks, setTasks] = useState<PingTaskInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [legacyCustomRangeFallback, setLegacyCustomRangeFallback] = useState(false)
  const [selectedTaskIds, setSelectedTaskIds] = useState<number[]>([])
  const [cutPeak, setCutPeak] = useState(false)
  const [isTouchTooltipMode, setIsTouchTooltipMode] = useState(false)
  const [activeTaskTooltipId, setActiveTaskTooltipId] = useState<number | null>(null)
  const [smoothInfoTooltipOpen, setSmoothInfoTooltipOpen] = useState(false)
  const sequence = useRef(0)

  // Latest values for the async fetch (Vue read refs at call time).
  const live = useRef({ uuid, isCustomRange, customRange, customRangeError, selectedHours, maxPingRecordPreserveTime, selectedTaskIds })
  live.current = { uuid, isCustomRange, customRange, customRangeError, selectedHours, maxPingRecordPreserveTime, selectedTaskIds }

  const fetchRecords = useCallback(async () => {
    const seq = ++sequence.current
    const l = live.current
    const requestedUuid = l.uuid
    if (!requestedUuid)
      return
    if (l.isCustomRange && !l.customRange) {
      setRemoteData([])
      setTasks([])
      setError(l.customRangeError || '请选择有效的自定义时间范围')
      setLegacyCustomRangeFallback(false)
      setLoading(false)
      return
    }
    const range = l.isCustomRange ? l.customRange : null
    setAppliedCustomRange(range)
    setLoading(true)
    setError(null)
    try {
      const metricPayload = await loadMetricPingPayload(requestedUuid, range, l.isCustomRange, l.selectedHours).catch(() => null)
      if (seq !== sequence.current || requestedUuid !== live.current.uuid)
        return
      setLegacyCustomRangeFallback(!metricPayload && l.isCustomRange)
      const legacyHours = range
        ? Math.min(l.maxPingRecordPreserveTime, Math.max(range.hours, Math.ceil(dayjs().diff(range.start, 'hour', true))))
        : l.selectedHours
      const result = metricPayload ?? await loadPingRecordsWithTasks(legacyHours, PING_RECORD_MAX_COUNT, requestedUuid)
      if (seq !== sequence.current || requestedUuid !== live.current.uuid)
        return
      const records = result.records
      records.sort((a, b) => dayjs(a.time).valueOf() - dayjs(b.time).valueOf())
      setRemoteData(records)
      setTasks(result.tasks)
      if (result.tasks.length > 0 && live.current.selectedTaskIds.length === 0)
        setSelectedTaskIds(result.tasks.map(task => task.id))
    }
    catch (err) {
      if (seq !== sequence.current || requestedUuid !== live.current.uuid)
        return
      setError(err instanceof Error ? err.message : '获取数据失败')
      setLegacyCustomRangeFallback(false)
      setRemoteData([])
      setTasks([])
    }
    finally {
      if (seq === sequence.current)
        setLoading(false)
    }
  }, [])

  // onMounted: touch detection and the first fetch.
  useEffect(() => {
    const sync = () => {
      const hasCoarsePointer = window.matchMedia('(pointer: coarse)').matches
      const hasTouchPoints = typeof navigator !== 'undefined' && navigator.maxTouchPoints > 0
      setIsTouchTooltipMode(hasCoarsePointer || hasTouchPoints)
    }
    sync()
    const mq = window.matchMedia('(pointer: coarse)')
    mq.addEventListener('change', sync)
    void fetchRecords()
    return () => mq.removeEventListener('change', sync)
  }, [fetchRecords])

  // watch(selectedView): reset selection, default custom range, refetch.
  const prevView = useRef(selectedView)
  useEffect(() => {
    if (prevView.current === selectedView)
      return
    prevView.current = selectedView
    setSelectedTaskIds([])
    live.current.selectedTaskIds = []
    if (selectedView === CUSTOM_VIEW_LABEL && !(customStartInput && customEndInput)) {
      const end = dayjs()
      const hours = Math.max(1, Math.min(DEFAULT_CUSTOM_RANGE_HOURS, maxPingRecordPreserveTime))
      const start = end.subtract(hours, 'hour').format('YYYY-MM-DDTHH:mm')
      const endText = end.format('YYYY-MM-DDTHH:mm')
      setCustomStartInput(start)
      setCustomEndInput(endText)
      live.current.customRange = parseRange(start, endText)
      live.current.isCustomRange = true
      live.current.customRangeError = ''
    }
    void fetchRecords()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedView])

  // watch(() => props.uuid)
  const prevUuid = useRef(uuid)
  useEffect(() => {
    if (prevUuid.current === uuid)
      return
    prevUuid.current = uuid
    setRemoteData([])
    setTasks([])
    setSelectedTaskIds([])
    live.current.selectedTaskIds = []
    setActiveTaskTooltipId(null)
    setSmoothInfoTooltipOpen(false)
    void fetchRecords()
  }, [uuid, fetchRecords])

  const mergedData = useMemo(() => mergePingRecords(remoteData, tasks, appliedCustomRange, isCustomRange, selectedHours), [remoteData, tasks, appliedCustomRange, isCustomRange, selectedHours])
  const chartData = useMemo(() => {
    const keys = selectedTaskIds.map(String)
    if (keys.length === 0)
      return []
    return cutPeak ? cutPeakValues(mergedData, keys) : mergedData
  }, [mergedData, selectedTaskIds, cutPeak])
  const latestValues = useMemo(() => buildLatestValues(tasks, remoteData, chartColors), [tasks, remoteData, chartColors])
  const selectedTasks = useMemo(() => tasks.filter(task => selectedTaskIds.includes(task.id)), [tasks, selectedTaskIds])
  const pingChartOption = useMemo(() => buildPingChartOption({
    selectedTasks,
    chartData,
    selectedHours,
    colorVisionFriendly: app.colorVisionFriendly,
    cutPeak,
    tasks,
    chartColors,
    isDark: app.isDark,
  }), [selectedTasks, chartData, selectedHours, app.colorVisionFriendly, cutPeak, tasks, chartColors, app.isDark])

  function toggleTask(taskId: number) {
    setSelectedTaskIds(ids => ids.includes(taskId) ? ids.filter(id => id !== taskId) : [...ids, taskId])
  }

  return (
    <div {...attrs} className={vclass('flex flex-col gap-4', className)}>
      <div className="flex flex-col gap-2">
        <Tabs value={selectedView} onValueChange={setSelectedView} className="w-full items-center">
          <div className="min-w-0 flex-1 overflow-x-auto rounded-sm pointer-events-auto">
            <TabsList className="w-max h-8 bg-background/50 backdrop-blur-xl rounded-md">
              {availableViews.map(view => (
                <TabsTrigger key={view.label} value={view.label} className="h-6.5 flex-none shrink-0 text-xs border-none data-[state=active]:text-green-600 shadow-none rounded-sm">
                  {view.label}
                </TabsTrigger>
              ))}
            </TabsList>
          </div>
          <div className="md:flex-1" />
          <div className="flex gap-2 items-center">
            <Button
              variant="ghost"
              size="xs"
              className={vclass('h-7 rounded-sm bg-background/50 hover:bg-background border-none', selectedTaskIds.length === tasks.length ? 'shadow-[0_0_0_2px] shadow-green-600/10 text-green-600' : '')}
              onClick={() => setSelectedTaskIds(tasks.map(task => task.id))}
            >
              {t('全选')}
            </Button>
            <Button
              variant="ghost"
              size="xs"
              className={vclass('h-7 rounded-sm bg-background/50 hover:bg-background border-none', !selectedTaskIds.length && 'shadow-[0_0_0_2px] shadow-green-600/10 text-green-600')}
              onClick={() => setSelectedTaskIds([])}
            >
              {t('全不选')}
            </Button>
          </div>
        </Tabs>

        {isCustomRange && (
          <div className="flex w-full flex-col items-center gap-2 sm:flex-row sm:justify-center">
            <div className="grid w-full gap-2 sm:w-auto sm:grid-cols-[minmax(0,13rem)_minmax(0,13rem)_auto]">
              <Input value={customStartInput} onValueChange={v => setCustomStartInput(String(v))} type="datetime-local" aria-label="延迟图开始时间" className="h-8 bg-background/50 text-xs" />
              <Input value={customEndInput} onValueChange={v => setCustomEndInput(String(v))} type="datetime-local" aria-label="延迟图结束时间" className="h-8 bg-background/50 text-xs" />
              <Button type="button" size="sm" variant="outline" disabled={!customRange} className="h-8 text-xs" onClick={() => void fetchRecords()}>
                {' 应用 '}
              </Button>
            </div>
            {customRangeError
              ? <div className="text-[11px] text-orange-500">{customRangeError}</div>
              : legacyCustomRangeFallback
                ? <div className="text-[11px] text-muted-foreground">旧接口按可用保留时长回溯，再裁剪到所选区间</div>
                : null}
          </div>
        )}
      </div>

      <Spinner show={loading} contentClass="flex flex-col gap-4">
        {error
          ? <div className="text-red-500 py-8 text-center">{error}</div>
          : tasks.length === 0 && !loading
            ? (
                <div className="py-8">
                  <Empty description="暂无延迟数据" />
                </div>
              )
            : (
                <>
                  {latestValues.length > 0 && (
                    <div className="gap-3 grid" style={{ gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))' }}>
                      {latestValues.map(task => (
                        <div
                          key={task.id}
                          data-ping-task-id={task.id}
                          className={vclass('p-2 rounded-md bg-background/50 hover:bg-background hover:shadow-[0_0_0_2px] hover:shadow-primary/10 flex gap-3 cursor-pointer select-none transition-all items-center', [!selectedTaskIds.includes(task.id) && 'opacity-30'])}
                          onMouseOver={e => (e.currentTarget.style.borderColor = task.color)}
                          onMouseOut={e => (e.currentTarget.style.borderColor = '')}
                          onClick={() => toggleTask(task.id)}
                        >
                          <div className="flex-1 min-w-0">
                            <TooltipProvider>
                              <div className="flex gap-2 items-center">
                                <div className="rounded h-4 w-1" style={{ backgroundColor: task.color }} />
                                <span className="text-sm font-semibold truncate">{task.name}</span>
                                <div className="flex-1" />
                                <Tooltip
                                  open={isTouchTooltipMode ? activeTaskTooltipId === task.id : undefined}
                                  onOpenChange={open => setActiveTaskTooltipId(cur => open ? task.id : cur === task.id ? null : cur)}
                                >
                                  <TooltipTrigger asChild>
                                    <Button
                                      variant="ghost"
                                      size="icon-xs"
                                      className="text-slate-500"
                                      onClick={(e) => {
                                        e.stopPropagation()
                                        if (!isTouchTooltipMode)
                                          return
                                        setActiveTaskTooltipId(cur => cur === task.id ? null : task.id)
                                        setSmoothInfoTooltipOpen(false)
                                      }}
                                    >
                                      <Icon icon="carbon:information" width={14} height={14} />
                                    </Button>
                                  </TooltipTrigger>
                                  <TooltipContent className="!rounded p-3">
                                    <div className="text-xs gap-x-4 gap-y-1.5 grid grid-cols-4">
                                      {STAT_ROWS.map(([key, label, fmt]) => {
                                        const value = task[key]
                                        if (key === 'type' ? !value : value === undefined)
                                          return null
                                        return (
                                          <Fragment key={key}>
                                            <span className="text-muted-foreground">{label}</span>
                                            <span className="font-medium">{fmt(value as number | string)}</span>
                                          </Fragment>
                                        )
                                      })}
                                    </div>
                                  </TooltipContent>
                                </Tooltip>
                              </div>
                            </TooltipProvider>
                            <div className="text-xs mt-1 flex gap-1.5 items-center text-muted-foreground">
                              <span className="font-medium" title="平均延迟">
                                {task.avg !== undefined ? `${Math.round(task.avg)}ms` : '-'}
                              </span>
                              <span className="opacity-60">·</span>
                              <span title={t('丢包率')}>{`${task.loss.toFixed(2)}%${task.loss_approximate ? '≈' : ''}`}</span>
                              {task.p99_p50_ratio !== undefined && (
                                <>
                                  <span className="opacity-60">·</span>
                                  <span title="波动率">{task.p99_p50_ratio.toFixed(2)}</span>
                                </>
                              )}
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}

                  <div className="flex flex-wrap gap-4 items-center py-2 justify-between">
                    <TooltipProvider>
                      <div className="flex gap-2 items-center">
                        <Button
                          variant="ghost"
                          size="xs"
                          className={vclass('h-7 rounded-sm bg-background/50 hover:bg-background border-none', cutPeak && 'shadow-[0_0_0_2px] shadow-green-600/10 text-green-600')}
                          onClick={() => setCutPeak(v => !v)}
                        >
                          {t('平滑峰值')}
                        </Button>
                        <Tooltip open={isTouchTooltipMode ? smoothInfoTooltipOpen : undefined} onOpenChange={setSmoothInfoTooltipOpen}>
                          <TooltipTrigger asChild>
                            <Button
                              variant="ghost"
                              size="icon-xs"
                              className="text-slate-500"
                              onClick={(e) => {
                                e.stopPropagation()
                                if (!isTouchTooltipMode)
                                  return
                                setSmoothInfoTooltipOpen((open) => {
                                  if (!open)
                                    setActiveTaskTooltipId(null)
                                  return !open
                                })
                              }}
                            >
                              <Icon icon="carbon:information" width={14} height={14} />
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>
                            <span>使用 EWMA 算法平滑数据并过滤突变值</span>
                          </TooltipContent>
                        </Tooltip>
                      </div>
                    </TooltipProvider>
                  </div>

                  <div className="h-80 bg-background/50 p-4 rounded-md">
                    <EChart option={pingChartOption} autoresize />
                  </div>
                </>
              )}
      </Spinner>
    </div>
  )
}
