import { assetTypeMeta, statusLabel, statusTone, type StatusTone } from '../../api/assetTypes'

const TONE: Record<StatusTone, string> = {
  success: 'bg-success-soft text-success',
  warning: 'bg-warning-soft text-warning',
  danger: 'bg-danger-soft text-danger',
  neutral: 'bg-neutral-soft text-text-muted',
}

/** Icon + label for an asset type; `compact` shows the short code (SUB, TX, ...). */
export function TypeBadge({ type, compact = false }: { type: string; compact?: boolean }) {
  const meta = assetTypeMeta(type)
  const Icon = meta.icon
  return (
    <span
      title={meta.label}
      className="inline-flex shrink-0 items-center gap-1 rounded-sm bg-neutral-soft px-2 py-1 text-[11px] font-semibold tracking-wide text-text-muted uppercase"
    >
      <Icon aria-hidden className="size-3" />
      {compact ? meta.short : meta.label}
      {compact && <span className="sr-only"> ({meta.label})</span>}
    </span>
  )
}

export function StatusBadge({ status }: { status: string }) {
  return (
    <span
      className={`inline-flex shrink-0 items-center gap-2 rounded-full px-2 py-1 text-xs font-semibold whitespace-nowrap ${TONE[statusTone(status)]}`}
    >
      <span aria-hidden className="size-1.5 rounded-full bg-current" />
      {statusLabel(status)}
    </span>
  )
}

/** Dot-only status marker for dense rows; the status text is for screen readers. */
export function StatusDot({ status }: { status: string }) {
  const tone = statusTone(status)
  const colour =
    tone === 'success'
      ? 'bg-success'
      : tone === 'warning'
        ? 'bg-warning'
        : tone === 'danger'
          ? 'bg-danger'
          : 'bg-text-muted'
  return (
    <>
      <span aria-hidden className={`size-2 shrink-0 rounded-full ${colour}`} />
      <span className="sr-only">{statusLabel(status)}</span>
    </>
  )
}
