import { ArrowDown, ArrowUp, ArrowUpDown } from 'lucide-react'
import type { SortDir } from '../../lib'

/**
 * A table column heading that sorts by that column. The active column shows its
 * direction (and says it with aria-sort); the others show a faint up/down mark.
 */
export function SortHeader({
  label,
  active,
  dir,
  align,
  className = '',
  onSort,
}: {
  label: string
  active: boolean
  dir: SortDir
  align?: 'right'
  className?: string
  onSort: () => void
}) {
  const Icon = !active ? ArrowUpDown : dir === 'asc' ? ArrowUp : ArrowDown
  return (
    <th
      scope="col"
      aria-sort={active ? (dir === 'asc' ? 'ascending' : 'descending') : undefined}
      className={`px-4 py-3 font-semibold ${align === 'right' ? 'text-right' : ''} ${className}`}
    >
      <button type="button" onClick={onSort} className={`inline-flex items-center gap-1 rounded-sm uppercase hover:text-text ${active ? 'text-text' : ''}`}>
        {label}
        <Icon aria-hidden className={`size-4 ${active ? '' : 'opacity-50'}`} />
      </button>
    </th>
  )
}
