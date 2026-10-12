import '@/scoped/traffic-progress.css'
import { scope } from '@/scoped/scope'
import { formatBytes } from '@shared/utils/helper'

const s = scope('traffic-progress')

export interface TrafficProgressProps {
  upload: number
  download: number
  trafficLimit: number
  trafficLimitType: 'up' | 'down' | 'min' | 'max' | 'sum'
  uploadColor?: string
  downloadColor?: string
  singleColor?: string
  height?: number | string
  showIndicator?: boolean
}

export default function TrafficProgress({ upload, download, trafficLimit, trafficLimitType, height, showIndicator = false }: TrafficProgressProps) {
  const showProgress = trafficLimit > 0
  let usedTraffic: number
  switch (trafficLimitType) {
    case 'up': usedTraffic = upload; break
    case 'down': usedTraffic = download; break
    case 'min': usedTraffic = Math.min(upload, download); break
    case 'max': usedTraffic = Math.max(upload, download); break
    default: usedTraffic = upload + download
  }
  const totalPercentage = trafficLimit <= 0 ? 0 : Math.min((usedTraffic / trafficLimit) * 100, 100)
  const uploadPercentage = trafficLimit <= 0 ? 0 : Math.min((upload / trafficLimit) * 100, 100)
  const downloadPercentage = trafficLimit <= 0 ? 0 : Math.min((download / trafficLimit) * 100, 100)
  const isDualColorMode = trafficLimitType === 'sum'
  const progressHeight = height === undefined ? undefined : typeof height === 'number' ? `${height}px` : height

  return (
    <div {...s} className="traffic-progress">
      {isDualColorMode
        ? (
            <div {...s} className="traffic-progress__rail bg-muted" style={{ height: progressHeight }}>
              <div {...s} className="traffic-progress__fill bg-green-600" style={{ width: `${uploadPercentage}%` }} />
              <div {...s} className="traffic-progress__fill traffic-progress__fill--last bg-blue-600" style={{ width: `${downloadPercentage}%` }} />
            </div>
          )
        : (
            <div {...s} className="traffic-progress__rail bg-muted" style={{ height: progressHeight }}>
              <div {...s} className="traffic-progress__fill traffic-progress__fill--last bg-green-600" style={{ width: `${totalPercentage}%` }} />
            </div>
          )}
      {showIndicator && showProgress && (
        <div {...s} className="traffic-progress__indicator">
          <span {...s}>{`${totalPercentage.toFixed(1)}%`}</span>
          <span {...s} className="traffic-progress__indicator-detail">
            {`${formatBytes(usedTraffic)} / ${formatBytes(trafficLimit)}`}
          </span>
        </div>
      )}
    </div>
  )
}
