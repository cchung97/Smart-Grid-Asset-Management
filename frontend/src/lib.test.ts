import { describe, expect, it } from 'vitest'
import { statusLabel } from './api/assetTypes'
import type { AssetNode } from './api/types'
import { descendantTypes, formatBytes, rejectionsToCsv, reportFilename, sortChildren, type ChildSortKey, type SortDir } from './lib'

describe('rejectionsToCsv', () => {
  it('writes a header and one line per rejection, quoting commas, quotes and newlines', () => {
    const csv = rejectionsToCsv([
      { csv_row: 2, asset_id: 'TX-1', reason: 'bad, very bad' },
      { csv_row: 3, asset_id: '', reason: 'says "no"' },
    ])
    expect(csv).toBe('row,asset_id,reason\r\n2,TX-1,"bad, very bad"\r\n3,,"says ""no"""\r\n')
  })

  it('neutralises spreadsheet formulas coming from the uploaded file', () => {
    const csv = rejectionsToCsv([{ csv_row: 2, asset_id: '=HYPERLINK("http://x")', reason: '+1' }])
    expect(csv).toContain(`"'=HYPERLINK(""http://x"")"`)
    expect(csv).toContain(",'+1")
  })
})

describe('helpers', () => {
  it('names the report after the file', () => {
    expect(reportFilename('grid_assets.csv')).toBe('grid_assets-rejections.csv')
    expect(reportFilename('GRID.CSV')).toBe('GRID-rejections.csv')
  })
  it('formats sizes', () => {
    expect([formatBytes(512), formatBytes(2048), formatBytes(5 * 1024 * 1024)]).toEqual(['512 B', '2.0 KB', '5.0 MB'])
  })
})

describe('statusLabel', () => {
  it.each([
    ['IN_SERVICE', 'In service'],
    ['OUT_OF_SERVICE', 'Out of service'],
    ['MAINTENANCE', 'Maintenance'],
    ['DECOMMISSIONED_SOON', 'Decommissioned soon'],
  ])('%s -> %s', (code, want) => expect(statusLabel(code)).toBe(want))
})

describe('sortChildren', () => {
  const n = (id: string, type: string, name: string, status: string, rating: number | null, date: string | null): AssetNode => ({
    asset_id: id, asset_type: type, asset_name: name, operational_status: status, parent_asset_id: 'P',
    rating_kva: rating, commissioned_date: date, child_count: 0, subtree_count: 1,
  })
  const rows = [
    n('TX-2', 'TRANSFORMER', 'Transformer 10', 'MAINTENANCE', 250, '2021-01-01'),
    n('SWB-1', 'SWITCHBOARD', 'Switchboard 1', 'IN_SERVICE', null, null),
    n('TX-1', 'TRANSFORMER', 'Transformer 2', 'OUT_OF_SERVICE', 1000, '2019-05-02'),
    n('LV-1', 'LV_BOARD', 'LV Board', 'IN_SERVICE', 100, '2020-06-30'),
  ]
  const ids = (key: ChildSortKey, dir: SortDir) => sortChildren(rows, key, dir).map((r) => r.asset_id)

  it.each<[ChildSortKey, SortDir, string[]]>([
    ['type', 'asc', ['TX-1', 'TX-2', 'LV-1', 'SWB-1']], // hierarchy order, then id
    ['type', 'desc', ['SWB-1', 'LV-1', 'TX-1', 'TX-2']],
    ['name', 'asc', ['LV-1', 'SWB-1', 'TX-1', 'TX-2']], // "Transformer 2" before "Transformer 10" (numeric)
    ['name', 'desc', ['TX-2', 'TX-1', 'SWB-1', 'LV-1']],
    ['status', 'asc', ['LV-1', 'SWB-1', 'TX-2', 'TX-1']], // In service, Maintenance, Out of service
    ['rating', 'asc', ['LV-1', 'TX-2', 'TX-1', 'SWB-1']], // no rating is last
    ['rating', 'desc', ['TX-1', 'TX-2', 'LV-1', 'SWB-1']], // ... in both directions
    ['commissioned', 'asc', ['TX-1', 'LV-1', 'TX-2', 'SWB-1']],
    ['commissioned', 'desc', ['TX-2', 'LV-1', 'TX-1', 'SWB-1']],
  ])('by %s %s', (key, dir, want) => expect(ids(key, dir)).toEqual(want))

  it('does not mutate its input', () => {
    const before = rows.map((r) => r.asset_id)
    sortChildren(rows, 'name', 'desc')
    expect(rows.map((r) => r.asset_id)).toEqual(before)
  })
})

describe('descendantTypes', () => {
  const rules = [
    { child_type: 'SWITCHBOARD_PANEL', parent_type: 'SWITCHBOARD' },
    { child_type: 'LV_BOARD', parent_type: 'SUBSTATION' },
    { child_type: 'TRANSFORMER', parent_type: 'SUBSTATION' },
    { child_type: 'SWITCHBOARD', parent_type: 'SUBSTATION' },
  ]

  it('follows the parent rules transitively and returns hierarchy order', () => {
    expect(descendantTypes('SUBSTATION', rules)).toEqual(['TRANSFORMER', 'LV_BOARD', 'SWITCHBOARD', 'SWITCHBOARD_PANEL'])
  })

  it('returns only what can sit beneath a mid-level type, and nothing for a leaf type', () => {
    expect(descendantTypes('SWITCHBOARD', rules)).toEqual(['SWITCHBOARD_PANEL'])
    expect(descendantTypes('SWITCHBOARD_PANEL', rules)).toEqual([])
    expect(descendantTypes('UNKNOWN', rules)).toEqual([])
  })

  it('reads the rules, not type names, and cannot loop on a cyclic rule set', () => {
    expect(descendantTypes('A', [{ child_type: 'B', parent_type: 'A' }, { child_type: 'C', parent_type: 'B' }])).toEqual(['B', 'C'])
    expect(descendantTypes('A', [{ child_type: 'B', parent_type: 'A' }, { child_type: 'A', parent_type: 'B' }])).toEqual(['A', 'B'])
  })
})
