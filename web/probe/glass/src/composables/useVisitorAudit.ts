import { computed } from 'vue'
import type { VisitorAuditEventParams } from '@/utils/rpc'
export function useVisitorAudit() { return { supported: computed(() => false), enabled: computed(() => false), record: async (_event: VisitorAuditEventParams, _security = false) => {} } }
export function useVisitorPageAudit() {}
