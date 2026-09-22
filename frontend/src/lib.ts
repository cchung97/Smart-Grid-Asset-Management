import { sortTypes, statusLabel, typeRank } from './api/assetTypes'
import type { AssetNode, ParentRule, Rejection } from './api/types'

/** Format a byte count for humans: 512 B, 3.4 KB, 1.2 MB. */
export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / (1024 * 1024)).toFixed(1)} MB`
}

/**
 * Spreadsheet programs treat a cell starting with = + - @ (or a tab/CR) as a
 * formula, and asset ids come from an uploaded file: prefix such cells with an
 * apostrophe so a downloaded report cannot run one.
 */
function csvCell(value: string): string {
  const safe = /^[=+\-@\t\r]/.test(value) ? `'${value}` : value
  return /[",\n\r]/.test(safe) ? `"${safe.replace(/"/g, '""')}"` : safe
}

/** The rejection report as CSV text (row, asset_id, reason). */
export function rejectionsToCsv(rejections: Rejection[]): string {
  const lines = ['row,asset_id,reason']
  for (const r of rejections) lines.push([String(r.csv_row), csvCell(r.asset_id), csvCell(r.reason)].join(','))
  return lines.join('\r\n') + '\r\n'
}

/** Hand `text` to the browser as a file download. */
export function downloadText(filename: string, text: string, mime = 'text/csv;charset=utf-8'): void {
  const url = URL.createObjectURL(new Blob([text], { type: mime }))
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

/** "grid_assets.csv" -> "grid_assets-rejections.csv" */
export function reportFilename(source: string): string {
  return `${source.replace(/\.csv$/i, '')}-rejections.csv`
}

/** Local, readable date-time for an ISO timestamp. */
export function formatDateTime(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString('en-GB', { dateStyle: 'medium', timeStyle: 'short' })
}

export type ChildSortKey = 'name' | 'type' | 'status' | 'rating' | 'commissioned'
export type SortDir = 'asc' | 'desc'
export const CHILD_SORT_KEYS: readonly ChildSortKey[] = ['name', 'type', 'status', 'rating', 'commissioned']

const collator = new Intl.Collator('en', { numeric: true, sensitivity: 'base' })

/**
 * Sort child assets by one column. Assets with no rating / commissioned date go
 * last in either direction (a blank is not "smallest"); ties fall back to
 * hierarchy order of type, then asset id, so the order is always stable.
 * Returns a new array.
 */
export function sortChildren(rows: readonly AssetNode[], key: ChildSortKey, dir: SortDir): AssetNode[] {
  const sign = dir === 'asc' ? 1 : -1
  const byKey = (a: AssetNode, b: AssetNode): number => {
    switch (key) {
      case 'name':
        return collator.compare(a.asset_name, b.asset_name)
      case 'type':
        return typeRank(a.asset_type) - typeRank(b.asset_type) || collator.compare(a.asset_type, b.asset_type)
      case 'status':
        return collator.compare(statusLabel(a.operational_status), statusLabel(b.operational_status))
      case 'rating':
        return (a.rating_kva ?? 0) - (b.rating_kva ?? 0)
      case 'commissioned':
        return (a.commissioned_date ?? '').localeCompare(b.commissioned_date ?? '')
    }
  }
  const missing = (n: AssetNode) => (key === 'rating' ? n.rating_kva == null : key === 'commissioned' && n.commissioned_date == null)
  return [...rows].sort((a, b) => {
    const ma = missing(a)
    const mb = missing(b)
    if (ma !== mb) return ma ? 1 : -1
    return (
      sign * (ma ? 0 : byKey(a, b)) ||
      typeRank(a.asset_type) - typeRank(b.asset_type) ||
      collator.compare(a.asset_id, b.asset_id)
    )
  })
}

/**
 * Every asset type that may sit anywhere beneath `assetType`, in hierarchy order, read from the
 * backend's parent rules (child type -> permitted parent type) and followed transitively. Nothing
 * here names a type: with the seed data a substation yields transformer, LV board, switchboard and
 * switchboard panel, a switchboard yields switchboard panel, and a panel yields nothing. Used to
 * show a count (including zero) for each of them.
 */
export function descendantTypes(assetType: string, rules: readonly ParentRule[]): string[] {
  const found = new Set<string>()
  const pending = [assetType]
  for (let parent = pending.pop(); parent !== undefined; parent = pending.pop()) {
    for (const rule of rules) {
      if (rule.parent_type === parent && !found.has(rule.child_type)) {
        found.add(rule.child_type)
        pending.push(rule.child_type)
      }
    }
  }
  return sortTypes([...found], (t) => t)
}

const pad = (n: number) => String(n).padStart(2, '0')

/** "2026-09-21 03:09:19 UTC" in the browser's own time zone. */
export function formatClock(d: Date): string {
  const date = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
  const time = `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  const zone = new Intl.DateTimeFormat(undefined, { timeZoneName: 'short' })
    .formatToParts(d)
    .find((p) => p.type === 'timeZoneName')?.value
  return `${date} ${time}${zone ? ` ${zone}` : ''}`
}
