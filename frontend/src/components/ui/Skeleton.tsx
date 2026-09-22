/** Pulsing placeholder block. Decorative — wrap groups in <SkeletonRegion>. */
export function Skeleton({ className = '' }: { className?: string }) {
  return <div aria-hidden className={`animate-pulse rounded-md bg-neutral-soft ${className}`} />
}

/** Announces "Loading <label>" once for the whole group of skeleton blocks. */
export function SkeletonRegion({
  label,
  className = '',
  children,
}: {
  label: string
  className?: string
  children: React.ReactNode
}) {
  return (
    <div role="status" aria-busy="true" className={className}>
      <span className="sr-only">Loading {label}…</span>
      {children}
    </div>
  )
}
