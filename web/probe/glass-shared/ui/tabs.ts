// Variant definitions shared by the Vue and React builds; class strings
// are the theme's own and must stay byte-identical on both sides.
import type { VariantProps } from 'class-variance-authority'
import { cva } from 'class-variance-authority'


export const tabsListVariants = cva(
  'rounded-lg p-[3px] group-data-horizontal/tabs:h-9 group-data-vertical/tabs:rounded-lg data-[variant=line]:rounded-none group/tabs-list inline-flex w-fit items-center justify-center text-muted-foreground group-data-vertical/tabs:h-fit group-data-vertical/tabs:flex-col',
  {
    variants: {
      variant: {
        default: 'bg-muted',
        line: 'gap-1 bg-transparent',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  },
)

export type TabsListVariants = VariantProps<typeof tabsListVariants>
