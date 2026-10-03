<script setup lang="ts">
// Layout from vlongx/komari-theme-Glassmorphism-three-network (MIT).
import { computed } from 'vue'
import { t } from '@/i18n'
import { useAppStore } from '@/stores/app'
import { DataTooltip } from '@/components/ui/data-tooltip'
import { useNodeCarrierPingDisplay } from '@/composables/useNodeCarrierPingDisplay'
const props = defineProps<{ uuid: string; nodeName: string; online: boolean; enabled: boolean }>()
const emit = defineEmits<{ click: [] }>()
const app = useAppStore()
const { carrierDisplays } = useNodeCarrierPingDisplay(() => props.uuid, () => props.enabled)
const panelClass = computed(() => app.nodeCardSize === 'large' ? 'h-[128px] gap-2 p-2' : app.nodeCardSize === 'comfortable' ? 'h-[120px] gap-1.5 p-2' : app.nodeCardSize === 'mini' ? 'h-[92px] gap-1 p-1' : 'h-[112px] gap-1.5 p-1.5')
</script>
<template>
<div class="grid grid-cols-2 gap-1.5">
          <button
            type="button"
            @click.stop="emit('click')"
            :aria-label="`${nodeName} ${t('网络监测')}`"
            class="group/panel relative flex flex-col rounded-lg bg-slate-500/5"
            :class="[panelClass, !online ? 'blur-xs opacity-50' : '']"
          >
            <div class="flex items-center justify-between text-[11px] leading-none">
              <span class="text-muted-foreground">{{ t('延迟') }}</span>
              <span class="text-[10px] text-muted-foreground/70">{{ t('三网') }}</span>
            </div>

            <div class="grid min-h-0 flex-1 grid-rows-3 gap-1">
              <div
                v-for="carrier in carrierDisplays"
                :key="`${carrier.key}-latency`" :data-carrier="carrier.key" data-carrier-metric="latency"
                class="flex min-h-0 flex-col gap-[2px]"
                :title="carrier.latencyTooltip"
              >
                <div class="flex items-center justify-between text-[10px] leading-none">
                  <span class="flex min-w-0 items-center gap-1 text-muted-foreground">
                    <span class="size-1.5 shrink-0 rounded-full" :class="carrier.dotClass" />
                    <span class="truncate">{{ carrier.label }}</span>
                  </span>
                  <span class="shrink-0 tabular-nums font-medium">{{ carrier.latencyDisplay }}</span>
                </div>
                <div
                  class="grid h-1.5 items-end gap-[1px] opacity-80 group-hover/panel:opacity-100"
                  :style="{ gridTemplateColumns: `repeat(${carrier.latencyBars.length}, minmax(0, 1fr))` }"
                >
                  <DataTooltip
                    v-for="bar in carrier.latencyBars" :key="bar.key"
                    placement="top" :content="bar.tooltip" class="h-full w-full"
                  >
                    <span
                      class="block h-full w-full rounded-[1px] transition-transform duration-150 group-hover/data-tooltip:scale-y-160 group-hover/panel:opacity-60 group-hover/data-tooltip:!opacity-100"
                      :class="bar.className"
                    />
                  </DataTooltip>
                </div>
              </div>
            </div>
          </button>

          <button
            type="button"
            @click.stop="emit('click')"
            :aria-label="`${nodeName} ${t('网络监测')}`"
            class="group/panel relative flex flex-col rounded-lg bg-slate-500/5"
            :class="[panelClass, !online ? 'blur-xs opacity-50' : '']"
          >
            <div class="flex items-center justify-between text-[11px] leading-none">
              <span class="text-muted-foreground">{{ t('丢包') }}</span>
              <span class="text-[10px] text-muted-foreground/70">{{ t('三网') }}</span>
            </div>

            <div class="grid min-h-0 flex-1 grid-rows-3 gap-1">
              <div
                v-for="carrier in carrierDisplays"
                :key="`${carrier.key}-loss`" :data-carrier="carrier.key" data-carrier-metric="loss"
                class="flex min-h-0 flex-col gap-[2px]"
                :title="carrier.lossTooltip"
              >
                <div class="flex items-center justify-between text-[10px] leading-none">
                  <span class="flex min-w-0 items-center gap-1 text-muted-foreground">
                    <span class="size-1.5 shrink-0 rounded-full" :class="carrier.dotClass" />
                    <span class="truncate">{{ carrier.label }}</span>
                  </span>
                  <span class="shrink-0 tabular-nums font-medium">{{ carrier.lossDisplay }}</span>
                </div>
                <div
                  class="grid h-1.5 items-end gap-[1px] opacity-80 group-hover/panel:opacity-100"
                  :style="{ gridTemplateColumns: `repeat(${carrier.lossBars.length}, minmax(0, 1fr))` }"
                >
                  <DataTooltip
                    v-for="bar in carrier.lossBars" :key="bar.key"
                    placement="top" :content="bar.tooltip" class="h-full w-full"
                  >
                    <span
                      class="block h-full w-full rounded-[1px] transition-transform duration-150 group-hover/data-tooltip:scale-y-160 group-hover/panel:opacity-60 group-hover/data-tooltip:!opacity-100"
                      :class="bar.className"
                    />
                  </DataTooltip>
                </div>
              </div>
            </div>
          </button>
        </div>
</template>
