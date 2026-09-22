import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { mockApi, renderApp } from '../test/utils'

afterEach(() => {
  delete document.documentElement.dataset.theme
  localStorage.clear()
})

describe('theme', () => {
  it('starts light, switches to dark on the toggle and remembers the choice', async () => {
    mockApi()
    const user = userEvent.setup()
    renderApp('/')
    const toggle = await screen.findByRole('button', { name: 'Dark mode' })
    expect(toggle).toHaveAttribute('aria-pressed', 'false')

    await user.click(toggle)
    expect(document.documentElement.dataset.theme).toBe('dark')
    expect(localStorage.getItem('theme')).toBe('dark')
    expect(toggle).toHaveAttribute('aria-pressed', 'true')

    await user.click(toggle)
    expect(document.documentElement.dataset.theme).toBe('light')
    expect(localStorage.getItem('theme')).toBe('light')
    expect(toggle).toHaveAttribute('aria-pressed', 'false')
  })

  it('shows the theme that index.html set before first paint', async () => {
    document.documentElement.dataset.theme = 'dark'
    mockApi()
    renderApp('/')
    expect(await screen.findByRole('button', { name: 'Dark mode' })).toHaveAttribute('aria-pressed', 'true')
  })

  it('still switches when storage is blocked', async () => {
    const spy = () => {
      throw new DOMException('blocked', 'SecurityError')
    }
    const original = Storage.prototype.setItem
    Storage.prototype.setItem = spy
    try {
      mockApi()
      const user = userEvent.setup()
      renderApp('/')
      await user.click(await screen.findByRole('button', { name: 'Dark mode' }))
      expect(document.documentElement.dataset.theme).toBe('dark')
    } finally {
      Storage.prototype.setItem = original
    }
  })
})
