import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { mockApi, renderApp } from '../test/utils'

describe('SearchBar', () => {
  it('search → Enter → ancestors → tree expanded → navigate → highlighted', async () => {
    const calls = mockApi()
    const user = userEvent.setup()
    renderApp('/')

    const box = screen.getByRole('combobox', { name: /search assets/i })
    await user.type(box, 'panel')

    // debounced: results arrive after the 300ms window, as one request
    const option = await screen.findByRole('option', { name: /PNL-1 name/ }, { timeout: 2000 })
    expect(option).toHaveTextContent('PNL-1')
    expect(calls.filter((c) => c.startsWith('GET /api/assets/search'))).toEqual([
      'GET /api/assets/search?q=panel&limit=20',
    ])

    await user.keyboard('{ArrowDown}{Enter}')

    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/assets/PNL-1'))
    const pnl = await screen.findByRole('treeitem', { name: /PNL-1 name/ })
    expect(pnl).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('treeitem', { name: /SUB-1 name/ })).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByRole('treeitem', { name: /SWB-1 name/ })).toHaveAttribute('aria-expanded', 'true')

    // call order: search → ancestors → (only then) the routed asset's details
    const at = (call: string) => calls.indexOf(call)
    expect(at('GET /api/assets/PNL-1/ancestors')).toBeGreaterThan(
      at('GET /api/assets/search?q=panel&limit=20'),
    )
    expect(at('GET /api/assets/PNL-1')).toBeGreaterThan(at('GET /api/assets/PNL-1/ancestors'))
    expect(box).toHaveValue('')
  })

  it('arrow keys move the active option and Escape closes the list', async () => {
    mockApi()
    const user = userEvent.setup()
    renderApp('/')
    const box = screen.getByRole('combobox')
    await user.type(box, 'panel')
    await screen.findByRole('option', { name: /PNL-1 name/ }, { timeout: 2000 })
    expect(box).toHaveAttribute('aria-expanded', 'true')

    await user.keyboard('{ArrowDown}')
    expect(box).toHaveAttribute('aria-activedescendant', screen.getByRole('option').id)
    expect(screen.getByRole('option')).toHaveAttribute('aria-selected', 'true')

    await user.keyboard('{Escape}')
    expect(box).toHaveAttribute('aria-expanded', 'false')
    expect(screen.getByTestId('location')).toHaveTextContent('/')
  })

  it('says so when nothing matches', async () => {
    mockApi({
      'GET /api/assets/search?q=zzz&limit=20': { query: 'zzz', type: '', count: 0, truncated: false, results: [] },
    })
    const user = userEvent.setup()
    renderApp('/')
    await user.type(screen.getByRole('combobox'), 'zzz')
    expect(await screen.findByText(/No assets match/, undefined, { timeout: 2000 })).toBeInTheDocument()
  })
})
