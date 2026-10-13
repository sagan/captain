import { scopes } from './scopes'

export type ScopeName = keyof typeof scopes

/** Props that mark an element as belonging to a scoped component, like Vue's data-v attribute. */
export function scope(name: ScopeName): Record<string, string> {
  return { [scopes[name].attr]: '' }
}
