import { screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { mockApi, renderApp, statsBody } from '../test/utils'

describe('Overview', () => {
  it('shows totals by type, the status breakdown and the top-level assets', async () => {
    mockApi()
    renderApp('/')
    expect(await screen.findByRole('heading', { name: 'Overview' })).toBeInTheDocument()

    const cards = within(await screen.findByRole('list', { name: 'Assets by type' }))
    expect(cards.getAllByRole('listitem').map((li) => li.textContent)).toEqual([
      '4Total assets',
      '1Substations',
      '1Transformers',
      '1Switchboards',
      '1Switchboard Panels',
    ])
    const status = within(screen.getByRole('region', { name: 'Operational status' }))
    expect(status.getByText('In service')).toBeInTheDocument() // readable, not IN_SERVICE
    expect(status.getByText('4 · 100%')).toBeInTheDocument()
    // beside the top-level assets: the two share one grid row
    const top = screen.getByRole('region', { name: 'Top-level assets' })
    expect(top.parentElement).toBe(screen.getByRole('region', { name: 'Operational status' }).parentElement)
    expect(screen.getByRole('link', { name: /SUB-1 name/ })).toHaveAttribute('href', '/assets/SUB-1')
    // navigation lives in the header only: no shortcut buttons repeat it on the page
    const main = within(screen.getByRole('main'))
    expect(main.queryByRole('link', { name: 'All assets' })).toBeNull()
    expect(main.queryByRole('link', { name: 'Import activity' })).toBeNull()
    expect(main.queryByRole('link', { name: /download template/i })).toBeNull()
  })

  it('with no assets: onboarding steps and the template download', async () => {
    mockApi({ 'GET /api/assets/stats': statsBody(0), 'GET /api/assets/roots': { total: 0, roots: [] } })
    renderApp('/')
    expect(await screen.findByText('No assets yet', { selector: 'h2' })).toBeInTheDocument()
    expect(screen.getByText('Download the template')).toBeInTheDocument()
    const links = screen.getAllByRole('link', { name: /download template/i })
    expect(links[0]).toHaveAttribute('href', '/api/imports/template')
    expect(links[0]).toHaveAttribute('download')
    expect(screen.getByRole('link', { name: 'Go to Import' })).toHaveAttribute('href', '/import')
  })

  it('has its own error state with retry', async () => {
    mockApi({ 'GET /api/assets/stats': Response.json({ error: 'boom' }, { status: 500 }) })
    renderApp('/')
    expect(await screen.findByText('Could not load the overview')).toBeInTheDocument()
    // the header, tree and actions still work
    expect(await screen.findByRole('treeitem', { name: /SUB-1 name/ })).toBeInTheDocument()
  })
})
