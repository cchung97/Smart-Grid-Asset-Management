import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { mockApi, renderApp } from '../test/utils'

const item = (name: string) => screen.findByRole('treeitem', { name: new RegExp(name) })

describe('AssetTree', () => {
  it('renders roots collapsed and fetches children only on first expand', async () => {
    const calls = mockApi()
    const user = userEvent.setup()
    renderApp('/')

    const sub = await item('SUB-1 name')
    expect(sub).toHaveAttribute('aria-expanded', 'false')
    expect(calls).not.toContain('GET /api/assets/SUB-1/children')

    sub.focus()
    await user.keyboard('{ArrowRight}')
    expect(await item('SWB-1 name')).toBeInTheDocument()
    expect(await item('TX-1 name')).toBeInTheDocument()
    expect(sub).toHaveAttribute('aria-expanded', 'true')

    await user.keyboard('{ArrowLeft}')
    await waitFor(() => expect(screen.queryByRole('treeitem', { name: /TX-1 name/ })).toBeNull())
    expect(sub).toHaveAttribute('aria-expanded', 'false')

    await user.keyboard('{ArrowRight}') // re-expand: served from the query cache
    expect(await item('TX-1 name')).toBeInTheDocument()
    expect(calls.filter((c) => c === 'GET /api/assets/SUB-1/children')).toHaveLength(1)
  })

  it('shows the substation descendant count badge', async () => {
    mockApi()
    renderApp('/')
    const sub = await item('SUB-1 name')
    expect(sub).toHaveTextContent('3') // subtree_count 4 includes itself
  })

  it('marks the routed asset selected and reveals it by expanding ancestors', async () => {
    mockApi()
    renderApp('/assets/PNL-1')

    const pnl = await item('PNL-1 name')
    expect(pnl).toHaveAttribute('aria-selected', 'true')
    expect(await item('SWB-1 name')).toHaveAttribute('aria-selected', 'false')
    expect(await item('SUB-1 name')).toHaveAttribute('aria-expanded', 'true')
    expect(pnl).toHaveClass('bg-primary-soft')
  })

  it('selecting a node navigates to /assets/:id', async () => {
    mockApi()
    const user = userEvent.setup()
    renderApp('/')
    await user.click(await item('SUB-1 name'))
    expect(screen.getByTestId('location')).toHaveTextContent('/assets/SUB-1')
    expect(await item('TX-1 name')).toBeInTheDocument()
  })

  it('moves focus with the arrow keys and keeps a single tab stop', async () => {
    mockApi()
    const user = userEvent.setup()
    renderApp('/')
    const sub = await item('SUB-1 name')
    sub.focus()
    await user.keyboard('{ArrowRight}')
    const swb = await item('SWB-1 name')
    await user.keyboard('{ArrowDown}')
    expect(swb).toHaveFocus()
    await user.keyboard('{ArrowLeft}') // leaf-or-collapsed: jump to parent
    expect(sub).toHaveFocus()
    await waitFor(() => {
      const stops = screen.getAllByRole('treeitem').filter((el) => el.tabIndex === 0)
      expect(stops).toHaveLength(1)
    })
  })

  it('shows an empty state when there are no assets', async () => {
    mockApi({ 'GET /api/assets/roots': { total: 0, roots: [] } })
    renderApp('/')
    expect(await screen.findByText('No assets yet')).toBeInTheDocument()
  })

  it('shows a tree-local error with retry', async () => {
    mockApi({ 'GET /api/assets/roots': Response.json({ error: 'boom' }, { status: 500 }) })
    renderApp('/')
    const rail = within(await screen.findByRole('complementary', { name: /^asset tree$/i }))
    expect(await rail.findByText('Could not load the asset tree')).toBeInTheDocument()
    expect(rail.getByRole('button', { name: /try again/i })).toBeInTheDocument()
    // the rest of the main panel is unaffected by the tree failing
    expect(screen.getByRole('heading', { name: 'Overview' })).toBeInTheDocument()
    expect(await screen.findByText('Total assets')).toBeInTheDocument()
  })

  it('Collapse all closes every open node', async () => {
    mockApi()
    const user = userEvent.setup()
    renderApp('/assets/PNL-1') // opens SUB-1 and SWB-1 to reveal the selected panel
    expect(await item('PNL-1 name')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Collapse all' }))
    await waitFor(() => expect(screen.queryByRole('treeitem', { name: /SWB-1 name/ })).toBeNull())
    expect(screen.queryByRole('treeitem', { name: /PNL-1 name/ })).toBeNull()
    expect(await item('SUB-1 name')).toHaveAttribute('aria-expanded', 'false')
  })

  it('has one icon-only Collapse all button in the tree header, and no Expand all', async () => {
    mockApi()
    renderApp('/')
    const rail = await screen.findByRole('complementary', { name: 'Asset tree' })
    const button = within(rail).getByRole('button', { name: 'Collapse all' })
    expect(button).toHaveTextContent('') // icon only, no label text
    expect(within(rail).queryByRole('button', { name: /expand all/i })).toBeNull()
  })

  it('a collapsed ancestor of the selected asset stays collapsed', async () => {
    mockApi()
    const user = userEvent.setup()
    renderApp('/assets/PNL-1')
    const swb = await item('SWB-1 name')
    expect(swb).toHaveAttribute('aria-expanded', 'true')

    swb.focus()
    await user.keyboard('{ArrowLeft}')
    await waitFor(() => expect(screen.queryByRole('treeitem', { name: /PNL-1 name/ })).toBeNull())
    expect(swb).toHaveAttribute('aria-expanded', 'false') // not re-opened to reveal the selection
  })
})
