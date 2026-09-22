import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { listBody, mockApi, PNL, renderApp, SUB, SWB, TX } from '../test/utils'

const manyItems = { ...listBody([SUB, SWB], 60), limit: 25 }

describe('AllAssetsPanel', () => {
  it('lists every asset with a wide, non-wrapping ID column and links to each asset', async () => {
    mockApi()
    renderApp('/assets')
    expect(await screen.findByRole('heading', { name: 'All assets' })).toBeInTheDocument()

    const table = await screen.findByRole('table')
    const rows = within(table).getAllByRole('row').slice(1)
    expect(rows).toHaveLength(4)
    const idCell = within(rows[0]).getByRole('link', { name: 'SUB-1' }).closest('td')
    expect(idCell).toHaveClass('whitespace-nowrap')
    expect(within(rows[3]).getByRole('link', { name: 'SWB-1' })).toHaveAttribute('href', '/assets/SWB-1') // PNL-1's parent
    expect(screen.getByText('Showing 1–4 of 4 assets')).toBeInTheDocument()
  })

  it('filters by type and status through the API and keeps them in the URL', async () => {
    const calls = mockApi({
      'GET /api/assets?limit=25&offset=0&type=TRANSFORMER': listBody([TX]),
      'GET /api/assets?limit=25&offset=0&type=TRANSFORMER&status=MAINTENANCE': listBody([]),
    })
    const user = userEvent.setup()
    renderApp('/assets')
    await screen.findByRole('table')

    await user.selectOptions(screen.getByLabelText('Filter by type'), 'TRANSFORMER')
    await waitFor(() => expect(within(screen.getByRole('table')).getAllByRole('row')).toHaveLength(2))
    expect(calls).toContain('GET /api/assets?limit=25&offset=0&type=TRANSFORMER')

    await user.selectOptions(screen.getByLabelText('Filter by status'), 'MAINTENANCE')
    expect(await screen.findByText('No assets match these filters')).toBeInTheDocument()
  })

  it('filters by text after the debounce and pages with the server total', async () => {
    const calls = mockApi({
      'GET /api/assets?limit=25&offset=0': manyItems,
      'GET /api/assets?limit=25&offset=25': { ...manyItems, offset: 25 },
      'GET /api/assets?limit=25&offset=0&q=pnl': listBody([PNL]),
    })
    const user = userEvent.setup()
    renderApp('/assets')
    expect(await screen.findByText('Showing 1–25 of 60 assets')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /previous/i })).toBeDisabled()

    await user.click(screen.getByRole('button', { name: /next/i }))
    expect(await screen.findByText('Showing 26–50 of 60 assets')).toBeInTheDocument()
    expect(calls).toContain('GET /api/assets?limit=25&offset=25')

    await user.type(screen.getByLabelText('Filter by asset ID or name'), 'pnl')
    await waitFor(() => expect(calls).toContain('GET /api/assets?limit=25&offset=0&q=pnl'), { timeout: 2000 })
    expect(calls.filter((c) => c.includes('q=')).length).toBe(1) // one request for the whole word, not one per key
  })

  it('shows an empty state', async () => {
    mockApi({ 'GET /api/assets?limit=25&offset=0': listBody([]) })
    renderApp('/assets')
    expect(await screen.findByText('No assets yet')).toBeInTheDocument()
  })

  it('shows an error state with retry, leaving the rest of the shell alone', async () => {
    mockApi({ 'GET /api/assets?limit=25&offset=0': Response.json({ error: 'boom' }, { status: 500 }) })
    renderApp('/assets')
    expect(await screen.findByText('Could not load assets')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument()
    expect(await screen.findByRole('treeitem', { name: /SUB-1 name/ })).toBeInTheDocument()
  })

  describe('sorting', () => {
    const idsInTable = () => within(screen.getByRole('table')).getAllByRole('row').slice(1).map((r) => within(r).getAllByRole('cell')[0].textContent)
    const header = (name: RegExp) => within(screen.getByRole('table')).getByRole('columnheader', { name })

    it('starts by asset ID ascending and shows it on the heading', async () => {
      mockApi()
      renderApp('/assets')
      await screen.findByRole('table')
      expect(header(/Asset ID/)).toHaveAttribute('aria-sort', 'ascending')
      expect(header(/Name/)).not.toHaveAttribute('aria-sort')
    })

    it('asks the server to sort (so it covers every page), flips on a second click and returns to page 1', async () => {
      const calls = mockApi({
        'GET /api/assets?limit=25&offset=25&sort=status': listBody([SUB, SWB], 60),
        'GET /api/assets?limit=25&offset=0&sort=name': listBody([PNL, SUB, SWB, TX]),
        'GET /api/assets?limit=25&offset=0&sort=name&dir=desc': listBody([TX, SWB, SUB, PNL]),
        'GET /api/assets?limit=25&offset=0&sort=parent': listBody([SWB, TX, PNL, SUB]),
      })
      const user = userEvent.setup()
      renderApp('/assets?page=2&sort=status')
      await screen.findByRole('table')
      expect(header(/Status/)).toHaveAttribute('aria-sort', 'ascending')

      await user.click(within(screen.getByRole('table')).getByRole('button', { name: /Name/ }))
      await waitFor(() => expect(calls).toContain('GET /api/assets?limit=25&offset=0&sort=name')) // back on page 1
      await waitFor(() => expect(idsInTable()).toEqual(['PNL-1', 'SUB-1', 'SWB-1', 'TX-1']))
      expect(header(/Name/)).toHaveAttribute('aria-sort', 'ascending')

      await user.click(within(screen.getByRole('table')).getByRole('button', { name: /Name/ }))
      await waitFor(() => expect(idsInTable()).toEqual(['TX-1', 'SWB-1', 'SUB-1', 'PNL-1']))
      expect(header(/Name/)).toHaveAttribute('aria-sort', 'descending')
      expect(calls).toContain('GET /api/assets?limit=25&offset=0&sort=name&dir=desc')

      await user.click(within(screen.getByRole('table')).getByRole('button', { name: /Parent/ }))
      await waitFor(() => expect(idsInTable()).toEqual(['SWB-1', 'TX-1', 'PNL-1', 'SUB-1']))
      expect(header(/Name/)).not.toHaveAttribute('aria-sort')
    })

    it('sorting keeps the filters, and the asset ID heading can be reversed', async () => {
      const calls = mockApi({
        'GET /api/assets?limit=25&offset=0&type=TRANSFORMER': listBody([TX]),
        'GET /api/assets?limit=25&offset=0&type=TRANSFORMER&dir=desc': listBody([TX]),
      })
      const user = userEvent.setup()
      renderApp('/assets?type=TRANSFORMER')
      await screen.findByRole('table')
      await user.click(within(screen.getByRole('table')).getByRole('button', { name: /Asset ID/ }))
      await waitFor(() => expect(calls).toContain('GET /api/assets?limit=25&offset=0&type=TRANSFORMER&dir=desc'))
      await waitFor(() => expect(header(/Asset ID/)).toHaveAttribute('aria-sort', 'descending'))
    })

    it('ignores an unknown sort column in the URL', async () => {
      const calls = mockApi()
      renderApp('/assets?sort=rating&dir=sideways')
      await screen.findByRole('table')
      expect(header(/Asset ID/)).toHaveAttribute('aria-sort', 'ascending')
      expect(calls).toContain('GET /api/assets?limit=25&offset=0')
    })
  })

  it('the filter dropdowns draw their own chevron inset from the edge', async () => {
    mockApi()
    renderApp('/assets')
    const select = await screen.findByLabelText('Filter by status')
    expect(select).toHaveClass('appearance-none', 'pr-8')
    expect(select.nextElementSibling).toHaveClass('right-3', 'pointer-events-none')
  })
})
