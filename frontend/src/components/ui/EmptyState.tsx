import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'

interface Props {
  icon: LucideIcon
  title: string
  description?: string
  children?: ReactNode
  className?: string
}

export function EmptyState({ icon: Icon, title, description, children, className = '' }: Props) {
  return (
    <div className={`flex flex-col items-center justify-center px-6 py-8 text-center ${className}`}>
      <Icon aria-hidden className="mb-4 size-10 text-border-strong" strokeWidth={1.5} />
      <h2 className="text-base font-semibold text-text">{title}</h2>
      {description && <p className="mt-1 max-w-sm text-sm text-text-muted">{description}</p>}
      {children && <div className="mt-4">{children}</div>}
    </div>
  )
}
