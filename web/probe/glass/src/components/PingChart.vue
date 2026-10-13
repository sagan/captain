<script setup lang="ts">
import { t } from '@shared/i18n'
import type { PingRecord, PingTaskInfo } from '@shared/utils/rpc'
import { Icon } from '@iconify/vue'
import dayjs from 'dayjs'
import { computed, onBeforeUnmount, onMounted, reactive, ref, shallowRef, watch, watchEffect } from 'vue'
import VChart from 'vue-echarts'
import { Button } from '@/components/ui/button'
import { Empty } from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { PING_RECORD_MAX_COUNT } from '@shared/constants/load'
import { loadPingRecordsWithTasks } from '@shared/services/history.service'
import { useAppStore } from '@/stores/app'
import { getChartSeriesPalette } from '@shared/utils/chartPalette'
import { cutPeakValues } from '@shared/utils/recordHelper'
import '@shared/utils/echarts' // 共享 ECharts 配置
import type { CustomRange } from '@shared/charts/ping'
import { buildLatestValues, buildPingChartOption, buildPingViews, loadMetricPingPayload, mergePingRecords } from '@shared/charts/ping'

const props = defineProps<{
  uuid: string
}>()

const appStore = useAppStore()
const isDark = computed(() => appStore.isDark)

// 图表主题相关颜色
const chartColors = reactive(getChartSeriesPalette(appStore.colorVisionFriendly))

watchEffect(() => {
  chartColors.splice(0, chartColors.length, ...getChartSeriesPalette(appStore.colorVisionFriendly))
})

// 从 publicSettings 获取记录保留时间
const maxPingRecordPreserveTime = computed(() => appStore.publicSettings?.ping_record_preserve_time || 168)

// 视图选项
const presetViews = [
  { label: t('1 小时'), hours: 1 },
  { label: t('6 小时'), hours: 6 },
  { label: t('12 小时'), hours: 12 },
  { label: t('1 天'), hours: 24 },
]
const CUSTOM_VIEW_LABEL = t('自定义')
const DEFAULT_CUSTOM_RANGE_HOURS = 24

// 可用视图列表
const availableViews = computed(() => buildPingViews(maxPingRecordPreserveTime.value, presetViews, CUSTOM_VIEW_LABEL))

// 当前选中的视图
const selectedView = ref<string>('')
const customStartInput = ref('')
const customEndInput = ref('')
const appliedCustomRange = shallowRef<CustomRange | null>(null)
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
const selectedHours = computed(() => {
  if (isCustomRange.value)
    return appliedCustomRange.value?.hours ?? customRange.value?.hours ?? DEFAULT_CUSTOM_RANGE_HOURS

  const view = availableViews.value.find(v => v.label === selectedView.value)
  return view?.hours || 1
})

function ensureDefaultCustomRange() {
  if (customStartInput.value && customEndInput.value)
    return

  const end = dayjs()
  const hours = Math.max(1, Math.min(DEFAULT_CUSTOM_RANGE_HOURS, maxPingRecordPreserveTime.value))
  customStartInput.value = end.subtract(hours, 'hour').format('YYYY-MM-DDTHH:mm')
  customEndInput.value = end.format('YYYY-MM-DDTHH:mm')
}

// 初始化默认视图
watch(availableViews, (views) => {
  const firstView = views[0]
  if (firstView && !selectedView.value) {
    selectedView.value = firstView.label
  }
}, { immediate: true })

// ==================== 数据状态 ====================
const remoteData = shallowRef<PingRecord[]>([])
const tasks = shallowRef<PingTaskInfo[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const legacyCustomRangeFallback = ref(false)

// 任务选择
const selectedTaskIds = ref<number[]>([])
const cutPeak = ref(false)
const isTouchTooltipMode = ref(false)
const activeTaskTooltipId = ref<number | null>(null)
const smoothInfoTooltipOpen = ref(false)

let coarsePointerMediaQuery: MediaQueryList | null = null
let fetchRecordsSequence = 0

function syncTouchTooltipMode() {
  if (typeof window === 'undefined') {
    isTouchTooltipMode.value = false
    return
  }

  const hasCoarsePointer = window.matchMedia('(pointer: coarse)').matches
  const hasTouchPoints = typeof navigator !== 'undefined' && navigator.maxTouchPoints > 0
  isTouchTooltipMode.value = hasCoarsePointer || hasTouchPoints
}

function setTaskTooltipOpen(taskId: number, open: boolean) {
  activeTaskTooltipId.value = open ? taskId : activeTaskTooltipId.value === taskId ? null : activeTaskTooltipId.value
}

function toggleTaskTooltip(taskId: number) {
  if (!isTouchTooltipMode.value)
    return

  activeTaskTooltipId.value = activeTaskTooltipId.value === taskId ? null : taskId
  smoothInfoTooltipOpen.value = false
}

function toggleSmoothInfoTooltip() {
  if (!isTouchTooltipMode.value)
    return

  smoothInfoTooltipOpen.value = !smoothInfoTooltipOpen.value
  if (smoothInfoTooltipOpen.value) {
    activeTaskTooltipId.value = null
  }
}

// ==================== 数据获取 ====================

async function fetchRecords() {
  const sequence = ++fetchRecordsSequence
  const requestedUuid = props.uuid
  if (!requestedUuid)
    return

  if (isCustomRange.value && !customRange.value) {
    remoteData.value = []
    tasks.value = []
    error.value = customRangeError.value || '请选择有效的自定义时间范围'
    legacyCustomRangeFallback.value = false
    loading.value = false
    return
  }

  appliedCustomRange.value = isCustomRange.value ? customRange.value : null

  loading.value = true
  error.value = null

  try {
    const metricPayload = await loadMetricPingPayload(requestedUuid, appliedCustomRange.value, isCustomRange.value, selectedHours.value).catch(() => null)
    if (sequence !== fetchRecordsSequence || requestedUuid !== props.uuid)
      return

    legacyCustomRangeFallback.value = !metricPayload && isCustomRange.value
    const range = appliedCustomRange.value
    const legacyHours = range
      ? Math.min(
          maxPingRecordPreserveTime.value,
          Math.max(range.hours, Math.ceil(dayjs().diff(range.start, 'hour', true))),
        )
      : selectedHours.value
    const result = metricPayload ?? await loadPingRecordsWithTasks(legacyHours, PING_RECORD_MAX_COUNT, requestedUuid)
    if (sequence !== fetchRecordsSequence || requestedUuid !== props.uuid)
      return

    const records = result.records
    records.sort((a, b) => dayjs(a.time).valueOf() - dayjs(b.time).valueOf())

    remoteData.value = records
    tasks.value = result.tasks

    if (tasks.value.length > 0 && selectedTaskIds.value.length === 0) {
      selectedTaskIds.value = tasks.value.map(t => t.id)
    }
  }
  catch (err) {
    if (sequence !== fetchRecordsSequence || requestedUuid !== props.uuid)
      return

    error.value = err instanceof Error ? err.message : '获取数据失败'
    legacyCustomRangeFallback.value = false
    remoteData.value = []
    tasks.value = []
  }
  finally {
    if (sequence === fetchRecordsSequence)
      loading.value = false
  }
}

// ==================== 数据处理 ====================

const mergedData = computed(() => mergePingRecords(remoteData.value, tasks.value, appliedCustomRange.value, isCustomRange.value, selectedHours.value))

const chartData = computed(() => {
  let data = mergedData.value
  const selectedKeys = selectedTaskIds.value.map(String)

  if (selectedKeys.length === 0)
    return []

  if (cutPeak.value) {
    data = cutPeakValues(data, selectedKeys)
  }

  // Missing samples and failed attempts remain gaps; never interpolate them.

  return data
})

// ==================== 工具函数 ====================

// ==================== 任务选择 ====================

// 最新值统计（从服务端 tasks 获取，保持颜色顺序）
const latestValues = computed(() => buildLatestValues(tasks.value, remoteData.value, chartColors))

const selectedTasks = computed(() => {
  return tasks.value.filter(t => selectedTaskIds.value.includes(t.id))
})

// 切换任务选中状态
function toggleTask(taskId: number) {
  if (selectedTaskIds.value.includes(taskId)) {
    selectedTaskIds.value = selectedTaskIds.value.filter(id => id !== taskId)
  }
  else {
    selectedTaskIds.value = [...selectedTaskIds.value, taskId]
  }
}

function showAllTasks() {
  selectedTaskIds.value = tasks.value.map(t => t.id)
}

function hideAllTasks() {
  selectedTaskIds.value = []
}

// ==================== 图表配置 ====================

const pingChartOption = computed(() => buildPingChartOption({
  selectedTasks: selectedTasks.value,
  chartData: chartData.value,
  selectedHours: selectedHours.value,
  colorVisionFriendly: appStore.colorVisionFriendly,
  cutPeak: cutPeak.value,
  tasks: tasks.value,
  chartColors,
  isDark: isDark.value,
}))

// ==================== 生命周期 ====================

watch(selectedView, () => {
  selectedTaskIds.value = []
  if (isCustomRange.value)
    ensureDefaultCustomRange()
  fetchRecords()
})

watch(() => props.uuid, () => {
  remoteData.value = []
  tasks.value = []
  selectedTaskIds.value = []
  activeTaskTooltipId.value = null
  smoothInfoTooltipOpen.value = false
  fetchRecords()
})

onMounted(() => {
  syncTouchTooltipMode()
  coarsePointerMediaQuery = window.matchMedia('(pointer: coarse)')
  coarsePointerMediaQuery.addEventListener('change', syncTouchTooltipMode)

  const firstView = availableViews.value[0]
  if (firstView && !selectedView.value) {
    selectedView.value = firstView.label
  }
  fetchRecords()
})

onBeforeUnmount(() => {
  coarsePointerMediaQuery?.removeEventListener('change', syncTouchTooltipMode)
})
</script>

<template>
  <div class="flex flex-col gap-4">
    <!-- 时间选择器 -->
    <div class="flex flex-col gap-2">
      <Tabs v-model="selectedView" class="w-full items-center">
        <div class="min-w-0 flex-1 overflow-x-auto rounded-sm pointer-events-auto">
          <TabsList class="w-max h-8 bg-background/50 backdrop-blur-xl rounded-md">
            <TabsTrigger
              v-for="view in availableViews" :key="view.label" :value="view.label"
              class="h-6.5 flex-none shrink-0 text-xs border-none data-[state=active]:text-green-600 shadow-none rounded-sm"
            >
              {{ view.label }}
            </TabsTrigger>
          </TabsList>
        </div>
        <div class="md:flex-1" />
        <div class="flex gap-2 items-center">
          <Button
            variant="ghost" size="xs" class="h-7 rounded-sm bg-background/50 hover:bg-background border-none"
            :class="selectedTaskIds.length === tasks.length ? 'shadow-[0_0_0_2px] shadow-green-600/10 text-green-600' : ''"
            @click="showAllTasks"
          >
            {{ t('全选') }}
          </Button>
          <Button
            variant="ghost" size="xs" class="h-7 rounded-sm bg-background/50 hover:bg-background border-none"
            :class="!selectedTaskIds.length && 'shadow-[0_0_0_2px] shadow-green-600/10 text-green-600'"
            @click="hideAllTasks"
          >
            {{ t('全不选') }}
          </Button>
        </div>
      </Tabs>

      <div v-if="isCustomRange" class="flex w-full flex-col items-center gap-2 sm:flex-row sm:justify-center">
        <div class="grid w-full gap-2 sm:w-auto sm:grid-cols-[minmax(0,13rem)_minmax(0,13rem)_auto]">
          <Input
            v-model="customStartInput"
            type="datetime-local"
            aria-label="延迟图开始时间"
            class="h-8 bg-background/50 text-xs"
          />
          <Input
            v-model="customEndInput"
            type="datetime-local"
            aria-label="延迟图结束时间"
            class="h-8 bg-background/50 text-xs"
          />
          <Button
            type="button"
            size="sm"
            variant="outline"
            :disabled="!customRange"
            class="h-8 text-xs"
            @click="fetchRecords"
          >
            应用
          </Button>
        </div>
        <div v-if="customRangeError" class="text-[11px] text-orange-500">
          {{ customRangeError }}
        </div>
        <div v-else-if="legacyCustomRangeFallback" class="text-[11px] text-muted-foreground">
          旧接口按可用保留时长回溯，再裁剪到所选区间
        </div>
      </div>
    </div>

    <!-- 内容区域 -->
    <Spinner :show="loading" content-class="flex flex-col gap-4">
      <div v-if="error" class="text-red-500 py-8 text-center">
        {{ error }}
      </div>
      <div v-else-if="tasks.length === 0 && !loading" class="py-8">
        <Empty description="暂无延迟数据" />
      </div>

      <template v-else>
        <!-- 最新值统计卡片（可点击切换选中状态） -->
        <div
          v-if="latestValues.length > 0" class="gap-3 grid"
          style="grid-template-columns: repeat(auto-fit, minmax(180px, 1fr))"
        >
          <div
            v-for="task in latestValues" :key="task.id"
            :data-ping-task-id="task.id"
            class="p-2 rounded-md bg-background/50 hover:bg-background hover:shadow-[0_0_0_2px] hover:shadow-primary/10 flex gap-3 cursor-pointer select-none transition-all items-center"
            :class="[!selectedTaskIds.includes(task.id) && 'opacity-30']"
            :onmouseover="(e: MouseEvent) => ((e.currentTarget as HTMLElement).style.borderColor = task.color)"
            :onmouseout="(e: MouseEvent) => ((e.currentTarget as HTMLElement).style.borderColor = '')"
            @click="toggleTask(task.id)"
          >
            <div class="flex-1 min-w-0">
              <TooltipProvider>
                <div class="flex gap-2 items-center">
                  <div class="rounded h-4 w-1" :style="{ backgroundColor: task.color }" />
                  <span class="text-sm font-semibold truncate">{{ task.name }}</span>
                  <div class="flex-1" />
                  <Tooltip
                    :open="isTouchTooltipMode ? activeTaskTooltipId === task.id : undefined"
                    @update:open="(open) => setTaskTooltipOpen(task.id, open)"
                  >
                    <TooltipTrigger as-child>
                      <Button variant="ghost" size="icon-xs" class="text-slate-500" @click.stop="toggleTaskTooltip(task.id)">
                        <Icon icon="carbon:information" :width="14" :height="14" />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent class="!rounded p-3">
                      <div class="text-xs gap-x-4 gap-y-1.5 grid grid-cols-4">
                        <template v-if="task.min !== undefined">
                          <span class="text-muted-foreground">最小</span>
                          <span class="font-medium">{{ Math.round(task.min) }} ms</span>
                        </template>
                        <template v-if="task.max !== undefined">
                          <span class="text-muted-foreground">最大</span>
                          <span class="font-medium">{{ Math.round(task.max) }} ms</span>
                        </template>
                        <template v-if="task.avg !== undefined">
                          <span class="text-muted-foreground">平均</span>
                          <span class="font-medium">{{ Math.round(task.avg) }} ms</span>
                        </template>
                        <template v-if="task.latest !== undefined">
                          <span class="text-muted-foreground">最新</span>
                          <span class="font-medium">{{ Math.round(task.latest) }} ms</span>
                        </template>
                        <template v-if="task.p50 !== undefined">
                          <span class="text-muted-foreground">P50</span>
                          <span class="font-medium">{{ Math.round(task.p50) }} ms</span>
                        </template>
                        <template v-if="task.p99 !== undefined">
                          <span class="text-muted-foreground">P99</span>
                          <span class="font-medium">{{ Math.round(task.p99) }} ms</span>
                        </template>
                        <template v-if="task.p99_p50_ratio !== undefined">
                          <span class="text-muted-foreground">波动率</span>
                          <span class="font-medium">{{ task.p99_p50_ratio.toFixed(2) }}</span>
                        </template>
                        <template v-if="task.interval !== undefined">
                          <span class="text-muted-foreground">间隔</span>
                          <span class="font-medium">{{ task.interval }}s</span>
                        </template>
                        <template v-if="task.type">
                          <span class="text-muted-foreground">类型</span>
                          <span class="font-medium">{{ task.type.toUpperCase() }}</span>
                        </template>
                        <template v-if="task.stddev !== undefined">
                          <span class="text-muted-foreground">标准差</span>
                          <span class="font-medium">{{ task.stddev.toFixed(1) }}</span>
                        </template>
                        <template v-if="task.total !== undefined">
                          <span class="text-muted-foreground">总数</span>
                          <span class="font-medium">{{ task.total }}</span>
                        </template>
                        <template v-if="task.valid !== undefined">
                          <span class="text-muted-foreground">有效</span>
                          <span class="font-medium">{{ task.valid }}</span>
                        </template>
                      </div>
                    </TooltipContent>
                  </Tooltip>
                </div>
              </TooltipProvider>
              <div class="text-xs mt-1 flex gap-1.5 items-center text-muted-foreground">
                <span class="font-medium" title="平均延迟">
                  {{ task.avg !== undefined ? `${Math.round(task.avg)}ms` : '-' }}
                </span>
                <span class="opacity-60">·</span>
                <span :title="t('丢包率')">{{ task.loss.toFixed(2) }}%{{ task.loss_approximate ? '≈' : '' }}</span>
                <template v-if="task.p99_p50_ratio !== undefined">
                  <span class="opacity-60">·</span>
                  <span title="波动率">{{ task.p99_p50_ratio.toFixed(2) }}</span>
                </template>
              </div>
            </div>
          </div>
        </div>

        <!-- 平滑峰值开关 -->
        <div class="flex flex-wrap gap-4 items-center py-2 justify-between">
          <TooltipProvider>
            <div class="flex gap-2 items-center">
              <Button
                variant="ghost" size="xs" class="h-7 rounded-sm bg-background/50 hover:bg-background border-none"
                :class="cutPeak && 'shadow-[0_0_0_2px] shadow-green-600/10 text-green-600'" @click="cutPeak = !cutPeak"
              >
                {{ t('平滑峰值') }}
              </Button>
              <Tooltip
                :open="isTouchTooltipMode ? smoothInfoTooltipOpen : undefined"
                @update:open="(open) => smoothInfoTooltipOpen = open"
              >
                <TooltipTrigger as-child>
                  <Button variant="ghost" size="icon-xs" class="text-slate-500" @click.stop="toggleSmoothInfoTooltip">
                    <Icon icon="carbon:information" :width="14" :height="14" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  <span>使用 EWMA 算法平滑数据并过滤突变值</span>
                </TooltipContent>
              </Tooltip>
            </div>
          </TooltipProvider>
        </div>

        <!-- 图表 -->
        <div class="h-80 bg-background/50 p-4 rounded-md">
          <VChart :option="pingChartOption" autoresize />
        </div>
      </template>
    </Spinner>
  </div>
</template>
