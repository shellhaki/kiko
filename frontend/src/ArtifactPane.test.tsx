import { StrictMode } from 'react'
import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { ArtifactPane } from './ArtifactPane'
import type { ArtifactRecord, Workbook } from './artifact-model'
import type { Backend } from './api'
import { UIProvider } from './components/ui'
const initial: ArtifactRecord = {
  id: 'sheet',
  chatId: 'chat',
  kind: 'sheet',
  title: 'Budget',
  rev: 1,
  payload: { tabs: [{ name: 'Budget', cells: { A1: { value: 10 }, A2: { formula: '=A1*2' } } }] },
  values: { Budget: { A1: { value: '10' }, A2: { value: '20' } } },
}
function setup(record = initial) {
  const api = {
    ReadArtifact: vi.fn().mockResolvedValue(structuredClone(record)),
    SaveArtifactDraft: vi.fn().mockResolvedValue(undefined),
    SaveArtifact: vi
      .fn()
      .mockImplementation(async (_id, rev, payload) => ({ ...record, rev: rev + 1, payload })),
    ExportArtifact: vi.fn().mockResolvedValue(true),
    ExportDeckPptx: vi.fn().mockResolvedValue(true),
    Attachments: vi.fn().mockResolvedValue([]),
  }
  window.go = { main: { App: api as unknown as Backend } }
  Object.assign(navigator, { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } })
  const close = vi.fn().mockResolvedValue(undefined)
  render(
    <StrictMode>
      <UIProvider>
        <ArtifactPane id={record.id} close={close} />
      </UIProvider>
    </StrictMode>,
  )
  return { api, close }
}
afterEach(() => {
  cleanup()
  delete window.go
  delete (navigator as unknown as Record<string, unknown>).clipboard
  vi.restoreAllMocks()
})
it('edits a spreadsheet, persists its draft, and flushes before closing', async () => {
  const { api, close } = setup()
  const cell = await screen.findByRole('textbox', { name: 'Budget A1' })
  fireEvent.change(cell, { target: { value: '25' } })
  fireEvent.click(screen.getByRole('button', { name: 'Close artifact' }))
  await waitFor(() => expect(close).toHaveBeenCalledTimes(1))
  expect(api.SaveArtifactDraft).toHaveBeenCalled()
  const payload = api.SaveArtifact.mock.calls[0][2] as Workbook
  expect(payload.tabs[0].cells.A1.value).toBe(25)
})
it('confirms copying the draft to the clipboard', async () => {
  setup()
  await screen.findByRole('button', { name: 'Copy artifact draft' })
  fireEvent.click(screen.getByRole('button', { name: 'Copy artifact draft' }))
  await waitFor(() => expect(navigator.clipboard.writeText).toHaveBeenCalled())
  expect(await screen.findByText(/Draft copied/)).toBeTruthy()
  expect(screen.queryByRole('alert')).toBeNull()
})
it('shows preparing state while an export runs', async () => {
  const { api } = setup()
  let resolve!: (done: boolean) => void
  api.ExportArtifact.mockImplementation(() => new Promise<boolean>((done) => void (resolve = done)))
  fireEvent.click(await screen.findByRole('button', { name: 'Export XLSX' }))
  expect(await screen.findByText(/Preparing export/)).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Export XLSX' }).hasAttribute('disabled')).toBe(true)
  resolve(true)
  await waitFor(() => expect(screen.queryByText(/Preparing export/)).toBeNull())
  expect(await screen.findByText(/Exported/)).toBeTruthy()
})
it('exports slides through the editor preview instead of the server', async () => {
  const { api } = setup({
    ...initial,
    id: 'deck',
    kind: 'deck',
    title: 'Slides',
    payload: { slides: [{ id: 'one', layout: 'bullets', title: 'Goals', body: ['Ship it'] }] },
    values: undefined,
  })
  fireEvent.click(await screen.findByRole('button', { name: 'Export PPTX' }))
  await waitFor(() => expect(api.ExportDeckPptx).toHaveBeenCalledTimes(1))
  expect(api.ExportDeckPptx.mock.calls[0][0]).toBe('deck')
  expect(atob(api.ExportDeckPptx.mock.calls[0][1] as string).slice(0, 2)).toBe('PK')
  expect(api.ExportArtifact).not.toHaveBeenCalled()
})
it('closes a conflicted pane because the draft is already persisted', async () => {
  const { api, close } = setup({ ...initial, draft: { baseRev: 1, payload: initial.payload } })
  api.SaveArtifact.mockRejectedValue(new Error('The artifact changed'))
  fireEvent.click(await screen.findByRole('button', { name: 'Close artifact' }))
  await waitFor(() => expect(close).toHaveBeenCalledTimes(1))
})
it('closes a conflicted pane after persisting the draft for recovery', async () => {
  const { api, close } = setup()
  api.SaveArtifact.mockRejectedValue(new Error('The artifact changed'))
  const cell = await screen.findByRole('textbox', { name: 'Budget A1' })
  fireEvent.change(cell, { target: { value: '30' } })
  fireEvent.click(screen.getByRole('button', { name: 'Close artifact' }))
  await screen.findByRole('alert')
  await waitFor(() => expect(close).toHaveBeenCalledTimes(1))
  expect(api.SaveArtifactDraft).toHaveBeenCalled()
  const draft = api.SaveArtifactDraft.mock.calls[0][2] as Workbook
  expect(draft.tabs[0].cells.A1.value).toBe(30)
})
it('hides retry when another writer moved the saved revision', async () => {
  const { api } = setup()
  let advanced = false
  api.ReadArtifact.mockImplementation(async () => ({
    ...structuredClone(initial),
    rev: advanced ? 2 : 1,
  }))
  api.SaveArtifact.mockRejectedValue(new Error('The artifact changed'))
  const cell = await screen.findByRole('textbox', { name: 'Budget A1' })
  fireEvent.change(cell, { target: { value: '30' } })
  advanced = true
  fireEvent.click(screen.getByRole('button', { name: 'Close artifact' }))
  await screen.findByRole('alert')
  expect(screen.queryByRole('button', { name: 'Retry save' })).toBeNull()
  expect(screen.getByRole('button', { name: 'Discard draft and reload' })).toBeTruthy()
  expect((cell as HTMLInputElement).value).toBe('30')
})
it('opens and edits a slide deck through the same guarded save flow', async () => {
  const deck: ArtifactRecord = {
    id: 'deck',
    chatId: 'chat',
    kind: 'deck',
    title: 'Slides',
    rev: 1,
    payload: {
      slides: [{ id: 'slide1', layout: 'title', title: 'Original', body: ['Start here'] }],
    },
  }
  const { api, close } = setup(deck)
  const title = await screen.findByRole('textbox', { name: 'Slide title' })
  fireEvent.change(title, { target: { value: 'Updated title' } })
  fireEvent.click(screen.getByRole('button', { name: 'Close artifact' }))
  await waitFor(() => expect(close).toHaveBeenCalledTimes(1))
  expect(api.SaveArtifact.mock.calls[0][2]).toMatchObject({
    slides: [{ id: 'slide1', title: 'Updated title' }],
  })
})
it('discards a conflicting draft only after confirmation and reloads the saved version', async () => {
  const { api, close } = setup()
  const discard = vi.fn().mockResolvedValue(undefined)
  Object.assign(api, { DiscardArtifactDraft: discard })
  vi.spyOn(window, 'confirm').mockReturnValue(true)
  api.SaveArtifact.mockRejectedValue(new Error('The artifact changed'))
  fireEvent.change(await screen.findByRole('textbox', { name: 'Budget A1' }), {
    target: { value: '99' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Close artifact' }))
  fireEvent.click(await screen.findByRole('button', { name: 'Discard draft and reload' }))
  await waitFor(() => expect(discard).toHaveBeenCalledWith('sheet'))
  await waitFor(() =>
    expect((screen.getByRole('textbox', { name: 'Budget A1' }) as HTMLInputElement).value).toBe(
      '10',
    ),
  )
  fireEvent.click(screen.getByRole('button', { name: 'Close artifact' }))
  await waitFor(() => expect(close).toHaveBeenCalledTimes(1))
})
