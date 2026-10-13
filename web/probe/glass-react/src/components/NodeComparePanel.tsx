import { Fragment, useEffect, useMemo, useState } from 'react'
import type { CSSProperties } from 'react'
import { Icon } from '@iconify/react'
import { t } from '@shared/i18n'
import { shows } from '@shared/captain'
import { formatBytesPerSecondWithConfig, formatBytesWithConfig, formatUptimeWithFormat } from '@shared/utils/helper'
import { getDiskPercentage, getMemoryPercentage, getTrafficUsed, getTrafficUsedPercentage, hasTrafficLimit } from '@shared/utils/nodeMetricsHelper'
import { isNodeMatchSearch } from '@shared/utils/nodeSearch'
import { formatPriceWithCycle, isFreePrice } from '@shared/utils/tagHelper'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { vclass } from '@/lib/vclass'
import { readStored, writeStored } from '@/stores/storage'
import { useApp } from '@/stores/app'
import type { NodeData } from '@/stores/nodes'

interface CompareMetric {
  key: string
  label: string
  value: (node: NodeData) => string
  percentage?: (node: NodeData) => number | null
}

const MAX_COMPARE_NODES = 4
const STORAGE_KEY = 'theme:node-compare:v1'

function clampPercentage(value: number): number {
  return Math.min(100, Math.max(0, Number.isFinite(value) ? value : 0))
}

export default function NodeComparePanel({ nodes }: { nodes: NodeData[] }) {
  const app = useApp(a => ({ byteDecimals: a.byteDecimals, privateFeaturesAllowed: a.privateFeaturesAllowed, hidePriceWhenLoggedOut: a.hidePriceWhenLoggedOut, lang: a.lang, favoriteNodeIdSet: a.favoriteNodeIdSet }))
  const [searchText, setSearchText] = useState('')
  // useStorageAsync('theme:node-compare:v1', [], localStorage): JSON array.
  const [selectedIds, setSelectedIdsState] = useState<unknown>(() => readStored<string[]>(STORAGE_KEY, [], 'json'))
  const setSelectedIds = (ids: string[]) => {
    setSelectedIdsState(ids)
    writeStored(STORAGE_KEY, ids, 'json')
  }
  const normalizedSelectedIds = useMemo(() => [...new Set(
    (Array.isArray(selectedIds) ? selectedIds : [])
      .filter((id): id is string => typeof id === 'string' && Boolean(id.trim())),
  )].slice(0, MAX_COMPARE_NODES), [selectedIds])

  const filteredNodes = useMemo(() => {
    const query = searchText.trim()
    return query ? nodes.filter(node => isNodeMatchSearch(node, query)) : nodes
  }, [nodes, searchText])
  const selectedNodes = normalizedSelectedIds
    .map(uuid => nodes.find(node => node.uuid === uuid))
    .filter((node): node is NodeData => Boolean(node))
  const comparisonGridStyle: CSSProperties = { gridTemplateColumns: `7.5rem repeat(${Math.max(1, selectedNodes.length)}, minmax(11rem, 1fr))` }

  const formatBytes = (bytes: number) => formatBytesWithConfig(bytes, app.byteDecimals)
  const formatSpeed = (bytes: number) => formatBytesPerSecondWithConfig(bytes, app.byteDecimals)

  const metrics: CompareMetric[] = [
    { key: 'status', label: t('状态'), value: node => node.online ? t('在线') : t('离线') },
    { key: 'cpuModel', label: 'CPU 型号', value: node => node.cpu_name || '-' },
    { key: 'cpuCores', label: 'vCPU', value: node => `${node.cpu_cores || 0} 核` },
    { key: 'cpu', label: 'CPU 使用率', value: node => `${(node.cpu || 0).toFixed(1)}%`, percentage: node => node.cpu || 0 },
    { key: 'load', label: '系统负载', value: node => `${(node.load || 0).toFixed(2)} / ${(node.load5 || 0).toFixed(2)} / ${(node.load15 || 0).toFixed(2)}` },
    { key: 'memory', label: t('内存'), value: node => `${formatBytes(node.ram || 0)} / ${formatBytes(node.mem_total || 0)}`, percentage: getMemoryPercentage },
    { key: 'disk', label: t('磁盘'), value: node => `${formatBytes(node.disk || 0)} / ${formatBytes(node.disk_total || 0)}`, percentage: getDiskPercentage },
    { key: 'network', label: t('实时网络'), value: node => `↑ ${formatSpeed(node.net_out || 0)}  ↓ ${formatSpeed(node.net_in || 0)}` },
    { key: 'traffic', label: t('累计流量'), value: node => `${formatBytes(node.net_total_up ?? Number.NaN)} ↑ / ${formatBytes(node.net_total_down ?? Number.NaN)} ↓` },
    {
      key: 'trafficQuota',
      label: t('流量配额'),
      value: node => hasTrafficLimit(node) ? `${formatBytes(getTrafficUsed(node))} / ${formatBytes(node.traffic_limit)}` : '无限',
      percentage: node => hasTrafficLimit(node) ? getTrafficUsedPercentage(node) : null,
    },
    { key: 'uptime', label: t('运行时间'), value: node => formatUptimeWithFormat(node.uptime || 0, 'day') },
  ]
  if (app.privateFeaturesAllowed || !app.hidePriceWhenLoggedOut) {
    metrics.push({
      key: 'price',
      label: '价格',
      value: node => node.price > 0 || isFreePrice(node.price) ? formatPriceWithCycle(node.price, node.billing_cycle, node.currency, app.lang) : '-',
    })
  }
  const compareMetrics = metrics.filter(metric => !['traffic', 'trafficQuota'].includes(metric.key) || shows('traffic'))

  const isSelected = (uuid: string) => normalizedSelectedIds.includes(uuid)
  function toggleNode(uuid: string): void {
    if (isSelected(uuid)) {
      setSelectedIds(normalizedSelectedIds.filter(id => id !== uuid))
      return
    }
    if (normalizedSelectedIds.length >= MAX_COMPARE_NODES) {
      window.$message?.warning(`最多同时对比 ${MAX_COMPARE_NODES} 台节点。`)
      return
    }
    setSelectedIds([...normalizedSelectedIds, uuid])
  }

  // watch(node uuids, …, { immediate: true }): drop selections that vanished.
  const uuidKey = nodes.map(node => node.uuid).join('|')
  useEffect(() => {
    const available = new Set(nodes.map(node => node.uuid))
    const validSelected = normalizedSelectedIds.filter(uuid => available.has(uuid))
    if (validSelected.join('|') !== normalizedSelectedIds.join('|') || !Array.isArray(selectedIds))
      setSelectedIds(validSelected)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [uuidKey])

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-52 flex-1 sm:max-w-80">
          <Input value={searchText} onValueChange={v => setSearchText(String(v))} placeholder="搜索节点、IP 或 CPU" className="h-8 bg-background/55 pl-8" />
          <Icon icon="tabler:search" width="14" height="14" className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground" />
        </div>
        <span className="text-xs tabular-nums text-muted-foreground">{`已选 ${selectedNodes.length} / ${MAX_COMPARE_NODES}`}</span>
        <Button variant="ghost" size="sm" disabled={!selectedNodes.length} onClick={() => setSelectedIds([])}>
          <Icon icon="tabler:x" width="14" height="14" />
          {' 清空 '}
        </Button>
      </div>

      <div className="flex max-h-32 flex-wrap gap-1.5 overflow-y-auto">
        {filteredNodes.map(node => (
          <button
            key={node.uuid}
            type="button"
            className={vclass('inline-flex h-7 items-center gap-1.5 rounded-md bg-background/45 px-2 text-xs text-muted-foreground transition-colors hover:bg-background/70 hover:text-foreground', isSelected(node.uuid) && 'bg-background text-selection shadow-sm')}
            aria-pressed={isSelected(node.uuid)}
            onClick={() => toggleNode(node.uuid)}
          >
            <span className={vclass('size-1.5 rounded-full', node.online ? 'bg-success' : 'bg-destructive')} />
            {app.favoriteNodeIdSet.has(node.uuid) && <Icon icon="tabler:star-filled" width="11" height="11" className="text-amber-500" />}
            <span className="max-w-38 truncate">{node.name}</span>
          </button>
        ))}
      </div>

      {selectedNodes.length
        ? (
            <div className="overflow-x-auto rounded-md bg-background/35 p-1">
              <div className="grid min-w-max gap-px overflow-hidden rounded-sm" style={comparisonGridStyle}>
                <div className="bg-background/65 px-3 py-2 text-xs font-semibold text-muted-foreground">{' 实时快照 '}</div>
                {selectedNodes.map(node => (
                  <div key={`header-${node.uuid}`} className="min-w-0 bg-background/65 px-3 py-2">
                    <div className="flex items-center gap-1.5">
                      <span className={vclass('size-2 rounded-full', node.online ? 'bg-success' : 'bg-destructive')} />
                      <span className="truncate text-sm font-semibold">{node.name}</span>
                    </div>
                  </div>
                ))}
                {compareMetrics.map(metric => (
                  <Fragment key={metric.key}>
                    <div className="bg-background/45 px-3 py-2 text-xs font-medium text-muted-foreground">{metric.label}</div>
                    {selectedNodes.map((node) => {
                      const percentage = metric.percentage?.(node)
                      return (
                        <div key={`${metric.key}-${node.uuid}`} className="min-w-0 bg-background/45 px-3 py-2">
                          <div className="break-words text-xs tabular-nums" title={metric.value(node)}>{metric.value(node)}</div>
                          {percentage != null && (
                            <div className="mt-1.5 h-1 overflow-hidden rounded-full bg-slate-500/10">
                              <div className="h-full rounded-full bg-selection/70" style={{ width: `${clampPercentage(percentage ?? 0)}%` }} />
                            </div>
                          )}
                        </div>
                      )
                    })}
                  </Fragment>
                ))}
              </div>
            </div>
          )
        : (
            <div className="space-y-3 py-7 text-center text-sm text-muted-foreground">
              <p>选择 2 至 4 台节点进行横向对比</p>
              <div className="mx-auto flex max-w-3xl flex-wrap justify-center gap-1.5">
                {compareMetrics.map(metric => (
                  <span key={`preview-${metric.key}`} className="rounded-sm bg-background/45 px-2 py-1 text-xs">{metric.label}</span>
                ))}
              </div>
            </div>
          )}
    </div>
  )
}
