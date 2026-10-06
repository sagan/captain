type Settings = Record<string, unknown>
type Target = { server_name: string; settings?: Settings }

function object(value: unknown): Settings | undefined {
  return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Settings : undefined
}

// The tunnel and REALITY user listener share one target in the managed
// connection API. Keep the advanced draft consistent with that contract.
export function withReverseTarget<T extends Target>(value: T, serverName: string): T {
  const tls = object(value.settings?.tls)
  const settings = tls?.mode === 2 ? {
    ...value.settings,
    tls: { ...tls, server_name: serverName, reality: { ...object(tls.reality), handshake_server: serverName, handshake_port: 443 } },
  } : value.settings
  return { ...value, server_name: serverName, settings }
}

export function withReverseProtocol<T extends Target>(value: T, settings: Settings): T {
  const tls = object(settings.tls)
  const target = tls?.mode === 2 && typeof tls.server_name === 'string' ? tls.server_name.trim() : value.server_name
  return withReverseTarget({ ...value, settings }, target)
}
