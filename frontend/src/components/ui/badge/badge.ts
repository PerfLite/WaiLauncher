import { cva, type VariantProps } from 'class-variance-authority'

export const badgeVariants = cva(
  'inline-flex items-center rounded-md border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2',
  {
    variants: {
      variant: {
        default:
          'border-transparent bg-emerald-600/20 text-emerald-400 border border-emerald-500/30',
        secondary:
          'border-transparent bg-neutral-800 text-neutral-300',
        destructive:
          'border-transparent bg-rose-500/20 text-rose-400 border border-rose-500/30',
        outline: 'text-neutral-300 border-neutral-700',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  }
)

export type BadgeVariants = VariantProps<typeof badgeVariants>
