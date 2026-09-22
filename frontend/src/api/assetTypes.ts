import { Box, Building2, Columns3, Rows3, SquarePower, Zap, type LucideIcon } from 'lucide-react'

export interface AssetTypeMeta {
  label: string
  plural: string
  short: string
  icon: LucideIcon
}

// One icon + short code per asset type, reused by the tree, badges and the
// details panel so an asset looks the same everywhere. The backend's types are
// data (lookup table), so an unknown code gets a derived label and Box icon
// instead of breaking.
const KNOWN: Record<string, AssetTypeMeta> = {
  SUBSTATION: { label: 'Substation', plural: 'Substations', short: 'SUB', icon: Building2 },
  TRANSFORMER: { label: 'Transformer', plural: 'Transformers', short: 'TX', icon: Zap },
  LV_BOARD: { label: 'LV Board', plural: 'LV Boards', short: 'LV', icon: Rows3 },
  SWITCHBOARD: { label: 'Switchboard', plural: 'Switchboards', short: 'SWB', icon: Columns3 },
  SWITCHBOARD_PANEL: {
    label: 'Switchboard Panel',
    plural: 'Switchboard Panels',
    short: 'PNL',
    icon: SquarePower,
  },
}

const titleCase = (code: string) =>
  code
    .toLowerCase()
    .split('_')
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(' ')

export function assetTypeMeta(code: string): AssetTypeMeta {
  return (
    KNOWN[code] ?? {
      label: titleCase(code),
      plural: `${titleCase(code)}s`,
      short: code.replace(/_/g, '').slice(0, 3),
      icon: Box,
    }
  )
}

/** "IN_SERVICE" -> "In service". Statuses are lookup data, so an unknown code is made readable the same way. */
export function statusLabel(code: string): string {
  const words = code.toLowerCase().replace(/_/g, ' ').trim()
  return words.charAt(0).toUpperCase() + words.slice(1)
}

export type StatusTone = 'success' | 'warning' | 'danger' | 'neutral'

export function statusTone(status: string): StatusTone {
  switch (status) {
    case 'IN_SERVICE':
      return 'success'
    case 'MAINTENANCE':
      return 'warning'
    case 'OUT_OF_SERVICE':
      return 'danger'
    default:
      return 'neutral'
  }
}

/** Display order for type summaries: the hierarchy top-down; unknown types follow alphabetically. */
const TYPE_ORDER = ['SUBSTATION', 'TRANSFORMER', 'LV_BOARD', 'SWITCHBOARD', 'SWITCHBOARD_PANEL']

export const typeRank = (t: string) => (TYPE_ORDER.includes(t) ? TYPE_ORDER.indexOf(t) : TYPE_ORDER.length)

export function sortTypes<T>(items: T[], code: (item: T) => string): T[] {
  return [...items].sort((a, b) => typeRank(code(a)) - typeRank(code(b)) || code(a).localeCompare(code(b)))
}
