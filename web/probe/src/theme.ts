import { createTheme } from '@mantine/core'

// Status page: operator-selected preset and palette, with Mantine colour schemes.
export function buildTheme(primary = 'cyan', radius = 'lg', font?: string) {
  return createTheme({
  primaryColor: primary,
  primaryShade: 6,
  defaultRadius: radius,
  fontFamily: font || 'Inter, -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif',
  headings: { fontWeight: '700' },
  components: {
    Card: { defaultProps: { withBorder: true, padding: 'xl', radius } },
    Button: { defaultProps: { radius: 'md' } },
  },
  })
}

export const theme = buildTheme()
