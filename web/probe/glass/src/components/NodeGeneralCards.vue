<script setup lang="ts">
import type { NodeData } from '@/stores/nodes'
import type { CurrencyCode, ExchangeRateSource } from '@shared/utils/financeHelper'
import type { GeneralMetricCard } from '@shared/views/generalCards'
import { buildGeneralCards } from '@shared/views/generalCards'
import { Icon } from '@iconify/vue'
import { useNow } from '@vueuse/core'
import { computed, defineAsyncComponent, onMounted, ref } from 'vue'
import NodeEarthGlobe from '@/components/NodeEarthGlobe.vue'
import { CardX } from '@/components/ui/card-x'
import { DataTooltip } from '@/components/ui/data-tooltip'
import { UI_CONFIG } from '@shared/constants/ui'
import { useAppStore } from '@/stores/app'
import { useNodesStore } from '@/stores/nodes'
import * as financeHelper from '@shared/utils/financeHelper'

const props = defineProps<{
  nodes?: NodeData[]
  globeNodes?: NodeData[]
  transitionKey?: string
}>()
const appStore = useAppStore()
const nodesStore = useNodesStore()
const FinanceDetailsDialog = defineAsyncComponent(() => import('@/components/FinanceDetailsDialog.vue'))
// 未登录且开启「未登录隐藏价格」时，屏蔽金额类信息
const showPrice = computed(() => appStore.privateFeaturesAllowed || !appStore.hidePriceWhenLoggedOut)
const exchangeRates = ref(financeHelper.DEFAULT_EXCHANGE_RATES)
const dailyExchangeRates = ref(financeHelper.DEFAULT_EXCHANGE_RATES)
const exchangeRateSource = ref<ExchangeRateSource | 'loading'>('loading')
const exchangeRateUpdatedAt = ref<number | null>(null)
const financeCurrency = ref<CurrencyCode>('CNY')
const excludeFreeNodes = ref(true)
const financeDetailsOpen = ref(false)
const currentTime = useNow({ interval: 1000 })
const summaryNodes = computed(() => props.nodes ?? nodesStore.visibleNodes)
const summaryTransitionKey = computed(() => props.transitionKey ?? nodesStore.visibleNodes.length)
const metricSwitchTransitionProps = computed(() => ({
  ...(appStore.disablePageAnimation
    ? { css: false }
    : { name: 'metric-switch', mode: 'out-in' as const }),
}))

function getMetricSwitchStyle(index: number): Record<string, string> {
  return {
    '--metric-switch-delay': `${index * UI_CONFIG.motion.staggerMs}ms`,
  }
}

const visibleCards = computed(() => buildGeneralCards({
  nodes: summaryNodes.value,
  byteDecimals: appStore.byteDecimals,
  homeHighLoadThreshold: appStore.homeHighLoadThreshold,
  homeExpiringDays: appStore.homeExpiringDays,
  homeTrafficWarningThreshold: appStore.homeTrafficWarningThreshold,
  excludeFreeNodes: excludeFreeNodes.value,
  exchangeRates: exchangeRates.value,
  financeCurrency: financeCurrency.value,
  showPrice: showPrice.value,
  now: currentTime.value,
}, appStore.generalCardOrder))
const showEarth = computed(() => !appStore.hideEarth)
const isTiledEarth = computed(() => showEarth.value && appStore.earthRenderer === 'tiled')
const shouldRenderHeader = computed(() => showEarth.value || visibleCards.value.length > 0)
const hasExtraCards = computed(() => visibleCards.value.length > 6)
const wrapperClass = computed(() => {
  if (!showEarth.value)
    return 'p-4 grid grid-cols-1 gap-2 h-auto'

  if (isTiledEarth.value)
    return 'p-3 sm:p-4 grid grid-cols-12 gap-2 sm:gap-3 h-auto min-h-[40rem] sm:min-h-[30rem] md:min-h-[36rem] lg:min-h-[40rem]'

  return hasExtraCards.value
    ? 'p-4 grid grid-cols-12 gap-2 h-auto md:min-h-58'
    : 'p-4 grid grid-cols-12 grid-rows-1 gap-2 h-auto md:h-58'
})
const earthClass = computed(() => {
  if (isTiledEarth.value)
    return 'col-span-12 row-start-2 min-h-[18rem] h-[18rem] sm:h-[20rem] md:h-[24rem] lg:h-[28rem]'

  return 'col-span-12 col-start-1 md:col-span-6 md:col-start-7 md:row-start-1'
})
const cardGridClass = computed(() => {
  if (!showEarth.value)
    return 'col-span-1 grid grid-cols-3 md:grid-cols-6 gap-2'

  if (isTiledEarth.value)
    return 'col-span-12 row-start-1 z-9 grid grid-cols-12 auto-rows-[4.75rem] sm:auto-rows-[5rem] md:auto-rows-[5.8rem] gap-2 sm:gap-3'

  return hasExtraCards.value
    ? 'h-auto -mt-42 md:mt-0 col-span-12 row-start-3 z-9 md:h-auto md:col-span-6 md:row-start-1 grid grid-cols-12 auto-rows-[5rem] md:auto-rows-[7rem] gap-2'
    : 'h-42 -mt-42 md:mt-0 col-span-12 row-start-3 z-9 md:h-auto md:col-span-6 md:row-start-1 grid grid-cols-12 grid-rows-2 gap-2'
})
const cardClass = 'group relative z-10 h-full bg-background/50 border-none hover:bg-background backdrop-blur-sm md:backdrop-blur-none transition-all'
const cardPositionClasses = [
  'col-span-4 row-span-1 col-start-1 row-start-1',
  'col-span-4 row-span-1 col-start-1 row-start-2',
  'col-span-4 row-span-1 col-start-5 row-start-1',
  'col-span-4 row-span-1 col-start-5 row-start-2',
  'col-span-4 row-span-1 col-start-9 row-start-1',
  'col-span-4 row-span-1 col-start-9 row-start-2',
]
const tiledCardPositionClasses = [
  'col-span-6 sm:col-span-3 row-span-1 sm:col-start-1 row-start-1',
  'col-span-6 sm:col-span-3 row-span-1 sm:col-start-4 row-start-1',
  'col-span-6 sm:col-span-3 row-span-1 sm:col-start-7 row-start-2 sm:row-start-1',
  'col-span-6 sm:col-span-3 row-span-1 sm:col-start-10 row-start-2 sm:row-start-1',
  'col-span-6 sm:col-span-3 row-span-1 sm:col-start-1 row-start-3 sm:row-start-2',
  'col-span-6 sm:col-span-3 row-span-1 sm:col-start-4 row-start-3 sm:row-start-2',
  'col-span-6 sm:col-span-3 row-span-1 sm:col-start-7 row-start-4 sm:row-start-2',
  'col-span-6 sm:col-span-3 row-span-1 sm:col-start-10 row-start-4 sm:row-start-2',
]
const unitClass = 'text-[11px] md:text-xs font-medium text-muted-foreground truncate'

function getCardPositionClass(index: number): string {
  if (!showEarth.value)
    return 'col-span-1 min-h-18 md:min-h-28'

  if (isTiledEarth.value)
    return tiledCardPositionClasses[index] ?? 'col-span-6 sm:col-span-3 row-span-1'

  return cardPositionClasses[index] ?? 'col-span-4 row-span-1'
}

function activateCard(card: GeneralMetricCard) {
  if (card.action === 'financeDetails')
    financeDetailsOpen.value = true
}

function handleCardKeydown(event: KeyboardEvent, card: GeneralMetricCard) {
  if (!card.action || (event.key !== 'Enter' && event.key !== ' '))
    return
  event.preventDefault()
  activateCard(card)
}

function updateFinanceCurrency(currency: CurrencyCode) {
  financeCurrency.value = currency
  financeHelper.setStoredFinanceCurrency(currency)
}

function updateExcludeFreeNodes(exclude: boolean) {
  excludeFreeNodes.value = exclude
  financeHelper.setExcludeFreeNodes(exclude)
}

function updateExchangeRate(currency: CurrencyCode, value: number) {
  financeHelper.setExchangeRateOverride(currency, value)
  exchangeRates.value = { ...exchangeRates.value, [currency]: value, CNY: 1 }
}

function resetExchangeRates() {
  financeHelper.clearExchangeRateOverrides()
  exchangeRates.value = { ...dailyExchangeRates.value }
}

onMounted(async () => {
  financeCurrency.value = financeHelper.getStoredFinanceCurrency()
  excludeFreeNodes.value = financeHelper.shouldExcludeFreeNodes()

  const { rates, source, updatedAt } = await financeHelper.getDailyExchangeRates()
  dailyExchangeRates.value = rates
  exchangeRates.value = financeHelper.applyExchangeRateOverrides(rates)
  exchangeRateSource.value = source
  exchangeRateUpdatedAt.value = updatedAt
})
</script>

<template>
  <div v-if="shouldRenderHeader" :class="wrapperClass">
    <NodeEarthGlobe
      v-if="showEarth"
      :nodes="globeNodes"
      :class="earthClass"
    />

    <div v-if="visibleCards.length > 0" :class="cardGridClass">
      <CardX
        v-for="(card, index) in visibleCards"
        :key="card.key"
        :data-general-card-key="card.key"
        hoverable
        :class="[cardClass, getCardPositionClass(index), card.action && 'cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring']"
        content-class="h-full !p-3"
        :role="card.action ? 'button' : undefined"
        :tabindex="card.action ? 0 : undefined"
        :aria-label="card.action ? `查看${card.label}明细` : undefined"
        @click="activateCard(card)"
        @keydown="handleCardKeydown($event, card)"
      >
        <div class="flex h-full flex-col justify-between gap-1">
          <div class="flex items-start justify-between gap-2">
            <span class="text-xs font-medium tracking-wider text-muted-foreground truncate">{{ card.label }}</span>
            <Icon
              :icon="card.icon" :width="20" :height="20"
              class="shrink-0 text-slate-500/20 group-hover:text-slate-500 transition-colors"
            />
          </div>
          <DataTooltip
            as="span"
            placement="top"
            :content="card.tooltip"
            class="min-w-0"
            content-class="whitespace-pre px-2 py-1 left-0 -translate-x-0 leading-normal"
          >
            <Transition v-bind="metricSwitchTransitionProps">
              <div
                :key="`${card.key}-${summaryTransitionKey}`"
                class="flex items-baseline gap-1 min-w-0"
                :style="getMetricSwitchStyle(index)"
              >
                <span class="text-md md:text-2xl font-bold leading-none tracking-tight truncate">
                  {{ card.value }}
                </span>
                <span v-if="card.unit" :class="unitClass">
                  {{ card.unit }}
                </span>
              </div>
            </Transition>
          </DataTooltip>
        </div>
      </CardX>
    </div>
  </div>

  <FinanceDetailsDialog
    v-if="financeDetailsOpen"
    v-model:open="financeDetailsOpen"
    :nodes="summaryNodes"
    :rates="exchangeRates"
    :source="exchangeRateSource"
    :rates-updated-at="exchangeRateUpdatedAt"
    :currency="financeCurrency"
    :exclude-free="excludeFreeNodes"
    :now="currentTime"
    @update:currency="updateFinanceCurrency"
    @update:exclude-free="updateExcludeFreeNodes"
    @update:rate="updateExchangeRate"
    @reset-rates="resetExchangeRates"
  />
</template>

<style scoped>
.metric-switch-enter-active,
.metric-switch-leave-active {
  transition:
    opacity 160ms ease,
    transform 180ms cubic-bezier(0.22, 1, 0.36, 1),
    filter 180ms ease;
}

.metric-switch-enter-active {
  transition-delay: var(--metric-switch-delay, 0ms);
}

.metric-switch-enter-from {
  opacity: 0;
  transform: translateY(6px);
  filter: blur(3px);
}

.metric-switch-leave-to {
  opacity: 0;
  transform: translateY(-4px);
  filter: blur(2px);
}

@media (prefers-reduced-motion: reduce) {
  .metric-switch-enter-active,
  .metric-switch-leave-active {
    transition: none;
    transition-delay: 0ms;
  }

  .metric-switch-enter-from,
  .metric-switch-leave-to {
    opacity: 1;
    transform: none;
    filter: none;
  }
}
</style>
