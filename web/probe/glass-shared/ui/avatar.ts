// Variant definitions shared by the Vue and React builds; class strings
// are the theme's own and must stay byte-identical on both sides.
import type { VariantProps } from 'class-variance-authority'
import { cva } from 'class-variance-authority'


export const avatarVariants = cva(
  'size-8 rounded-full data-[size=lg]:size-10 data-[size=sm]:size-6 group/avatar relative flex shrink-0 select-none overflow-hidden',
  {
    variants: {
      size: {
        sm: '',
        default: '',
        lg: '',
      },
    },
    defaultVariants: {
      size: 'default',
    },
  },
)

export type AvatarVariants = VariantProps<typeof avatarVariants>
