<script setup lang="ts">
import { t } from '@/i18n'
import { defineAsyncComponent } from 'vue'
import { AppDialog } from '@/components/ui/app-dialog'

defineProps<{
  open: boolean
  uuid: string
  nodeName: string
}>()

const emit = defineEmits<{
  'update:open': [open: boolean]
}>()

const PingChart = defineAsyncComponent(() => import('@/components/PingChart.vue'))
</script>

<template>
  <AppDialog
    :open="open"
    :title="`${nodeName} ${t('延迟')} / ${t('丢包')}`"
    :description="t('探测任务延迟、丢包率与波动统计')"
    content-class="max-w-6xl"
    @update:open="emit('update:open', $event)"
  >
    <PingChart v-if="open && uuid" :uuid="uuid" />
  </AppDialog>
</template>
