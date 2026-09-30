export interface ProbeAppearance { preset: string; scheme: string }
export interface SiteTheme { primary?: string; radius?: string; site_scheme?: string; font_family?: string }

// Finite bundled presets. Unknown values fall back, never become CSS or imports.
export function resolveAppearance(appearance?: ProbeAppearance | null, site?: SiteTheme) {
 const preset = ['aurora', 'paper', 'terminal'].includes(appearance?.preset ?? '') ? appearance!.preset : 'inherit'
 const selected = appearance?.scheme === 'inherit' || !appearance?.scheme ? site?.site_scheme : appearance.scheme
 const scheme: 'auto' | 'light' | 'dark' = selected === 'auto' || selected === 'light' ? selected : 'dark'
 const primary = preset === 'aurora' ? 'violet' : preset === 'paper' ? 'blue' : preset === 'terminal' ? 'teal' : site?.primary || 'cyan'
 const radius = preset === 'paper' ? 'md' : preset === 'terminal' ? 'xs' : site?.radius || 'lg'
 const font = preset === 'terminal' ? 'ui-monospace, SFMono-Regular, Menlo, monospace' : site?.font_family
 return { preset, scheme, primary, radius, font }
}
