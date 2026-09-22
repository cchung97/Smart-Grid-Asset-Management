import { render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ImportResponse } from '../api/types'
import { previewBody } from '../test/utils'
import { ImportResult } from './ImportResult'

const base: ImportResponse = {
  import_id: 'i',
  total_rows: 222,
  imported_rows: 0,
  rejected_rows: 222,
  committed: false,
  ignored_columns: ['notes'],
  rejections: previewBody.rejections,
}

describe('ImportResult', () => {
  it('nothing stored: says so, with counts, ignored columns and every rejected row', () => {
    render(<ImportResult fileName="grid_assets.csv" result={base} />)
    expect(screen.getByText('Data committed: no — nothing was saved')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'grid_assets.csv' })).toBeInTheDocument()
    expect(screen.getByText('Total rows').previousSibling).toHaveTextContent('222')
    expect(screen.getByText('Imported').previousSibling).toHaveTextContent('0')
    expect(screen.getByText(/notes/)).toBeInTheDocument()
    const rows = within(screen.getByRole('table')).getAllByRole('row').slice(1)
    expect(rows).toHaveLength(2)
    expect(rows[1]).toHaveTextContent('cycle detected')
  })

  it('partly stored: committed yes, counts both ways', () => {
    render(<ImportResult fileName="g.csv" result={{ ...base, committed: true, imported_rows: 205, rejected_rows: 17 }} />)
    expect(screen.getByText('Data committed: yes — 205 rows stored, 17 rejected rows were not saved')).toBeInTheDocument()
  })

  it('all stored: no rejection table or report button', () => {
    render(<ImportResult fileName="g.csv" result={{ ...base, committed: true, imported_rows: 222, rejected_rows: 0, rejections: [], ignored_columns: [] }} />)
    expect(screen.getByText('Data committed: yes — all 222 rows imported')).toBeInTheDocument()
    expect(screen.queryByRole('table')).toBeNull()
    expect(screen.queryByRole('button', { name: /download/i })).toBeNull()
  })
})
