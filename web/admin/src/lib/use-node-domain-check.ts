import { useDebouncedValue } from '@mantine/hooks'
import { useQuery } from '@tanstack/react-query'
import { api } from './api'

export interface DomainCheck {
  domain: string
  valid: boolean
  unchanged: boolean
  conflicts: { id: number; name: string; shared: boolean }[]
}

export function useNodeDomainCheck(domain: string, shared: boolean, nodeID: number | undefined, enabled: boolean) {
  const [debounced] = useDebouncedValue(domain, 250)
  const query = useQuery({
    queryKey: ['node-domain-check', debounced, nodeID],
    queryFn: ({ signal }) => api.get<DomainCheck>(`/api/admin/nodes/domain-check?domain=${encodeURIComponent(debounced)}${nodeID ? `&exclude_id=${nodeID}` : ''}`, signal),
    enabled, staleTime: 0, retry: false,
  })
  // Never display a previous input's result while its debounce is pending.
  const data = debounced === domain ? query.data : undefined
  const blocked = enabled && (domain !== debounced || query.isFetching || !!data && (!data.valid || data.conflicts.length > 0 && !shared && !data.unchanged))
  return { data, blocked, unavailable: query.isError && domain === debounced }
}

export type NodeDomainCheck = ReturnType<typeof useNodeDomainCheck>
