import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import type { StatsResponse } from '../api/types'
import { StatusDonut } from './StatusDonut'

const stats: StatsResponse = {
  total: 205,
  by_type: [],
  by_status: [
    { operational_status: 'IN_SERVICE', count: 130 },
    { operational_status: 'MAINTENANCE', count: 37 },
    { operational_status: 'OUT_OF_SERVICE', count: 38 },
  ],
}

const arcs = () => [...document.querySelectorAll<SVGCircleElement>('circle[data-status]')]
const len = (c: SVGCircleElement) => Number(c.getAttribute('stroke-dasharray')!.split(' ')[0])

describe('StatusDonut', () => {
  it('draws one segment per status, in order, sized by share, with a gap between them', () => {
    render(<StatusDonut stats={stats} />)
    const [inService, maintenance, outOfService] = arcs()
    expect(arcs().map((c) => c.dataset.status)).toEqual(['IN_SERVICE', 'MAINTENANCE', 'OUT_OF_SERVICE'])

    const C = 2 * Math.PI * 56
    expect(len(inService)).toBeCloseTo((130 / 205) * C - 2, 1) // share of the ring, less the 2-unit gap
    expect(len(maintenance)).toBeLessThan(len(outOfService)) // 37 < 38
    // each segment starts where the previous one ended
    expect(Number(maintenance.getAttribute('stroke-dashoffset'))).toBeCloseTo(-(130 / 205) * C, 1)
    expect(Number(outOfService.getAttribute('stroke-dashoffset'))).toBeCloseTo(-((130 + 37) / 205) * C, 1)
  })

  it('carries the exact numbers in a legend (close values cannot be read off the ring), with readable labels and icons', () => {
    render(<StatusDonut stats={stats} />)
    const region = within(screen.getByRole('region', { name: 'Operational status' }))
    const rows = region.getAllByRole('listitem')
    expect(rows.map((r) => r.textContent)).toEqual(['In service130 · 63%', 'Maintenance37 · 18%', 'Out of service38 · 19%'])
    for (const row of rows) expect(row.querySelector('svg')).toBeInTheDocument() // colour is never the only cue
    expect(region.queryByText('IN_SERVICE')).toBeNull()
  })

  it('describes itself to screen readers and shows the total in the centre', () => {
    render(<StatusDonut stats={stats} />)
    expect(screen.getByRole('img', { name: /In service 130 \(63%\), Maintenance 37 \(18%\), Out of service 38 \(19%\)/ })).toBeInTheDocument()
    expect(screen.getByText('205')).toBeInTheDocument()
  })

  it('hovering a segment or a legend row puts that status in the centre and dims the others', async () => {
    const user = userEvent.setup()
    render(<StatusDonut stats={stats} />)
    const maintenanceRow = screen.getAllByRole('listitem')[1]
    await user.hover(maintenanceRow)
    expect(screen.getByText('37')).toBeInTheDocument()
    expect(arcs().filter((c) => c.classList.contains('opacity-30')).map((c) => c.dataset.status)).toEqual(['IN_SERVICE', 'OUT_OF_SERVICE'])
    await user.unhover(maintenanceRow)
    expect(screen.getByText('205')).toBeInTheDocument()

    await user.hover(arcs()[2])
    expect(screen.getByText('38')).toBeInTheDocument()
  })

  it('one status is a whole ring with no gap, and an empty status list still renders', () => {
    const { unmount } = render(<StatusDonut stats={{ total: 4, by_type: [], by_status: [{ operational_status: 'IN_SERVICE', count: 4 }] }} />)
    expect(len(arcs()[0])).toBeCloseTo(2 * Math.PI * 56, 1)
    unmount()
    render(<StatusDonut stats={{ total: 0, by_type: [], by_status: [] }} />)
    expect(arcs()).toHaveLength(0)
  })
})
