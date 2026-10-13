import type { VisitorAuditEventParams } from '@shared/utils/rpc'

// Captain has no visitor audit; same inert stub as the Vue build.
const audit = {
  supported: false,
  enabled: false,
  record: async (_event: VisitorAuditEventParams, _security = false) => {},
}
export function useVisitorAudit() { return audit }
export function useVisitorPageAudit() {}
