import { cva, type VariantProps } from 'class-variance-authority'

export const buttonVariants = cva(
  'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-lg text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-emerald-500 disabled:pointer-events-none disabled:opacity-50 cursor-pointer select-none',
  {
    variants: {
      variant: {
        default:
          'bg-emerald-600 text-white shadow hover:bg-emerald-500 active:bg-emerald-700',
        destructive:
          'bg-rose-600 text-white shadow-sm hover:bg-rose-500 active:bg-rose-700',
        outline:
          'border border-neutral-700 bg-transparent shadow-sm hover:bg-neutral-800 hover:text-white',
        secondary:
          'bg-neutral-800 text-neutral-200 shadow-sm hover:bg-neutral-700 hover:text-white',
        ghost:
          'hover:bg-neutral-800 hover:text-white',
        link:
          'text-emerald-400 underline-offset-4 hover:underline',
      },
      size: {
        default: 'h-9 px-4 py-2',
        sm: 'h-8 rounded-md px-3 text-xs',
        lg: 'h-10 rounded-md px-8',
        icon: 'h-9 w-9',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  }
)

export type ButtonVariants = VariantProps<typeof buttonVariants>
