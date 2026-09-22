import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import type { ImportResponse } from '../api/types'
import { mockApi, previewBody, renderApp, rootsBody } from '../test/utils'

const emptyTree = {
  'GET /api/assets/roots': rootsBody([]),
  'GET /api/assets/stats': { total: 0, by_type: [], by_status: [] },
}

const csv = () => new File(['asset_id\nX'], 'upload.csv', { type: 'text/csv' })

const committed: ImportResponse = {
  import_id: 'imp-1',
  total_rows: 222,
  imported_rows: 205,
  rejected_rows: 2,
  committed: true,
  ignored_columns: [],
  rejections: previewBody.rejections,
}

async function chooseAndCheck(user: ReturnType<typeof userEvent.setup>) {
  await user.upload(await screen.findByLabelText('Choose file'), csv())
  await user.click(screen.getByRole('button', { name: 'Check file' }))
}

describe('Import page: choosing a file', () => {
  it('replaces the dropzone with a chip that has a remove button', async () => {
    mockApi(emptyTree)
    const user = userEvent.setup()
    renderApp('/import')

    expect(await screen.findByText('Drag a CSV file here')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Check file' })).toBeDisabled()

    await user.upload(screen.getByLabelText('Choose file'), csv())

    expect(screen.getByText('upload.csv')).toBeInTheDocument()
    expect(screen.queryByText('Drag a CSV file here')).toBeNull() // dropzone is gone, so nothing can be dropped over it
    expect(screen.queryByLabelText('Choose file')).toBeNull()
    expect(screen.getByRole('button', { name: 'Check file' })).toBeEnabled()

    await user.click(screen.getByRole('button', { name: 'Remove selected file' }))
    expect(screen.getByText('Drag a CSV file here')).toBeInTheDocument()
    expect(screen.queryByText('upload.csv')).toBeNull()
    expect(screen.getByRole('button', { name: 'Check file' })).toBeDisabled()
  })

  it('refuses a non-CSV file client-side (UX only; the API re-validates)', async () => {
    const calls = mockApi(emptyTree)
    const user = userEvent.setup({ applyAccept: false })
    renderApp('/import')
    await user.upload(await screen.findByLabelText('Choose file'), new File(['x'], 'notes.txt', { type: 'text/plain' }))
    expect(await screen.findByText('Only .csv files can be imported.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Check file' })).toBeDisabled()
    expect(calls.some((c) => c.startsWith('POST'))).toBe(false)
  })

  it('shows the API error in the upload card and keeps the file so another can replace it', async () => {
    mockApi({
      ...emptyTree,
      'POST /api/imports/preview': Response.json({ error: 'malformed CSV', line: 7 }, { status: 422 }),
    })
    const user = userEvent.setup()
    renderApp('/import')
    await chooseAndCheck(user)
    expect(await screen.findByText(/malformed CSV \(line 7\)/)).toBeInTheDocument()
    expect(screen.getByText('upload.csv')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Check file' })).toBeEnabled()
  })
})

describe('import: check, review, confirm', () => {
  it('Check file only previews: nothing is stored until Import is pressed', async () => {
    const calls = mockApi({ ...emptyTree, 'POST /api/imports/preview': previewBody })
    const user = userEvent.setup()
    renderApp('/import')
    await chooseAndCheck(user)

    expect(await screen.findByText('Nothing has been saved yet')).toBeInTheDocument()
    expect(screen.getByText('Total rows').previousSibling).toHaveTextContent('222')
    expect(screen.getByText('Can be imported').previousSibling).toHaveTextContent('205')
    expect(screen.getByText('Will be rejected').previousSibling).toHaveTextContent('2')
    expect(screen.getByRole('button', { name: 'Import 205 valid rows' })).toBeEnabled()

    const rows = within(screen.getByRole('table', { name: /Rows rejected from/ })).getAllByRole('row').slice(1)
    expect(rows).toHaveLength(2)
    expect(within(rows[0]).getAllByRole('cell').map((c) => c.textContent)).toEqual(['178', 'TX-NR', 'rating_kva cannot be negative (-500)'])
    // ids never wrap: the cell is nowrap and its column is wide
    expect(within(rows[1]).getByText('PNL-001-1-01')).toHaveClass('whitespace-nowrap')

    expect(calls).toContain('POST /api/imports/preview')
    expect(calls).not.toContain('POST /api/imports')
  })

  it('Cancel discards the preview and the chosen file', async () => {
    const calls = mockApi({ ...emptyTree, 'POST /api/imports/preview': previewBody })
    const user = userEvent.setup()
    renderApp('/import')
    await chooseAndCheck(user)
    await user.click(await screen.findByRole('button', { name: 'Cancel' }))

    expect(screen.queryByText('Nothing has been saved yet')).toBeNull()
    expect(screen.getByText('Drag a CSV file here')).toBeInTheDocument()
    expect(calls).not.toContain('POST /api/imports')
  })

  it('Import sends the same file and the previewed fingerprint, then reports the result', async () => {
    let sent: FormData | undefined
    mockApi({
      ...emptyTree,
      'POST /api/imports/preview': previewBody,
      'POST /api/imports': (init?: RequestInit) => {
        sent = init?.body as FormData
        return committed
      },
    })
    const user = userEvent.setup()
    renderApp('/import')
    await chooseAndCheck(user)
    await user.click(await screen.findByRole('button', { name: 'Import 205 valid rows' }))

    expect(await screen.findByText(/Data committed: yes — 205 rows stored, 2 rejected rows were not saved/)).toBeInTheDocument()
    expect(sent).toBeDefined()
    expect(sent!.get('expected_fingerprint')).toBe('fp-1')
    expect((sent!.get('file') as File).name).toBe('upload.csv')
    expect(screen.getByRole('button', { name: /download report/i })).toBeInTheDocument()
    // the dropzone is back so another file can be chosen
    expect(screen.getByText('Drag a CSV file here')).toBeInTheDocument()
  })

  it('a stale fingerprint (412) re-checks the same file and shows the updated result with a notice', async () => {
    let previews = 0
    const calls = mockApi({
      ...emptyTree,
      'POST /api/imports/preview': () => (++previews === 1 ? previewBody : { ...previewBody, importable_rows: 190, rejected_rows: 17, fingerprint: 'fp-2' }),
      'POST /api/imports': Response.json({ error: "the file's validation result changed since the preview; review it again" }, { status: 412 }),
    })
    const user = userEvent.setup()
    renderApp('/import')
    await chooseAndCheck(user)
    await user.click(await screen.findByRole('button', { name: 'Import 205 valid rows' }))

    expect(await screen.findByText(/The data changed since you checked the file/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Import 190 valid rows' })).toBeInTheDocument()
    expect(calls.filter((c) => c === 'POST /api/imports/preview')).toHaveLength(2)
  })

  it('a lost race (409) says nothing was saved and offers Check again', async () => {
    mockApi({
      ...emptyTree,
      'POST /api/imports/preview': previewBody,
      'POST /api/imports': Response.json({ error: 'the stored data changed while the request was being processed; please retry' }, { status: 409 }),
    })
    const user = userEvent.setup()
    renderApp('/import')
    await chooseAndCheck(user)
    await user.click(await screen.findByRole('button', { name: 'Import 205 valid rows' }))

    expect(await screen.findByText(/Nothing was saved because the data changed while importing/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Check again' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Import 205/ })).toBeNull()
  })

  it('cannot import when no row is importable', async () => {
    mockApi({ ...emptyTree, 'POST /api/imports/preview': { ...previewBody, importable_rows: 0, rejected_rows: 222 } })
    const user = userEvent.setup()
    renderApp('/import')
    await chooseAndCheck(user)
    expect(await screen.findByRole('button', { name: 'Import 0 valid rows' })).toBeDisabled()
    expect(screen.getByText(/No row can be imported/)).toBeInTheDocument()
  })

  it('offers "Import all" when nothing is rejected', async () => {
    mockApi({ ...emptyTree, 'POST /api/imports/preview': { ...previewBody, importable_rows: 222, rejected_rows: 0, rejections: [] } })
    const user = userEvent.setup()
    renderApp('/import')
    await chooseAndCheck(user)
    expect(await screen.findByRole('button', { name: 'Import all 222 rows' })).toBeEnabled()
    expect(screen.queryByRole('table', { name: /Rows rejected from/ })).toBeNull()
  })

  it('reviews in place: the check never leaves /import and the history stays below', async () => {
    mockApi({ ...emptyTree, 'POST /api/imports/preview': previewBody })
    const user = userEvent.setup()
    renderApp('/import')
    await chooseAndCheck(user)
    await screen.findByText('Nothing has been saved yet')
    expect(screen.getByRole('button', { name: 'Checked — review below' })).toBeDisabled()
    expect(screen.getByTestId('location')).toHaveTextContent('/import')
    expect(screen.getByRole('link', { name: /^Import history/ })).toHaveAttribute('href', '#import-history')
    expect(await screen.findByRole('heading', { name: 'Import history' })).toBeInTheDocument()
  })

  it('keeps the chosen file when you leave the page and come back', async () => {
    mockApi(emptyTree)
    const user = userEvent.setup()
    renderApp('/import')
    await user.upload(await screen.findByLabelText('Choose file'), csv())
    await user.click(screen.getByRole('link', { name: 'Grid Asset Explorer' }))
    expect(await screen.findByRole('heading', { name: 'Overview' })).toBeInTheDocument()
    await user.click(screen.getAllByRole('link', { name: 'Import' })[0])
    expect(await screen.findByText('upload.csv')).toBeInTheDocument()
  })
})
