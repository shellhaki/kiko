import { afterEach, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { WorkspaceFilePane } from './WorkspaceFilePane'
import { UIProvider } from './components/ui'
import type { Backend, WorkspaceEntry } from './api'
import { flushDocument } from './document-session'
afterEach(() => {
  cleanup()
  delete window.go
  vi.restoreAllMocks()
  vi.useRealTimers()
})
const entry: WorkspaceEntry = {
  path: 'notes.txt',
  content: 'Original',
  hash: 'old-hash',
  sourceId: '',
  chatId: 'chat',
  kind: '',
}
function setup(initial = entry) {
  const api = {
    ReadWorkspaceEntry: vi.fn().mockResolvedValue(initial),
    SaveWorkspaceTextDraft: vi.fn().mockResolvedValue(undefined),
    SaveWorkspaceText: vi
      .fn()
      .mockResolvedValue({ ...entry, content: 'Changed', hash: 'new-hash' }),
    DiscardWorkspaceTextDraft: vi.fn().mockResolvedValue(undefined),
  }
  window.go = { main: { App: api as unknown as Backend } }
  const view = render(
    <UIProvider>
      <WorkspaceFilePane
        project="project"
        chat="chat"
        path="notes.txt"
        close={vi.fn()}
        run={vi.fn()}
        onImported={vi.fn()}
      />
    </UIProvider>,
  )
  return { api, view }
}
it('recovers and persists text drafts and flushes a hash-guarded save on navigation', async () => {
  const { api } = setup({ ...entry, draft: { baseHash: 'old-hash', content: 'Recovered' } })
  const input = (await screen.findByRole('textbox', { name: 'File editor' })) as HTMLTextAreaElement
  expect(input.value).toBe('Recovered')
  fireEvent.change(input, { target: { value: 'Changed' } })
  await act(async () => {
    await flushDocument()
  })
  expect(api.SaveWorkspaceTextDraft).toHaveBeenCalledWith(
    'project',
    'notes.txt',
    'old-hash',
    'Changed',
  )
  expect(api.SaveWorkspaceText).toHaveBeenCalledWith(
    'project',
    'chat',
    'notes.txt',
    'old-hash',
    'Changed',
  )
  await screen.findByText('Saved')
})
it('keeps conflicting input and loads the external version only after comparison', async () => {
  const { api } = setup({
    ...entry,
    hash: 'changed-on-disk',
    draft: { baseHash: 'old-hash', content: 'Recovered' },
  })
  const input = (await screen.findByRole('textbox', { name: 'File editor' })) as HTMLTextAreaElement
  api.SaveWorkspaceText.mockRejectedValue(new Error('File changed outside Kiko'))
  fireEvent.change(input, { target: { value: 'My local work' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save file' }))
  await screen.findByRole('alert')
  expect(input.value).toBe('My local work')
  api.ReadWorkspaceEntry.mockResolvedValue({
    ...entry,
    content: 'External work',
    hash: 'external-hash',
  })
  fireEvent.click(await screen.findByRole('button', { name: 'Compare versions' }))
  await screen.findByRole('dialog', { name: 'Compare file versions' })
  expect(api.DiscardWorkspaceTextDraft).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: 'Load folder version' }))
  await waitFor(() =>
    expect(api.DiscardWorkspaceTextDraft).toHaveBeenCalledWith(
      'project',
      'notes.txt',
      'external-hash',
    ),
  )
  expect(input.value).toBe('External work')
})
