<script setup lang="ts">
import { computed } from 'vue'
import { snapshot } from '@/captain'
import { t } from '@/i18n'
import { selectPings } from '@/ping-selection'
import { useNodeCarrierPingDisplay } from '@/composables/useNodeCarrierPingDisplay'
import { useNodePingDisplay } from '@/composables/useNodePingDisplay'

const props = defineProps<{
  uuid: string
  online: boolean
  enabled: boolean
}>()

const emit = defineEmits<{
  click: []
}>()

const node = computed(() => snapshot.value?.nodes.find(n => String(n.id) === props.uuid))
const threeNetwork = computed(() => selectPings(node.value, snapshot.value?.carrier_ping ?? false).threeNetwork)
const { carrierDisplays } = useNodeCarrierPingDisplay(() => props.uuid, () => props.enabled && threeNetwork.value)

const {
  latencyRenderBars,
  lossRenderBars,
} = useNodePingDisplay(() => props.uuid, { enabled: () => props.enabled && !threeNetwork.value })
</script>

<template>
  <!-- Keep the upstream 64px list row: card panels are too tall for this cell. -->
  <button
    v-if="threeNetwork"
    type="button"
    class="group flex w-full min-w-0 flex-col gap-[2px] pr-4 text-left"
    :aria-label="`${node?.name ?? ''} ${t('网络监测')}`"
    @click.stop="emit('click')"
    @keydown.stop
  >
    <span
      v-for="carrier in carrierDisplays" :key="carrier.key" :data-carrier="carrier.key"
      class="flex min-w-0 items-center gap-1"
      :title="`${carrier.label} · ${t('延迟')} ${carrier.latencyDisplay} · ${t('丢包')} ${carrier.lossDisplay}`"
    >
      <span class="size-1.5 shrink-0 rounded-full" :class="carrier.dotClass" />
      <span class="flex min-w-0 flex-1 flex-col gap-[1px]">
        <span
          v-for="metric in (['latency', 'loss'] as const)" :key="metric" :data-carrier-metric="metric"
          class="grid h-1 items-end gap-[1px] opacity-80 group-hover:opacity-100"
          :style="{ gridTemplateColumns: `repeat(${carrier[`${metric}Bars`].length}, minmax(0, 1fr))` }"
        >
          <span
            v-for="bar in carrier[`${metric}Bars`]" :key="bar.key"
            :title="`${t(metric === 'latency' ? '延迟' : '丢包')} · ${bar.tooltip}`" :aria-label="bar.tooltip"
            class="block h-full w-full rounded-[1px]" :class="bar.className"
          />
        </span>
      </span>
    </span>
  </button>
  <button
    v-else
    type="button"
    class="group flex w-full flex-col gap-[1px] pr-4 text-left"
    aria-label="打开延迟和丢包监测"
    @click.stop="emit('click')"
    @keydown.stop
  >
    <div class="group/panel relative items-center gap-1 opacity-80 hover:opacity-100">
      <div
        class="grid h-1 cursor-auto items-end gap-[1px] transition-all hover:h-2.5"
        :style="{ gridTemplateColumns: `repeat(${latencyRenderBars.length}, minmax(0, 1fr))` }"
      >
        <span
          v-for="bar in latencyRenderBars"
          :key="bar.key"
          :title="bar.tooltip"
          :aria-label="bar.tooltip"
          class="h-full w-full"
        >
          <span class="block h-full w-full rounded-[1px] transition-all group-hover:opacity-50 hover:scale-y-160 hover:opacity-100" :class="bar.className" />
        </span>
      </div>
    </div>
    <div class="group/panel relative items-center gap-1 opacity-80 hover:opacity-100">
      <div
        class="grid h-1 cursor-auto items-end gap-[1px] transition-all hover:h-2.5"
        :style="{ gridTemplateColumns: `repeat(${lossRenderBars.length}, minmax(0, 1fr))` }"
      >
        <span
          v-for="bar in lossRenderBars"
          :key="bar.key"
          :title="bar.tooltip"
          :aria-label="bar.tooltip"
          class="h-full w-full"
        >
          <span class="block h-full w-full rounded-[1px] transition-all group-hover:opacity-50 hover:scale-y-160 hover:opacity-100" :class="bar.className" />
        </span>
      </div>
    </div>
  </button>
</template>
