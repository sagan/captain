import { useStorageAsync } from '@vueuse/core'
import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import type { GlassCustomColors } from '@shared/types/theme'
import type { PermissionKey, VerifyLoginOptions } from '@shared/services/auth.service'
import type { MeInfo, PublicSettings } from '@shared/utils/api'
import type { ByteDecimalsConfig } from '@shared/utils/helper'
import { getAuthSession, requirePermission, setAuthSessionFromLogin, verifyLogin } from '@shared/services/auth.service'
import {
  BYTE_DECIMALS,
  getBeijingHour,
  isValidThemeMode,
  deriveAppSettings,
  normalizeThemeSettings,
} from '@shared/stores/app-settings'
import type {
  ChartDashboardCardKey,
  ChartDashboardTemplate,
  DetailMetricCardKey,
  GeneralCardKey,
  HomeQuickControlKey,
  Lang,
  ManagedThemeMode,
  NodeViewMode,
  ThemeMode,
} from '@shared/stores/app-settings'
export type {
  ThemeMode,
  ManagedThemeMode,
  GeneralCardKey,
  HomeQuickControlKey,
  DetailMetricCardKey,
  NodeListMetadataField,
  ChartDashboardCardKey,
  ChartDashboardTemplate,
} from '@shared/stores/app-settings'
export type { GlassCustomColors } from '@shared/types/theme'

const useAppStore = defineStore('app', () => {
  const loading = ref<boolean>(true)

  // 使用 VueUse 的 useStorageAsync 实现自动持久化
  const themeMode = useStorageAsync<ThemeMode>('themeMode', 'auto', localStorage)
  const lang = ref<Lang>('zh-CN')
  const publicSettings = ref<PublicSettings>()
  const nodeSelectedGroup = useStorageAsync<string>('nodeSelectedGroup', 'all', localStorage)
  const favoriteNodeIds = useStorageAsync<string[]>('theme:favorite-nodes:v1', [], localStorage)
  const isLoggedIn = ref<boolean>(getAuthSession().authenticated)
  const authStatus = ref(getAuthSession().status)
  const privateFeaturesAllowed = computed(() => authStatus.value === 'authenticated')
  const connectionError = ref<boolean>(false)
  const homeAdvancedToolsVisible = ref(false)
  const favoriteNodeIdSet = computed(() => new Set(
    (Array.isArray(favoriteNodeIds.value) ? favoriteNodeIds.value : [])
      .filter((id): id is string => typeof id === 'string' && Boolean(id.trim())),
  ))

  function isFavoriteNode(uuid: string): boolean {
    return favoriteNodeIdSet.value.has(uuid)
  }

  function toggleFavoriteNode(uuid: string): void {
    const normalized = uuid.trim()
    if (!normalized)
      return
    favoriteNodeIds.value = favoriteNodeIdSet.value.has(normalized)
      ? [...favoriteNodeIdSet.value].filter(id => id !== normalized)
      : [...favoriteNodeIdSet.value, normalized]
  }

  const themeSettings = computed(() => normalizeThemeSettings(publicSettings.value?.theme_settings))
  const visitorAuditSupported = computed(() => typeof publicSettings.value?.visitor_audit_enabled === 'boolean')
  const visitorAuditEnabled = computed(() => publicSettings.value?.visitor_audit_enabled === true)

  // 首页滚动位置记忆
  const homeScrollPosition = ref<number>(0)

  // 使用 null 表示未设置，等待主题配置加载后决定
  const storedViewMode = useStorageAsync<NodeViewMode | null>('nodeViewMode', null, localStorage)

  const beijingTimeTick = ref(Date.now())
  if (typeof window !== 'undefined') {
    window.setInterval(() => {
      beijingTimeTick.value = Date.now()
    }, 60 * 1000)
  }

  // 计算属性：从主题配置获取默认视图模式
  const derived = computed(() => deriveAppSettings(themeSettings.value))

  const defaultViewMode = computed(() => derived.value.defaultViewMode)

  // 校验视图模式是否为合法值
  function isValidViewMode(value: string | null): value is NodeViewMode {
    return value === 'card' || value === 'list'
  }

  const nodeCardSize = computed(() => derived.value.nodeCardSize)

  // 当前实际使用的视图模式
  const nodeViewMode = computed<NodeViewMode>({
    get: () => {
      // 校验 storedViewMode 是否为合法值，非法值时使用默认值
      if (storedViewMode.value !== null && isValidViewMode(storedViewMode.value)) {
        return storedViewMode.value
      }
      return defaultViewMode.value
    },
    set: (val) => {
      storedViewMode.value = val
    },
  })

  // 计算属性：从主题配置获取 RPC 连接模式
  const rpcTransportMode = computed(() => derived.value.rpcTransportMode)

  // 字节格式化精度（固定配置）
  const byteDecimals: ByteDecimalsConfig = { ...BYTE_DECIMALS }

  // 计算属性：公告配置
  const alertEnabled = computed(() => derived.value.alertEnabled)

  const alertTitle = computed(() => derived.value.alertTitle)

  const alertContent = computed(() => derived.value.alertContent)

  const dataUpdateInterval = computed(() => derived.value.dataUpdateInterval)

  const stopEarth = computed(() => derived.value.stopEarth)

  const earthRenderer = computed(() => derived.value.earthRenderer)

  const hideEarth = computed(() => derived.value.hideEarth)

  const hideGeneralCard = computed(() => derived.value.hideGeneralCard)

  const visitorInfoEnabled = computed(() => derived.value.visitorInfoEnabled)

  const generalCardEnabledMap = computed(() => derived.value.generalCardEnabledMap)

  const generalCardOrder = computed(() => derived.value.generalCardOrder)

  const homeToolsEnabled = computed(() => derived.value.homeToolsEnabled)

  const glassColorPreset = computed(() => derived.value.glassColorPreset)

  const glassCustomColors = computed(() => derived.value.glassCustomColors)

  const colorVisionMode = computed(() => derived.value.colorVisionMode)

  const colorVisionFriendly = computed(() => derived.value.colorVisionFriendly)

  const homeQuickControlsEnabled = computed(() => derived.value.homeQuickControlsEnabled)

  const homeQuickControlOrder = computed(() => derived.value.homeQuickControlOrder)

  const nodeListMetadataEnabled = computed(() => derived.value.nodeListMetadataEnabled)

  const nodeListMetadataFields = computed(() => derived.value.nodeListMetadataFields)

  const nodeListCustomTagsVisible = computed(() => derived.value.nodeListCustomTagsVisible)

  const nodeDetailSectionTabsEnabled = computed(() => derived.value.nodeDetailSectionTabsEnabled)

  const gpuChartEnabled = computed(() => derived.value.gpuChartEnabled)

  const detailMetricCardOrder = computed(() => derived.value.detailMetricCardOrder)

  const offlineNodesLast = computed(() => derived.value.offlineNodesLast)

  const homeHighLoadThreshold = computed(() => derived.value.homeHighLoadThreshold)

  const homeTrafficWarningThreshold = computed(() => derived.value.homeTrafficWarningThreshold)

  const homeExpiringDays = computed(() => derived.value.homeExpiringDays)

  const diskPredictionEnabled = computed(() => derived.value.diskPredictionEnabled)

  const diskPredictionThresholdDays = computed(() => derived.value.diskPredictionThresholdDays)

  const chartDashboardTemplate = computed(() => derived.value.chartDashboardTemplate)

  const hideAdminEntryWhenLoggedOut = computed(() => derived.value.hideAdminEntryWhenLoggedOut)

  const hidePriceWhenLoggedOut = computed(() => derived.value.hidePriceWhenLoggedOut)

  const providerAliases = computed(() => derived.value.providerAliases)

  const exportSecondaryPassword = computed(() => derived.value.exportSecondaryPassword)

  const disablePageAnimation = computed(() => derived.value.disablePageAnimation)

  // 计算属性：自定义背景配置
  const backgroundEnabled = computed(() => derived.value.backgroundEnabled)

  const backgroundType = computed(() => derived.value.backgroundType)

  const lightBackgroundUrl = computed(() => derived.value.lightBackgroundUrl)

  const darkBackgroundUrl = computed(() => derived.value.darkBackgroundUrl)

  const backgroundBlur = computed(() => derived.value.backgroundBlur)

  const backgroundOverlay = computed(() => derived.value.backgroundOverlay)

  // 当 publicSettings 加载后，如果 localStorage 没有保存过视图模式或值为非法值，使用默认值
  watch(publicSettings, (settings) => {
    if (settings && !isValidViewMode(storedViewMode.value)) {
      // 触发 computed setter，会自动保存到 localStorage
      storedViewMode.value = defaultViewMode.value
    }
  }, { immediate: true })

  watch(themeMode, (mode) => {
    if (!isValidThemeMode(mode)) {
      themeMode.value = 'auto'
    }
  }, { immediate: true })

  const managedThemeMode = computed(() => derived.value.managedThemeMode)

  const isBeijingDaytime = computed<boolean>(() => {
    const hour = getBeijingHour(beijingTimeTick.value)
    return hour >= 7 && hour < 19
  })

  // 计算当前是否为暗色模式。本机按钮选择 auto 时跟随后台托管设置；手动选择浅色/深色时仅覆盖当前浏览器。
  const isDark = computed(() => {
    const localMode = isValidThemeMode(themeMode.value) ? themeMode.value : 'auto'
    if (localMode === 'light')
      return false
    if (localMode === 'dark')
      return true

    if (managedThemeMode.value === 'beijing')
      return !isBeijingDaytime.value

    return managedThemeMode.value === 'dark'
  })

  const resolvedThemeMode = computed<'light' | 'dark'>(() => isDark.value ? 'dark' : 'light')

  // 计算属性：当前主题模式下的背景 URL
  const currentBackgroundUrl = computed<string>(() => {
    if (resolvedThemeMode.value === 'dark') {
      return darkBackgroundUrl.value
    }
    return lightBackgroundUrl.value
  })

  function updateThemeMode(mode?: ThemeMode) {
    if (mode) {
      themeMode.value = isValidThemeMode(mode) ? mode : 'auto'
      return
    }

    const nextMode: Record<ThemeMode, ThemeMode> = {
      auto: 'light',
      light: 'dark',
      dark: 'auto',
    }

    const currentMode = isValidThemeMode(themeMode.value) ? themeMode.value : 'auto'
    themeMode.value = nextMode[currentMode]
  }

  function syncAuthState() {
    const session = getAuthSession()
    isLoggedIn.value = session.authenticated
    authStatus.value = session.status
    return session
  }

  function updateLoginState(loggedIn: boolean, user?: MeInfo | null) {
    setAuthSessionFromLogin(loggedIn, user ?? null)
    syncAuthState()
  }

  async function verifyLoginState(options?: VerifyLoginOptions): Promise<boolean> {
    await verifyLogin(options)
    return syncAuthState().authenticated
  }

  async function requireLoginPermission(permission: PermissionKey, options?: VerifyLoginOptions): Promise<boolean> {
    const result = await requirePermission(permission, options)
    syncAuthState()
    return result.granted
  }

  return {
    loading,
    themeMode,
    managedThemeMode,
    isBeijingDaytime,
    isDark,
    resolvedThemeMode,
    lang,
    nodeSelectedGroup,
    favoriteNodeIds,
    favoriteNodeIdSet,
    isFavoriteNode,
    toggleFavoriteNode,
    nodeViewMode,
    defaultViewMode,
    nodeCardSize,
    rpcTransportMode,
    byteDecimals,
    alertEnabled,
    alertTitle,
    alertContent,
    dataUpdateInterval,
    stopEarth,
    earthRenderer,
    hideEarth,
    hideGeneralCard,
    visitorInfoEnabled,
    generalCardEnabledMap,
    generalCardOrder,
    homeToolsEnabled,
    homeAdvancedToolsVisible,
    glassColorPreset,
    glassCustomColors,
    colorVisionMode,
    colorVisionFriendly,
    visitorAuditSupported,
    visitorAuditEnabled,
    homeQuickControlsEnabled,
    homeQuickControlOrder,
    nodeListMetadataEnabled,
    nodeListMetadataFields,
    nodeListCustomTagsVisible,
    nodeDetailSectionTabsEnabled,
    gpuChartEnabled,
    detailMetricCardOrder,
    offlineNodesLast,
    homeHighLoadThreshold,
    homeTrafficWarningThreshold,
    homeExpiringDays,
    diskPredictionEnabled,
    diskPredictionThresholdDays,
    chartDashboardTemplate,
    hideAdminEntryWhenLoggedOut,
    hidePriceWhenLoggedOut,
    providerAliases,
    exportSecondaryPassword,
    disablePageAnimation,
    backgroundEnabled,
    backgroundType,
    lightBackgroundUrl,
    darkBackgroundUrl,
    currentBackgroundUrl,
    backgroundBlur,
    backgroundOverlay,
    isLoggedIn,
    authStatus,
    privateFeaturesAllowed,
    publicSettings,
    connectionError,
    homeScrollPosition,
    updateThemeMode,
    updateLoginState,
    verifyLoginState,
    requireLoginPermission,
  }
})

export { useAppStore }
