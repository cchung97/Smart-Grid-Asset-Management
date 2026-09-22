import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { mockApi, renderApp } from '../test/utils'

describe('AppShell navigation', () => {
  it('has links to the overview and all assets, marking the current page', async () => {
    mockApi()
    renderApp('/assets')
    const nav = within((await screen.findAllByRole('navigation', { name: 'Main' }))[0])
    expect(nav.getByRole('link', { name: 'All assets' })).toHaveAttribute('aria-current', 'page')
    expect(nav.getByRole('link', { name: 'Overview' })).not.toHaveAttribute('aria-current')
    expect(nav.queryByRole('link', { name: 'Import activity' })).toBeNull() // history now lives on the Import page
  })

  it.each(['/', '/assets', '/assets/SUB-1'])('offers an Import button from the explorer at %s', async (path) => {
    mockApi()
    renderApp(path)
    expect(await screen.findByRole('link', { name: 'Import' })).toHaveAttribute('href', '/import')
  })

  it('puts the version and copyright at the bottom of the tree rail, and the docs link in the footer', async () => {
    mockApi()
    renderApp('/')
    const rail = await screen.findByRole('complementary', { name: 'Asset tree' })
    expect(within(rail).getByText(`v${__APP_VERSION__}`)).toBeInTheDocument()
    expect(within(rail).getByText(/All rights reserved/)).toBeInTheDocument()
    const footer = screen.getByRole('contentinfo')
    expect(within(footer).getByRole('link', { name: 'API docs' })).toHaveAttribute('href', '/swagger/index.html')
    expect(within(footer).queryByText(`v${__APP_VERSION__}`)).toBeNull()
  })

  it('without a rail (Import) the version moves into the footer', async () => {
    mockApi()
    renderApp('/import')
    const footer = await screen.findByRole('contentinfo')
    expect(within(footer).getByText(`v${__APP_VERSION__}`)).toBeInTheDocument()
    expect(within(footer).getByRole('link', { name: 'API docs' })).toBeInTheDocument()
  })

  it('the Import page has the header but no asset tree, and its nav stays visible', async () => {
    mockApi()
    renderApp('/import')
    expect(await screen.findByRole('heading', { level: 1, name: 'Import' })).toBeInTheDocument()
    expect(screen.queryByRole('tree')).toBeNull()
    expect(screen.queryByRole('complementary', { name: 'Asset tree' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Assets' })).toBeNull() // no drawer without a rail
    expect(screen.getByRole('link', { name: 'Import' })).toHaveAttribute('aria-current', 'page')
    expect(within(screen.getByRole('navigation', { name: 'Main' })).getByRole('link', { name: 'All assets' })).toBeInTheDocument()
  })

  it('the explorer rail is the asset tree only (no import panel)', async () => {
    mockApi()
    renderApp('/')
    const rail = await screen.findByRole('complementary', { name: 'Asset tree' })
    expect(await within(rail).findByRole('tree', { name: 'Asset tree' })).toBeInTheDocument()
    expect(within(rail).queryByText('Import CSV')).toBeNull()
    expect(within(rail).queryByLabelText('Choose file')).toBeNull()
  })

  it('on small screens the rail opens as a drawer and closes when you navigate', async () => {
    mockApi()
    const user = userEvent.setup()
    renderApp('/')
    await user.click(await screen.findByRole('button', { name: 'Assets' }))

    const drawer = await screen.findByRole('dialog', { name: 'Assets' })
    expect(within(drawer).getByRole('tree', { name: 'Asset tree' })).toBeInTheDocument()

    await user.click(await within(drawer).findByRole('treeitem', { name: /TX-1 name|SUB-1 name/ }))
    expect(screen.queryByRole('dialog', { name: 'Assets' })).toBeNull() // navigating closed it
    expect(screen.getByTestId('location')).toHaveTextContent('/assets/SUB-1')
  })

  it('the scrolling panels are positioned, so hidden (sr-only) helpers cannot stretch the page', async () => {
    // sr-only elements are absolutely positioned; without a positioned ancestor they escape the
    // panel's overflow and make the whole document scroll (down and sideways).
    mockApi()
    renderApp('/')
    expect(await screen.findByRole('main')).toHaveClass('relative', 'overflow-y-auto')
    expect(screen.getByRole('complementary', { name: 'Asset tree' })).toHaveClass('relative', 'overflow-hidden')
    // every tree row carries an sr-only status label: the tree's own scroll container has to contain them
    const scroller = (await screen.findByRole('tree', { name: 'Asset tree' })).closest('.overflow-y-auto')
    expect(scroller).toHaveClass('relative')
  })
})
