<script setup lang="ts">
import { escapeHTML } from '@shared/utils/html'
import { t } from '@shared/i18n'
import { shows } from '@/captain'
import type { ChartDashboardCardKey } from '@/stores/app'
import type { NormalizedMetricSeries } from '@shared/utils/metricSeries'
import type { RecordFormat } from '@shared/utils/recordHelper'
import type { MetricQueryParams, MetricSeries, PingTaskInfo, StatusRecord } from '@shared/utils/rpc'
import { Icon } from '@iconify/vue'
import { useIntervalFn } from '@vueuse/core'
import dayjs from 'dayjs'
import { computed, onMounted, reactive, ref, shallowRef, watch, watchEffect } from 'vue'
import VChart from 'vue-echarts'
import MetricChartHeader from '@/components/MetricChartHeader.vue'
import MetricSeriesChartCard from '@/components/MetricSeriesChartCard.vue'
import { Button } from '@/components/ui/button'
import { CardX } from '@/components/ui/card-x'
import { Empty } from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { loadNodeLoadRecords, useNodeLoadStats } from '@/composables/useNodeLoadStats'
import { LOAD_RECORD_MAX_COUNT } from '@shared/constants/load'
import { loadMetricDefinitions, loadPublicPingTasks, queryMetrics } from '@shared/services/metrics.service'
import { useAppStore } from '@/stores/app'
import { useNodesStore } from '@/stores/nodes'
import { getChartSeriesPalette, getLoadChartPalette } from '@shared/utils/chartPalette'
import { gpuUsageFromStatus } from '@shared/utils/gpuHelper'
import { formatBytes, formatBytesSplit } from '@shared/utils/helper'
import { comparePingTaskOrder, createPingTaskOrderMap, metricTags, normalizeMetricSeriesList } from '@shared/utils/metricSeries'
import { fillMissingTimePoints } from '@shared/utils/recordHelper'
import { getSharedRpc } from '@shared/utils/rpc'
import '@shared/utils/echarts' // 共享 ECharts 配置
import { LOAD_METRIC_KEYS, createLoadChartModel } from '@shared/charts/load'
import type { LoadMetricKey, MetricChartSeriesData } from '@shared/charts/load'

const props = defineProps<{
  uuid: string
}>()

const appStore = useAppStore()
const nodesStore = useNodesStore()

// 从 publicSettings 获取记录保留时间
const maxRecordPreserveTime = computed(() => appStore.publicSettings?.record_preserve_time || 720)

const dataUpdateInterval = computed(() => appStore.dataUpdateInterval * 1000)
const detailLoadStatsHours = computed(() => appStore.publicSettings?.record_preserve_time || 720)

// 使用 store 中的 isDark computed
const isDark = computed(() => appStore.isDark)

const chartColors = reactive(getLoadChartPalette(appStore.colorVisionFriendly))

watchEffect(() => {
  Object.assign(chartColors, getLoadChartPalette(appStore.colorVisionFriendly))
})



const CUSTOM_VIEW_LABEL = t('自定义')
const PING_METRIC_KEYS = ['ping.latency_ms', 'ping.loss'] as const
const METRIC_HISTORY_MAX_POINTS = 700
const REALTIME_METRIC_REFRESH_MS = 30_000





interface CustomRange {
  start: dayjs.Dayjs
  end: dayjs.Dayjs
  hours: number
}

// 图表主题相关颜色
const presetViews = [
  { label: t('4 小时'), hours: 4 },
  { label: t('1 天'), hours: 24 },
  { label: t('7 天'), hours: 168 },
  { label: t('30 天'), hours: 720 },
]

// 可用视图列表
const availableViews = computed(() => {
  const views: { label: string, hours?: number }[] = [{ label: t('实时') }]
  const maxHours = maxRecordPreserveTime.value

  for (const v of presetViews) {
    if (maxHours >= v.hours) {
      views.push({ label: v.label, hours: v.hours })
    }
  }

  const maxPreset = presetViews.at(-1)
  if (maxPreset && maxHours > maxPreset.hours) {
    const label = maxHours % 24 === 0
      ? `${Math.floor(maxHours / 24)} 天`
      : `${maxHours} 小时`
    views.push({ label, hours: maxHours })
  }
  else if (maxHours > 4 && !presetViews.some(v => v.hours === maxHours)) {
    const label = maxHours % 24 === 0
      ? `${Math.floor(maxHours / 24)} 天`
      : `${maxHours} 小时`
    views.push({ label, hours: maxHours })
  }

  views.push({ label: CUSTOM_VIEW_LABEL })
  return views
})

// 当前选中的视图
const selectedView = ref<string>(t('实时'))
const customStartInput = ref('')
const customEndInput = ref('')
const selectedHours = computed(() => {
  const view = availableViews.value.find(v => v.label === selectedView.value)
  return view?.hours
})
const isRealtime = computed(() => selectedView.value === t('实时'))
const isCustomRange = computed(() => selectedView.value === CUSTOM_VIEW_LABEL)
const customRange = computed<CustomRange | null>(() => {
  if (!customStartInput.value || !customEndInput.value)
    return null

  const start = dayjs(customStartInput.value)
  const end = dayjs(customEndInput.value)
  if (!start.isValid() || !end.isValid() || !end.isAfter(start))
    return null

  return {
    start,
    end,
    hours: Math.max(1, Math.ceil(end.diff(start, 'hour', true))),
  }
})
const customRangeError = computed(() => {
  if (!isCustomRange.value || (!customStartInput.value && !customEndInput.value))
    return ''
  if (!customStartInput.value || !customEndInput.value)
    return '请选择开始和结束时间'
  return customRange.value ? '' : '结束时间必须晚于开始时间'
})
const effectiveHistoryHours = computed(() => isCustomRange.value ? customRange.value?.hours ?? 4 : selectedHours.value ?? 4)

// 数据状态
const remoteData = shallowRef<StatusRecord[]>([])
const metricData = shallowRef<RecordFormat[] | null>(null)
const rawMetricSeries = shallowRef<NormalizedMetricSeries[]>([])
const availableMetricKeys = shallowRef<Set<string>>(new Set())
const pingTasks = shallowRef<PingTaskInfo[]>([])
const loading = ref(false)
const isInitialLoad = ref(true) // 是否为首次加载（用于控制实时模式下的 NSpin 显示）
const error = ref<string | null>(null)
let lastRealtimeMetricFetchAt = 0

// 节点信息
const nodeInfo = computed(() => nodesStore.nodesByUuid.get(props.uuid))
const { diskPrediction, diskPredictionState } = useNodeLoadStats(
  () => props.uuid,
  {
    hours: () => detailLoadStatsHours.value,
    enabled: () => appStore.diskPredictionEnabled && appStore.privateFeaturesAllowed,
    diskTotal: () => nodeInfo.value?.disk_total ?? 0,
    online: () => nodeInfo.value?.online ?? false,
    permission: 'diskPrediction',
  },
)
const diskPredictionSummary = computed(() => {
  if (!appStore.diskPredictionEnabled || !appStore.privateFeaturesAllowed)
    return ''

  const prediction = diskPrediction.value
  if (prediction) {
    const days = Math.max(0, Math.ceil(prediction.daysUntilFull))
    return days <= 0
      ? '预计已满'
      : `预计 ${days} 天后满`
  }

  const state = diskPredictionState.value
  if (state.reason === 'no_samples')
    return '暂无趋势'
  if (state.reason === 'insufficient_samples')
    return '样本不足'
  if (state.reason === 'insufficient_duration')
    return '趋势积累中'
  return ''
})

// RPC 客户端
const rpc = getSharedRpc()

// ==================== 数据获取 ====================













interface MetricHistoryData {
  records: RecordFormat[]
  series: NormalizedMetricSeries[]
}

async function loadMetricCatalog(): Promise<void> {
  const [definitions, tasks] = await Promise.all([
    loadMetricDefinitions().catch(() => []),
    loadPublicPingTasks().catch(() => []),
  ])
  availableMetricKeys.value = new Set(definitions.map(definition => definition.name))
  pingTasks.value = tasks
}

async function loadMetricHistoryRecords(params: Pick<MetricQueryParams, 'hours' | 'start' | 'end'>): Promise<MetricHistoryData | null> {
  const definitions = await loadMetricDefinitions()
  const availableKeys = new Set(definitions.map(definition => definition.name))
  availableMetricKeys.value = availableKeys
  const metricKeys = LOAD_METRIC_KEYS.filter(key => availableKeys.has(key))
  if (!metricKeys.length)
    return null

  const result = await queryMetrics({
    metric_keys: metricKeys,
    entity_id: props.uuid,
    ...params,
    downsample: true,
    fill_empty: true,
    max_points: METRIC_HISTORY_MAX_POINTS,
    aggregation: 'avg',
    aggregation_by_metric: {
      'net.total.up': 'last',
      'net.total.down': 'last',
    },
  })

  const series = normalizeMetricSeriesList(result.series)
  if (!series.some(item => item.points.length > 0))
    return null

  return {
    records: metricSeriesToRecordFormat(result.series),
    series,
  }
}

async function refreshRealtimeMetricSeries(force = false): Promise<void> {
  const cards = appStore.chartDashboardTemplate.cards
  if (!cards.includes('ping') && !cards.includes('pingLoss')) {
    rawMetricSeries.value = []
    return
  }

  const now = Date.now()
  if (!force && now - lastRealtimeMetricFetchAt < REALTIME_METRIC_REFRESH_MS)
    return
  lastRealtimeMetricFetchAt = now
  const requestedUuid = props.uuid

  try {
    if (!availableMetricKeys.value.size)
      await loadMetricCatalog()

    const metricKeys = PING_METRIC_KEYS.filter(key => availableMetricKeys.value.has(key))
    if (!metricKeys.length) {
      rawMetricSeries.value = []
      return
    }

    const result = await queryMetrics({
      metric_keys: [...metricKeys],
      entity_id: props.uuid,
      hours: 1,
      downsample: true,
      fill_empty: true,
      max_points: 150,
      aggregation: 'avg',
    })
    if (!isRealtime.value || props.uuid !== requestedUuid)
      return
    rawMetricSeries.value = normalizeMetricSeriesList(result.series)
  }
  catch {
    if (isRealtime.value && props.uuid === requestedUuid)
      rawMetricSeries.value = []
  }
}

async function fetchRecentData() {
  if (!props.uuid)
    return

  metricData.value = null
  void refreshRealtimeMetricSeries()

  // 只在首次加载时显示 loading
  if (isInitialLoad.value) {
    loading.value = true
  }
  error.value = null

  try {
    const result = await rpc.getNodeRecentStatus(props.uuid)
    const records = result?.records || []
    records.sort((a, b) => dayjs(a.time).valueOf() - dayjs(b.time).valueOf())
    const maxLength = 150
    remoteData.value = records.slice(-maxLength)
  }
  catch (err) {
    error.value = err instanceof Error ? err.message : '获取数据失败'
    remoteData.value = []
  }
  finally {
    loading.value = false
    isInitialLoad.value = false
  }
}

async function fetchHistoryData() {
  if (!props.uuid)
    return

  if (isCustomRange.value && !customRange.value) {
    metricData.value = null
    rawMetricSeries.value = []
    remoteData.value = []
    error.value = customRangeError.value || '请选择有效的自定义时间范围'
    return
  }

  const range = customRange.value
  const hours = effectiveHistoryHours.value
  const metricParams: Pick<MetricQueryParams, 'hours' | 'start' | 'end'> = isCustomRange.value && range
    ? { start: range.start.toDate().toISOString(), end: range.end.toDate().toISOString() }
    : { hours }

  loading.value = true
  error.value = null

  try {
    const metricHistory = await loadMetricHistoryRecords(metricParams).catch(() => null)
    const hasCpuHistory = metricHistory?.records.some(record => record.cpu !== null && Number.isFinite(record.cpu)) ?? false
    if (metricHistory && hasCpuHistory) {
      metricData.value = metricHistory.records
      rawMetricSeries.value = metricHistory.series
      remoteData.value = []
    }
    else {
      metricData.value = null
      rawMetricSeries.value = metricHistory?.series ?? []
      remoteData.value = await loadNodeLoadRecords(props.uuid, hours, LOAD_RECORD_MAX_COUNT)
    }
  }
  catch (err) {
    error.value = err instanceof Error ? err.message : '获取数据失败'
    remoteData.value = []
    metricData.value = null
    rawMetricSeries.value = []
  }
  finally {
    loading.value = false
  }
}

async function fetchData() {
  if (isRealtime.value) {
    await fetchRecentData()
  }
  else {
    await fetchHistoryData()
  }
}

// ==================== 数据处理 ====================




const metricSeriesColors = reactive(getChartSeriesPalette(appStore.colorVisionFriendly))

watchEffect(() => {
  metricSeriesColors.splice(0, metricSeriesColors.length, ...getChartSeriesPalette(appStore.colorVisionFriendly))
})











// ==================== 工具函数 ====================




// ==================== 图表配置 ====================

// CPU 图表






// ==================== 实时更新 ====================


// Chart series and options come from the shared model (glass-shared/charts/load.ts).
const loadChartModel = computed(() => createLoadChartModel({
  uuid: props.uuid,
  isDark: isDark.value,
  remoteData: remoteData.value,
  metricData: metricData.value,
  rawMetricSeries: rawMetricSeries.value,
  pingTasks: pingTasks.value,
  effectiveHistoryHours: effectiveHistoryHours.value,
  nodeInfo: nodeInfo.value,
  chartColors,
  metricSeriesColors,
  colorVisionFriendly: appStore.colorVisionFriendly,
  gpuChartEnabled: appStore.gpuChartEnabled,
  chartDashboardTemplate: appStore.chartDashboardTemplate,
}))
const chartThemeColors = computed(() => loadChartModel.value.chartThemeColors)
const baseTooltipConfig = computed(() => loadChartModel.value.baseTooltipConfig)
const chartData = computed(() => loadChartModel.value.chartData)
const latestStatus = computed(() => loadChartModel.value.latestStatus)
const hasGpuData = computed(() => loadChartModel.value.hasGpuData)
const pingTaskMap = computed(() => loadChartModel.value.pingTaskMap)
const pingTaskOrder = computed(() => loadChartModel.value.pingTaskOrder)
const trafficChartSeries = computed(() => loadChartModel.value.trafficChartSeries)
const gpuMemoryChartSeries = computed(() => loadChartModel.value.gpuMemoryChartSeries)
const temperatureChartSeries = computed(() => loadChartModel.value.temperatureChartSeries)
const pingChartSeries = computed(() => loadChartModel.value.pingChartSeries)
const pingLossChartSeries = computed(() => loadChartModel.value.pingLossChartSeries)
const hasTrafficData = computed(() => loadChartModel.value.hasTrafficData)
const hasGpuMemoryData = computed(() => loadChartModel.value.hasGpuMemoryData)
const hasTemperatureData = computed(() => loadChartModel.value.hasTemperatureData)
const hasPingData = computed(() => loadChartModel.value.hasPingData)
const hasPingLossData = computed(() => loadChartModel.value.hasPingLossData)
const showDateInAxis = computed(() => loadChartModel.value.showDateInAxis)
const baseXAxisConfig = computed(() => loadChartModel.value.baseXAxisConfig)
const baseYAxisConfig = computed(() => loadChartModel.value.baseYAxisConfig)
const cpuChartOption = computed(() => loadChartModel.value.cpuChartOption)
const memoryChartOption = computed(() => loadChartModel.value.memoryChartOption)
const diskChartOption = computed(() => loadChartModel.value.diskChartOption)
const networkChartOption = computed(() => loadChartModel.value.networkChartOption)
const gpuDeviceUsageEChartSeries = computed(() => loadChartModel.value.gpuDeviceUsageEChartSeries)
const gpuChartOption = computed(() => loadChartModel.value.gpuChartOption)
const chartDashboardCards = computed(() => loadChartModel.value.chartDashboardCards)
const connectionsChartOption = computed(() => loadChartModel.value.connectionsChartOption)
const processChartOption = computed(() => loadChartModel.value.processChartOption)
const gpuDetailsFromStatus: ReturnType<typeof createLoadChartModel>['gpuDetailsFromStatus'] = (...args: Parameters<ReturnType<typeof createLoadChartModel>['gpuDetailsFromStatus']>) => (loadChartModel.value.gpuDetailsFromStatus as (...a: unknown[]) => never)(...args)
const averageGpuMemoryPercent: ReturnType<typeof createLoadChartModel>['averageGpuMemoryPercent'] = (...args: Parameters<ReturnType<typeof createLoadChartModel>['averageGpuMemoryPercent']>) => (loadChartModel.value.averageGpuMemoryPercent as (...a: unknown[]) => never)(...args)
const statusToRecordFormat: ReturnType<typeof createLoadChartModel>['statusToRecordFormat'] = (...args: Parameters<ReturnType<typeof createLoadChartModel>['statusToRecordFormat']>) => (loadChartModel.value.statusToRecordFormat as (...a: unknown[]) => never)(...args)
const metricValue: ReturnType<typeof createLoadChartModel>['metricValue'] = (...args: Parameters<ReturnType<typeof createLoadChartModel>['metricValue']>) => (loadChartModel.value.metricValue as (...a: unknown[]) => never)(...args)
const getMetricDeviceKey: ReturnType<typeof createLoadChartModel>['getMetricDeviceKey'] = (...args: Parameters<ReturnType<typeof createLoadChartModel>['getMetricDeviceKey']>) => (loadChartModel.value.getMetricDeviceKey as (...a: unknown[]) => never)(...args)
const getMetricDeviceIndex: ReturnType<typeof createLoadChartModel>['getMetricDeviceIndex'] = (...args: Parameters<ReturnType<typeof createLoadChartModel>['getMetricDeviceIndex']>) => (loadChartModel.value.getMetricDeviceIndex as (...a: unknown[]) => never)(...args)
const getMetricDeviceName: ReturnType<typeof createLoadChartModel>['getMetricDeviceName'] = (...args: Parameters<ReturnType<typeof createLoadChartModel>['getMetricDeviceName']>) => (loadChartModel.value.getMetricDeviceName as (...a: unknown[]) => never)(...args)
const ensureMetricRow: ReturnType<typeof createLoadChartModel>['ensureMetricRow'] = (...args: Parameters<ReturnType<typeof createLoadChartModel>['ensureMetricRow']>) => (loadChartModel.value.ensureMetricRow as (...a: unknown[]) => never)(...args)
const applyMetricPoint: ReturnType<typeof createLoadChartModel>['applyMetricPoint'] = (...args: Parameters<ReturnType<typeof createLoadChartModel>['applyMetricPoint']>) => (loadChartModel.value.applyMetricPoint as (...a: unknown[]) => never)(...args)
const finalizeGpuRows: ReturnType<typeof createLoadChartModel>['finalizeGpuRows'] = (...args: Parameters<ReturnType<typeof createLoadChartModel>['finalizeGpuRows']>) => (loadChartModel.value.finalizeGpuRows as (...a: unknown[]) => never)(...args)
const getGpuDeviceNames: ReturnType<typeof createLoadChartModel>['getGpuDeviceNames'] = (...args: Parameters<ReturnType<typeof createLoadChartModel>['getGpuDeviceNames']>) => (loadChartModel.value.getGpuDeviceNames as (...a: unknown[]) => never)(...args)
const metricSeriesToRecordFormat: ReturnType<typeof createLoadChartModel>['metricSeriesToRecordFormat'] = (...args: Parameters<ReturnType<typeof createLoadChartModel>['metricSeriesToRecordFormat']>) => (loadChartModel.value.metricSeriesToRecordFormat as (...a: unknown[]) => never)(...args)
const seriesHasData: ReturnType<typeof createLoadChartModel>['seriesHasData'] = (...args: Parameters<ReturnType<typeof createLoadChartModel>['seriesHasData']>) => (loadChartModel.value.seriesHasData as (...a: unknown[]) => never)(...args)
const recordMetricSeries: ReturnType<typeof createLoadChartModel>['recordMetricSeries'] = (...args: Parameters<ReturnType<typeof createLoadChartModel>['recordMetricSeries']>) => (loadChartModel.value.recordMetricSeries as (...a: unknown[]) => never)(...args)
const gpuDeviceEntries: ReturnType<typeof createLoadChartModel>['gpuDeviceEntries'] = (...args: Parameters<ReturnType<typeof createLoadChartModel>['gpuDeviceEntries']>) => (loadChartModel.value.gpuDeviceEntries as (...a: unknown[]) => never)(...args)
const pingSeries: ReturnType<typeof createLoadChartModel>['pingSeries'] = (...args: Parameters<ReturnType<typeof createLoadChartModel>['pingSeries']>) => (loadChartModel.value.pingSeries as (...a: unknown[]) => never)(...args)
const formatTime: ReturnType<typeof createLoadChartModel>['formatTime'] = (...args: Parameters<ReturnType<typeof createLoadChartModel>['formatTime']>) => (loadChartModel.value.formatTime as (...a: unknown[]) => never)(...args)
const formatTimeForTooltip: ReturnType<typeof createLoadChartModel>['formatTimeForTooltip'] = (...args: Parameters<ReturnType<typeof createLoadChartModel>['formatTimeForTooltip']>) => (loadChartModel.value.formatTimeForTooltip as (...a: unknown[]) => never)(...args)
const isChartCardEnabled: ReturnType<typeof createLoadChartModel>['isChartCardEnabled'] = (...args: Parameters<ReturnType<typeof createLoadChartModel>['isChartCardEnabled']>) => (loadChartModel.value.isChartCardEnabled as (...a: unknown[]) => never)(...args)
const getChartCardOrder: ReturnType<typeof createLoadChartModel>['getChartCardOrder'] = (...args: Parameters<ReturnType<typeof createLoadChartModel>['getChartCardOrder']>) => (loadChartModel.value.getChartCardOrder as (...a: unknown[]) => never)(...args)
const getChartCardStyle: ReturnType<typeof createLoadChartModel>['getChartCardStyle'] = (...args: Parameters<ReturnType<typeof createLoadChartModel>['getChartCardStyle']>) => (loadChartModel.value.getChartCardStyle as (...a: unknown[]) => never)(...args)

// 使用 VueUse 的 useIntervalFn 自动管理定时器
const { pause: pauseRealtimeUpdate, resume: resumeRealtimeUpdate } = useIntervalFn(
  () => fetchData(),
  dataUpdateInterval,
  { immediate: false },
)

// 根据是否为实时模式控制定时器
watch(isRealtime, (realtime) => {
  if (realtime) {
    resumeRealtimeUpdate()
  }
  else {
    pauseRealtimeUpdate()
  }
}, { immediate: true })

// 生命周期 ====================

watch(selectedView, () => {
  isInitialLoad.value = true // 切换视图时重置首次加载状态
  if (!isCustomRange.value || customRange.value)
    fetchData()
})

watch(() => props.uuid, () => {
  remoteData.value = []
  metricData.value = null
  rawMetricSeries.value = []
  lastRealtimeMetricFetchAt = 0
  isInitialLoad.value = true // 切换节点时重置首次加载状态
  fetchData()
})

watch(chartDashboardCards, () => {
  if (isRealtime.value) {
    lastRealtimeMetricFetchAt = 0
    void refreshRealtimeMetricSeries(true)
  }
})

onMounted(() => {
  void loadMetricCatalog()
  fetchData()
})
</script>

<template>
  <div class="flex flex-col gap-4">
    <!-- 时间选择器 -->
    <div class="flex flex-col items-center gap-2">
      <Tabs v-model="selectedView" class="w-full items-center">
        <TabsList class="h-8 bg-background/50 backdrop-blur-xl pointer-events-auto rounded-md" data-load-chart-range>
          <TabsTrigger
            v-for="view in availableViews" :key="view.label" :value="view.label"
            class="h-6.5 text-xs border-none data-[state=active]:text-green-600 shadow-none rounded-sm"
          >
            {{ view.label }}
          </TabsTrigger>
        </TabsList>
      </Tabs>

      <div v-if="isCustomRange" class="flex w-full flex-col items-center gap-2 sm:flex-row sm:justify-center">
        <div class="grid w-full gap-2 sm:w-auto sm:grid-cols-[minmax(0,13rem)_minmax(0,13rem)_auto]">
          <Input
            v-model="customStartInput"
            type="datetime-local"
            aria-label="负载图开始时间"
            class="h-8 bg-background/50 text-xs"
          />
          <Input
            v-model="customEndInput"
            type="datetime-local"
            aria-label="负载图结束时间"
            class="h-8 bg-background/50 text-xs"
          />
          <Button
            type="button"
            size="sm"
            variant="outline"
            :disabled="!customRange"
            class="h-8 text-xs"
            @click="fetchData"
          >
            应用
          </Button>
        </div>
        <div v-if="customRangeError" class="text-[11px] text-orange-500">
          {{ customRangeError }}
        </div>
      </div>
    </div>

    <!-- 内容区域 -->
    <Spinner :show="loading">
      <div v-if="error" class="text-red-500 py-8 text-center">
        {{ error }}
      </div>
      <div v-else-if="chartData.length === 0 && !loading" class="py-8">
        <Empty description="暂无负载数据" />
      </div>

      <!-- 图表网格 -->
      <div v-else class="gap-4 grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3">
        <!-- CPU 卡片 -->
        <CardX v-if="isChartCardEnabled('cpu')" size="small" class="bg-background/50 border-none hover:bg-background transition-all rounded-md" data-load-chart-card="cpu" :style="getChartCardStyle('cpu')">
          <template #header>
            <MetricChartHeader :title="t('CPU 与负载')" icon="tabler:cpu" tone="rose">
              <div v-if="latestStatus?.cpu != null" class="text-xs flex gap-0.5 items-baseline">
                <span data-latest-cpu>{{ latestStatus.cpu.toFixed(1) }}</span>
                <span>%</span>
              </div>
              <span v-else>-</span>
            </MetricChartHeader>
          </template>
          <div class="h-48">
            <VChart :option="cpuChartOption" autoresize />
          </div>
        </CardX>

        <!-- 内存卡片 -->
        <CardX v-if="isChartCardEnabled('memory')" size="small" class="bg-background/50 border-none hover:bg-background transition-all rounded-md" :style="getChartCardStyle('memory')">
          <template #header>
            <MetricChartHeader :title="t('内存与 Swap')" icon="tabler:database" tone="violet">
              <div class="text-xs flex gap-1 items-baseline">
                <template v-if="latestStatus?.ram != null">
                  <span>{{ formatBytesSplit(latestStatus.ram).value }}</span>
                  <span>{{ formatBytesSplit(latestStatus.ram).unit }}</span>
                </template>
                <span v-else>-</span>
                <span>·</span>
                <template v-if="nodeInfo?.mem_total">
                  <span>{{
                    formatBytesSplit(nodeInfo.mem_total).value }}</span>
                  <span>{{ formatBytesSplit(nodeInfo.mem_total).unit }}</span>
                </template>
                <span v-else>-</span>
              </div>
            </MetricChartHeader>
          </template>
          <div class="h-48">
            <VChart :option="memoryChartOption" autoresize />
          </div>
        </CardX>

        <!-- 磁盘卡片 -->
        <CardX v-if="isChartCardEnabled('disk')" size="small" class="bg-background/50 border-none hover:bg-background transition-all rounded-md" :style="getChartCardStyle('disk')">
          <template #header>
            <MetricChartHeader :title="t('磁盘')" icon="tabler:device-floppy" tone="emerald" :subtitle="diskPredictionSummary">
              <div class="text-xs flex gap-1 items-baseline shrink-0">
                <template v-if="latestStatus?.disk != null">
                  <span>{{ formatBytesSplit(latestStatus.disk).value }}</span>
                  <span>{{ formatBytesSplit(latestStatus.disk).unit }}</span>
                </template>
                <span v-else>-</span>
                <span>·</span>
                <template v-if="nodeInfo?.disk_total">
                  <span>{{ formatBytesSplit(nodeInfo.disk_total).value }}</span>
                  <span>{{ formatBytesSplit(nodeInfo.disk_total).unit }}</span>
                </template>
                <span v-else>-</span>
              </div>
            </MetricChartHeader>
          </template>
          <div class="h-48">
            <VChart :option="diskChartOption" autoresize />
          </div>
        </CardX>

        <!-- 网络卡片 -->
        <CardX v-if="isChartCardEnabled('network')" size="small" class="bg-background/50 border-none hover:bg-background transition-all rounded-md" :style="getChartCardStyle('network')">
          <template #header>
            <MetricChartHeader :title="t('实时网络')" icon="tabler:network" tone="sky">
              <div class="text-xs flex gap-2 items-baseline">
                <span class="flex flex-row items-center justify-center gap-0.5">
                  <Icon icon="tabler:chevron-up" width="12" height="12" />
                  <template v-if="latestStatus?.net_out != null">
                    {{ formatBytesSplit(latestStatus.net_out).value }}
                    {{ formatBytesSplit(latestStatus.net_out).unit }}/s
                  </template>
                  <template v-else>-</template>
                </span>
                <span class="flex flex-row items-center justify-center gap-0.5">
                  <Icon icon="tabler:chevron-down" width="12" height="12" />
                  <template v-if="latestStatus?.net_in != null">
                    {{ formatBytesSplit(latestStatus.net_in).value }}
                    {{ formatBytesSplit(latestStatus.net_in).unit }}/s
                  </template>
                  <template v-else>-</template>
                </span>
              </div>
            </MetricChartHeader>
          </template>
          <div class="h-48">
            <VChart :option="networkChartOption" autoresize />
          </div>
        </CardX>

        <MetricSeriesChartCard
          v-if="isChartCardEnabled('traffic')"
          title="累计与周期流量"
          icon="tabler:arrows-transfer-up-down"
          tone="sky"
          :series="trafficChartSeries"
          :order="getChartCardOrder('traffic')"
        />

        <!-- GPU 卡片 -->
        <CardX v-if="isChartCardEnabled('gpu')" size="small" class="bg-background/50 border-none hover:bg-background transition-all rounded-md" :style="getChartCardStyle('gpu')">
          <template #header>
            <MetricChartHeader title="GPU 利用率" icon="tabler:device-desktop-analytics" tone="cyan" :subtitle="getGpuDeviceNames(latestStatus)">
              <div class="text-xs flex gap-1 items-baseline shrink-0">
                <template v-if="latestStatus?.gpu_usage != null || latestStatus?.gpu != null">
                  <span>{{ (latestStatus.gpu_usage ?? latestStatus.gpu ?? 0).toFixed(1) }}</span>
                  <span>%</span>
                </template>
                <span v-else>-</span>
              </div>
            </MetricChartHeader>
          </template>
          <div class="h-48">
            <VChart :option="gpuChartOption" autoresize />
          </div>
        </CardX>

        <MetricSeriesChartCard
          v-if="isChartCardEnabled('gpuMemory')"
          title="GPU 显存"
          icon="tabler:stack-2"
          tone="violet"
          :series="gpuMemoryChartSeries"
          :order="getChartCardOrder('gpuMemory')"
        />

        <MetricSeriesChartCard
          v-if="isChartCardEnabled('temperature')"
          title="温度"
          icon="tabler:temperature"
          tone="orange"
          :series="temperatureChartSeries"
          :order="getChartCardOrder('temperature')"
        />

        <!-- 连接数卡片 -->
        <CardX v-if="isChartCardEnabled('connections')" size="small" class="bg-background/50 border-none hover:bg-background transition-all rounded-md" :style="getChartCardStyle('connections')">
          <template #header>
            <MetricChartHeader :title="t('网络连接')" icon="tabler:binary-tree" tone="amber">
              <div class="text-xs flex gap-1 items-baseline">
                <span>TCP: {{ latestStatus?.connections ?? '-' }}</span>
                <span>·</span>
                <span>UDP: {{ latestStatus?.connections_udp ?? '-' }}</span>
              </div>
            </MetricChartHeader>
          </template>
          <div class="h-48">
            <VChart :option="connectionsChartOption" autoresize />
          </div>
        </CardX>

        <!-- 进程卡片 -->
        <CardX v-if="isChartCardEnabled('process')" size="small" class="bg-background/50 border-none hover:bg-background transition-all rounded-md" :style="getChartCardStyle('process')">
          <template #header>
            <MetricChartHeader :title="t('进程')" icon="tabler:activity" tone="slate">
              <span class="text-xs">
                {{ latestStatus?.process ?? '-' }}
              </span>
            </MetricChartHeader>
          </template>
          <div class="h-48">
            <VChart :option="processChartOption" autoresize />
          </div>
        </CardX>

        <MetricSeriesChartCard
          v-if="isChartCardEnabled('ping')"
          title="Ping 延迟"
          icon="tabler:radar"
          tone="cyan"
          :series="pingChartSeries"
          :order="getChartCardOrder('ping')"
        />

        <MetricSeriesChartCard
          v-if="isChartCardEnabled('pingLoss')"
          title="Ping 丢包"
          icon="tabler:cloud-exclamation"
          tone="rose"
          :series="pingLossChartSeries"
          :order="getChartCardOrder('pingLoss')"
          percent-scale
        />
      </div>
    </Spinner>
  </div>
</template>
