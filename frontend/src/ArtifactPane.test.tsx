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
    Attachments: vi.fn().mockResolvedValue([]),
  }
  window.go = { main: { App: api as unknown as Backend } }
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
it('retains an artifact draft when a revision conflict prevents saving', async () => {
  const { api, close } = setup()
  api.SaveArtifact.mockRejectedValue(new Error('The artifact changed'))
  const cell = await screen.findByRole('textbox', { name: 'Budget A1' })
  fireEvent.change(cell, { target: { value: '30' } })
  fireEvent.click(screen.getByRole('button', { name: 'Close artifact' }))
  await screen.findByRole('alert')
  expect(close).not.toHaveBeenCalled()
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
