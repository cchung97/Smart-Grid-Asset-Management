import { ChevronDown } from 'lucide-react'
import type { ComponentProps } from 'react'

/**
 * A native <select> with its own chevron, inset from the edge. The browser's
 * built-in arrow sits flush against the border and cannot be padded.
 */
export function Select({ className = '', ...props }: ComponentProps<'select'>) {
  return (
    <div className="relative">
      <select
        {...props}
        className={`h-9 w-full appearance-none rounded-md border border-border-strong bg-surface pr-8 pl-3 text-sm text-text ${className}`}
      />
      <ChevronDown aria-hidden className="pointer-events-none absolute top-1/2 right-3 size-4 -translate-y-1/2 text-text-muted" />
    </div>
  )
}
