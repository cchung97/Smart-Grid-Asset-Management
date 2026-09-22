import { act, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { formatClock } from '../lib'
import { AppFooter, AppVersion } from './AppFooter'

describe('AppFooter', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 8, 21, 3, 9, 19))
  })
  afterEach(() => vi.useRealTimers())

  it('shows the links to the API docs and health check, and leaves the version to the sidebar by default', () => {
    render(<AppFooter />)
    expect(screen.queryByText(`v${__APP_VERSION__}`)).toBeNull()
    const docs = screen.getByRole('link', { name: 'API docs' })
    expect(docs).toHaveAttribute('href', '/swagger/index.html')
    expect(docs).toHaveAttribute('target', '_blank')
    expect(screen.getByRole('link', { name: 'Health' })).toHaveAttribute('href', '/healthz')
  })

  it('shows the version on the left when there is no sidebar to carry it', () => {
    render(<AppFooter withVersion />)
    expect(screen.getByText(`v${__APP_VERSION__}`)).toBeInTheDocument()
  })

  it('takes its version from package.json', () => {
    expect(__APP_VERSION__).toMatch(/^\d+\.\d+\.\d+$/)
  })

  it('ticks every second and stops when unmounted', () => {
    const { unmount } = render(<AppFooter />)
    expect(screen.getByText(/2026-09-21 03:09:19/)).toBeInTheDocument()
    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(screen.getByText(/2026-09-21 03:09:20/)).toBeInTheDocument()
    unmount()
    expect(vi.getTimerCount()).toBe(0)
  })
})

describe('AppVersion', () => {
  it('shows the name, version and copyright', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 8, 21))
    render(<AppVersion />)
    expect(screen.getByText(`v${__APP_VERSION__}`)).toBeInTheDocument()
    expect(screen.getByText(/© 2026 Nicholas Ong\. All rights reserved\./)).toBeInTheDocument()
    vi.useRealTimers()
  })
})

describe('formatClock', () => {
  it('pads the date and time and appends the time zone name', () => {
    expect(formatClock(new Date(2026, 0, 2, 3, 4, 5))).toMatch(/^2026-01-02 03:04:05( \S+)?$/)
  })
})
