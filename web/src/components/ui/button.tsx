import * as React from 'react'
import { Slot } from '@radix-ui/react-slot'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'
const buttonVariants = cva('inline-flex items-center justify-center rounded-md text-sm font-medium transition-colors focus-visible:outline-2 disabled:pointer-events-none disabled:opacity-50', { variants: { variant: { default: 'bg-slate-900 text-white hover:bg-slate-700', outline: 'border border-slate-300 bg-white hover:bg-slate-100' }, size: { default: 'h-9 px-4', sm: 'h-8 px-3' } }, defaultVariants: { variant: 'default', size: 'default' } })
function Button({ className, variant, size, asChild = false, ...props }: React.ComponentProps<'button'> & VariantProps<typeof buttonVariants> & { asChild?: boolean }) { const Comp = asChild ? Slot : 'button'; return <Comp className={cn(buttonVariants({ variant, size }), className)} {...props} /> }
export { Button, buttonVariants }
