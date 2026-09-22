import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { importsBody, mockApi, renderApp } from '../test/utils'

describe('Import history', () => {
  it('says it lists imports only and shows each import with its counts and outcome', async () => {
    mockApi()
    renderApp('/import')
    expect(await screen.findByRole('heading', { name: 'Import history' })).toBeInTheDocument()
    expect(screen.getByText(/Imports only — deletions are not listed here/)).toBeInTheDocument()

    const list = await screen.findByRole('list', { name: 'Past imports, page 1' })
    const run = within(within(list).getByRole('listitem'))
    expect(run.getByRole('button', { name: /grid_assets.csv/ })).toBeInTheDocument()
    expect(run.getByText(/205 of 222 rows imported/)).toBeInTheDocument()
    expect(run.getByText(/17 rejected/)).toBeInTheDocument()
    expect(run.getByText('Partly stored')).toBeInTheDocument()
    expect(run.getByText(/2026/)).toBeInTheDocument()
    expect(run.queryByText('203.0.113.7')).toBeNull() // the client IP is in the details
    expect(screen.getByText('Showing 1–1 of 1 imports')).toBeInTheDocument()
  })

  it('opens an import to show who uploaded it and its rejected rows (no download button here)', async () => {
    const calls = mockApi()
    const user = userEvent.setup()
    renderApp('/import')
    const toggle = await screen.findByRole('button', { name: /grid_assets.csv, show details/i })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    expect(calls).not.toContain('GET /api/imports/run-1')

    await user.click(toggle)
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByText('203.0.113.7')).toBeInTheDocument()
    const table = await screen.findByRole('table', { name: /Rows rejected from/ })
    expect(within(table).getByText('TX-NR')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /download report/i })).toBeNull() // the report is offered right after an import, not in the history
  })

  it('shows an empty state and an error state', async () => {
    mockApi({ 'GET /api/imports?limit=25&offset=0': { ...importsBody, total: 0, items: [] } })
    const { unmount } = renderApp('/import')
    expect(await screen.findByText('No imports yet')).toBeInTheDocument()
    unmount()

    mockApi({ 'GET /api/imports?limit=25&offset=0': Response.json({ error: 'boom' }, { status: 500 }) })
    renderApp('/import')
    expect(await screen.findByText('Could not load import activity')).toBeInTheDocument()
  })

  it('the old /imports address redirects to /import and keeps ?page=', async () => {
    mockApi()
    renderApp('/imports?page=1')
    expect(await screen.findByRole('heading', { name: 'Import history' })).toBeInTheDocument()
    expect(screen.getByTestId('location')).toHaveTextContent('/import')
  })
})
