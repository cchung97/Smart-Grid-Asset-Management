import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { detail, mockApi, renderApp, SWB } from '../test/utils'

describe('AssetDetailsPanel', () => {
  it('shows fields and immediate children grouped by type with counts', async () => {
    mockApi()
    renderApp('/assets/SUB-1')

    expect(await screen.findByRole('heading', { level: 1, name: 'SUB-1 name' })).toBeInTheDocument()
    expect(screen.getByText('22 kV')).toBeInTheDocument()
    expect(screen.getByText('14 Mar 2019')).toBeInTheDocument()
    expect(screen.getByText('— (root)')).toBeInTheDocument()

    const cards = within(await screen.findByRole('list', { name: 'Contents by type' }))
    expect(cards.getAllByRole('listitem').map((li) => li.textContent)).toEqual([
      '1Transformers1 direct',
      '0LV BoardsNone',
      '1Switchboards1 direct',
      '1Switchboard Panels0 direct · 1 deeper',
    ])
    const table = await screen.findByRole('table', { name: /Immediate children/ })
    expect(within(table).getByRole('link', { name: 'TX-1 name' })).toHaveAttribute('href', '/assets/TX-1')
    expect(within(table).getAllByText('In service')).toHaveLength(2) // readable status, not IN_SERVICE
    expect(within(table).queryByText('IN_SERVICE')).toBeNull()
  })

  describe('contents by type at every level (brief 4.3)', () => {
    const cards = async () =>
      within(await screen.findByRole('list', { name: 'Contents by type' })).getAllByRole('listitem').map((li) => li.textContent)

    it('shows every type that can sit beneath a substation, the total at every depth and how many are direct, with 0 for none', async () => {
      mockApi()
      renderApp('/assets/SUB-1')
      // the panel is two levels down (0 direct, 1 deeper); there is no LV board
      expect(await cards()).toEqual(['1Transformers1 direct', '0LV BoardsNone', '1Switchboards1 direct', '1Switchboard Panels0 direct · 1 deeper'])
    })

    it('shows only switchboard panels for a switchboard, and no cards for a leaf', async () => {
      mockApi({ 'GET /api/assets/SWB-1': detail(SWB) })
      const { unmount } = renderApp('/assets/SWB-1')
      expect(await cards()).toEqual(['1Switchboard Panels1 direct'])
      unmount()

      renderApp('/assets/PNL-1')
      expect(await screen.findByText('This asset has no child assets.')).toBeInTheDocument()
      expect(screen.queryByRole('list', { name: 'Contents by type' })).toBeNull()
    })

    it('makes a single children request for both the cards and the table', async () => {
      const calls = mockApi()
      renderApp('/assets/SUB-1')
      await cards()
      expect(calls.filter((c) => c === 'GET /api/assets/SUB-1/children')).toHaveLength(1)
    })

    it('still shows what the API returned if the parent rules cannot be loaded', async () => {
      mockApi({ 'GET /api/lookups': Response.json({ error: 'boom' }, { status: 500 }) })
      renderApp('/assets/SUB-1')
      expect(await cards()).toEqual(['1Transformers1 direct', '1Switchboards1 direct', '1Switchboard Panels0 direct · 1 deeper'])
    })

    it('leaves a failed request to the children error state', async () => {
      mockApi({ 'GET /api/assets/SUB-1/children': Response.json({ error: 'boom' }, { status: 500 }) })
      renderApp('/assets/SUB-1')
      expect(await screen.findByText('Could not load children')).toBeInTheDocument()
      expect(screen.queryByRole('list', { name: 'Contents by type' })).toBeNull()
    })
  })

  describe('sorting the children', () => {
    const ids = (table: HTMLElement) =>
      within(table).getAllByRole('row').slice(1).map((r) => within(r).getAllByRole('cell')[0].textContent)

    it('starts by type in hierarchy order and marks the active column', async () => {
      mockApi()
      renderApp('/assets/SUB-1')
      const table = await screen.findByRole('table', { name: /Immediate children/ })
      expect(ids(table)).toEqual(['TX-1', 'SWB-1']) // hierarchy order: transformer, then switchboard
      expect(within(table).getByRole('columnheader', { name: /Type/ })).toHaveAttribute('aria-sort', 'ascending')
      expect(within(table).getByRole('columnheader', { name: /Rating/ })).not.toHaveAttribute('aria-sort')
    })

    it('sorts by rating, flips on a second click, and keeps blanks last for commissioned date', async () => {
      mockApi()
      const user = userEvent.setup()
      renderApp('/assets/SUB-1')
      const table = await screen.findByRole('table', { name: /Immediate children/ })

      await user.click(within(table).getByRole('button', { name: /Rating/ }))
      expect(ids(table)).toEqual(['SWB-1', 'TX-1']) // 500 then 1000
      expect(within(table).getByRole('columnheader', { name: /Rating/ })).toHaveAttribute('aria-sort', 'ascending')

      await user.click(within(table).getByRole('button', { name: /Rating/ }))
      expect(ids(table)).toEqual(['TX-1', 'SWB-1'])
      expect(within(table).getByRole('columnheader', { name: /Rating/ })).toHaveAttribute('aria-sort', 'descending')
      expect(within(table).getByRole('columnheader', { name: /Type/ })).not.toHaveAttribute('aria-sort')

      // SWB-1 has no commissioned date: last whichever way it is sorted
      await user.click(within(table).getByRole('button', { name: /Commissioned/ }))
      expect(ids(table)).toEqual(['TX-1', 'SWB-1'])
      await user.click(within(table).getByRole('button', { name: /Commissioned/ }))
      expect(ids(table)).toEqual(['TX-1', 'SWB-1'])
    })

    it('restores the sort from the URL and ignores an unknown column', async () => {
      mockApi()
      const { unmount } = renderApp('/assets/SUB-1?sort=name&dir=desc')
      let table = await screen.findByRole('table', { name: /Immediate children/ })
      expect(within(table).getByRole('columnheader', { name: /Name/ })).toHaveAttribute('aria-sort', 'descending')
      expect(ids(table)).toEqual(['TX-1', 'SWB-1'])
      unmount()

      renderApp('/assets/SUB-1?sort=bogus')
      table = await screen.findByRole('table', { name: /Immediate children/ })
      expect(within(table).getByRole('columnheader', { name: /Type/ })).toHaveAttribute('aria-sort', 'ascending')
    })
  })

  it('shows a details-local 404 state while the tree still renders', async () => {
    mockApi({ 'GET /api/assets/NOPE': Response.json({ error: 'asset not found' }, { status: 404 }) })
    renderApp('/assets/NOPE')
    expect(await screen.findByText('Asset not found')).toBeInTheDocument()
    expect(await screen.findByRole('treeitem', { name: /SUB-1 name/ })).toBeInTheDocument()
  })

  it('shows a children-local error without hiding the asset', async () => {
    mockApi({ 'GET /api/assets/TX-1/children': Response.json({ error: 'boom' }, { status: 500 }) })
    renderApp('/assets/TX-1')
    expect(await screen.findByRole('heading', { level: 1, name: 'TX-1 name' })).toBeInTheDocument()
    expect(await screen.findByText('Could not load children')).toBeInTheDocument()
  })

  it('cannot delete an asset that has children, and says why', async () => {
    mockApi()
    renderApp('/assets/SUB-1')
    expect(await screen.findByText('Delete its 2 child assets first.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Delete asset' })).toBeDisabled()
  })

  it('deletes a leaf after confirmation and goes to its parent', async () => {
    const calls = mockApi({ 'DELETE /api/assets/TX-1': new Response(null, { status: 204 }) })
    const user = userEvent.setup()
    renderApp('/assets/TX-1')
    const button = await screen.findByRole('button', { name: 'Delete asset' })
    await waitFor(() => expect(button).toBeEnabled())

    await user.click(button)
    const dialog = await screen.findByRole('alertdialog')
    expect(dialog).toHaveTextContent('TX-1 name')
    expect(within(dialog).queryByLabelText(/api key/i)).toBeNull() // the proxy adds the key; the user never types it
    expect(calls).not.toContain('DELETE /api/assets/TX-1')
    await user.click(within(dialog).getByRole('button', { name: 'Delete asset' }))

    await waitFor(() => expect(calls).toContain('DELETE /api/assets/TX-1'))
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/assets/SUB-1'))
  })

  it('shows the API refusal inside the dialog and stays put', async () => {
    mockApi({ 'DELETE /api/assets/TX-1': Response.json({ error: 'asset "TX-1" has 1 child asset(s); delete them first' }, { status: 409 }) })
    const user = userEvent.setup()
    renderApp('/assets/TX-1')
    const button = await screen.findByRole('button', { name: 'Delete asset' })
    await waitFor(() => expect(button).toBeEnabled())
    await user.click(button)
    const dialog = await screen.findByRole('alertdialog')
    await user.click(within(dialog).getByRole('button', { name: 'Delete asset' }))
    expect(await screen.findByText(/has 1 child asset\(s\); delete them first/)).toBeInTheDocument()
    expect(screen.getByTestId('location')).toHaveTextContent('/assets/TX-1')
  })
})
