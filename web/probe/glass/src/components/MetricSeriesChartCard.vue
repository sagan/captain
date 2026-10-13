<script setup lang="ts">
import { computed } from 'vue'
import VChart from 'vue-echarts'
import { CardX } from '@/components/ui/card-x'
import { buildMetricSeriesChartOption, metricSeriesLatestText } from '@shared/charts/metricSeries'
import MetricChartHeader from './MetricChartHeader.vue'
import '@shared/utils/echarts'

type MetricChartTone = 'rose' | 'amber' | 'emerald' | 'cyan' | 'sky' | 'violet' | 'orange' | 'slate'
type MetricValueKind = 'bytes' | 'bytesPerSecond' | 'count' | 'milliseconds' | 'percent' | 'temperature'

export interface MetricChartSeriesData {
  name: string
  color: string
  kind: MetricValueKind
  data: Array<[string, number | null]>
  dashed?: boolean
}

const props = withDefaults(defineProps<{
  title: string
  icon: string
  tone?: MetricChartTone
  series: MetricChartSeriesData[]
  order?: number
  subtitle?: string
  latest?: string
  percentScale?: boolean
}>(), {
  tone: 'slate',
  order: 99,
  subtitle: '',
  latest: '',
  percentScale: false,
})


const latestText = computed(() => metricSeriesLatestText({ latest: props.latest, series: props.series }))

const chartOption = computed(() => buildMetricSeriesChartOption({ series: props.series, percentScale: props.percentScale }))
</script>

<template>
  <CardX
    size="small"
    class="bg-background/50 border-none hover:bg-background transition-all rounded-md"
    :style="{ order }"
  >
    <template #header>
      <MetricChartHeader :title="title" :icon="icon" :tone="tone" :subtitle="subtitle">
        {{ latestText }}
      </MetricChartHeader>
    </template>
    <div class="h-48">
      <VChart :option="chartOption" autoresize />
    </div>
  </CardX>
</template>
