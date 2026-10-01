import { useSearchParams } from 'react-router-dom'

// Only public navigation choices belong in the URL; never form values or secrets.
export function useURLChoice(key: string, choices: readonly string[], fallback: string) {
  const [params, setParams] = useSearchParams()
  const requested = params.get(key) ?? fallback
  const value = choices.includes(requested) ? requested : fallback
  const setValue = (next: string) => setParams(previous => {
    const result = new URLSearchParams(previous)
    if (next === fallback) result.delete(key)
    else result.set(key, choices.includes(next) ? next : fallback)
    return result
  })
  return [value, setValue] as const
}
