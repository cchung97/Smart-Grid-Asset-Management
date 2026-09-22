import { CircleAlert, RefreshCw } from 'lucide-react'
import { errorMessage } from '../../api/client'

interface Props {
  title?: string
  error?: unknown
  onRetry?: () => void
  className?: string
}

export function ErrorState({ title = 'Something went wrong', error, onRetry, className = '' }: Props) {
  return (
    <div
      role="alert"
      className={`flex flex-col items-center justify-center px-6 py-8 text-center ${className}`}
    >
      <CircleAlert aria-hidden className="mb-3 size-9 text-danger" strokeWidth={1.5} />
      <h2 className="text-base font-semibold text-text">{title}</h2>
      {error !== undefined && (
        <p className="mt-1 max-w-sm text-sm text-text-muted">{errorMessage(error)}</p>
      )}
      {onRetry && (
        <button
          type="button"
          onClick={onRetry}
          className="mt-4 inline-flex items-center gap-2 rounded-md border border-border-strong bg-surface px-3 py-2 text-sm font-medium text-text hover:bg-surface-muted"
        >
          <RefreshCw aria-hidden className="size-4" />
          Try again
        </button>
      )}
    </div>
  )
}
