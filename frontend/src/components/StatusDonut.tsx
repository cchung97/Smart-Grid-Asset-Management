import { Circle, CircleCheck, CircleX, TriangleAlert, type LucideIcon } from 'lucide-react'
import { useState } from 'react'
import { statusLabel, statusTone, type StatusTone } from '../api/assetTypes'
import type { StatsResponse } from '../api/types'

// Status colours are reserved for meaning (in service / maintenance / out of service), so the marks wear the
// status tokens. Amber and red are hard to tell apart for some colour-blind viewers, so colour never carries
// identity alone: every segment has an icon and a text label in the legend, a gap between segments, and a hover.
const TONE: Record<StatusTone, { stroke: string; text: string; Icon: LucideIcon }> = {
  success: { stroke: 'stroke-success', text: 'text-success', Icon: CircleCheck },
  warning: { stroke: 'stroke-warning', text: 'text-warning', Icon: TriangleAlert },
  danger: { stroke: 'stroke-danger', text: 'text-danger', Icon: CircleX },
  neutral: { stroke: 'stroke-text-muted', text: 'text-text-muted', Icon: Circle },
}

const SIZE = 140 // viewBox units
const RADIUS = 56
const RING = 20 // ring thickness: thin enough that the data, not the block, is what you see
const CIRCUMFERENCE = 2 * Math.PI * RADIUS
const GAP = 2 // surface-coloured gap between touching segments

/**
 * Share of all assets in each operational status: a donut for the at-a-glance split (three segments,
 * part of a whole) with a legend that carries the exact counts. The legend matters because close values
 * (37 and 38 assets) cannot be compared by ring length; it is also the text/table version of the chart.
 */
export function StatusDonut({ stats, className = '' }: { stats: StatsResponse; className?: string }) {
  const [active, setActive] = useState<string | null>(null)
  const total = stats.total
  const rows = stats.by_status
    .filter((s) => s.count > 0)
    .map((s) => ({
      key: s.operational_status,
      label: statusLabel(s.operational_status),
      count: s.count,
      pct: total ? Math.round((s.count / total) * 100) : 0,
      tone: TONE[statusTone(s.operational_status)],
    }))

  const arcs = rows.map((r, i) => {
    const share = total ? (r.count / total) * CIRCUMFERENCE : 0
    const offset = total ? (rows.slice(0, i).reduce((n, x) => n + x.count, 0) / total) * CIRCUMFERENCE : 0 // where the previous segments end
    const length = rows.length > 1 ? Math.max(0, share - GAP) : share // a lone segment is the whole ring
    return { ...r, length, offset }
  })

  const hovered = rows.find((r) => r.key === active)
  const summary = rows.map((r) => `${r.label} ${r.count} (${r.pct}%)`).join(', ')

  return (
    <section aria-labelledby="status-title" className={className}>
      <h2 id="status-title" className="mb-3 text-xs font-semibold tracking-wider text-text-muted uppercase">
        Operational status
      </h2>
      <div className="rounded-lg border border-border bg-surface p-6">
        <div className="relative mx-auto size-40">
          <svg viewBox={`0 0 ${SIZE} ${SIZE}`} role="img" aria-label={`Assets by operational status: ${summary}`} className="size-full">
            <g transform={`rotate(-90 ${SIZE / 2} ${SIZE / 2})`} fill="none" strokeWidth={RING}>
              {arcs.map((a) => (
                <circle
                  key={a.key}
                  data-status={a.key}
                  cx={SIZE / 2}
                  cy={SIZE / 2}
                  r={RADIUS}
                  className={`${a.tone.stroke} transition-opacity ${active && active !== a.key ? 'opacity-30' : ''}`}
                  strokeDasharray={`${a.length} ${CIRCUMFERENCE - a.length}`}
                  strokeDashoffset={-a.offset}
                  onMouseEnter={() => setActive(a.key)}
                  onMouseLeave={() => setActive(null)}
                >
                  <title>{`${a.label}: ${a.count} of ${total} assets (${a.pct}%)`}</title>
                </circle>
              ))}
            </g>
          </svg>
          {/* The centre reads the total, or the hovered segment's own count. */}
          <div aria-hidden className="pointer-events-none absolute inset-0 grid place-content-center text-center">
            <div className="text-3xl font-bold">{hovered ? hovered.count : total}</div>
            <div className="text-xs text-text-muted">{hovered ? hovered.label : 'Assets'}</div>
          </div>
        </div>

        <ul className="mt-6 space-y-1">
          {rows.map((r) => (
            <li
              key={r.key}
              onMouseEnter={() => setActive(r.key)}
              onMouseLeave={() => setActive(null)}
              className={`flex items-center justify-between gap-3 rounded-md px-2 py-2 text-sm ${active === r.key ? 'bg-surface-muted' : ''}`}
            >
              <span className="inline-flex items-center gap-2">
                <r.tone.Icon aria-hidden className={`size-4 ${r.tone.text}`} />
                <span className="font-medium">{r.label}</span>
              </span>
              <span className="text-text-muted tabular-nums">
                {r.count} · {r.pct}%
              </span>
            </li>
          ))}
        </ul>
      </div>
    </section>
  )
}
