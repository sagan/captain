export interface AvailabilitySummary {
 online_seconds: number; offline_seconds: number; unknown_seconds: number; maintenance_seconds: number
 percent: number | null; coverage_percent: number | null
}
export interface Availability extends AvailabilitySummary {
 from: number; to: number; buckets: (AvailabilitySummary & { from: number; to: number })[]
}
