import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import type { Backend } from './api'
import { FolderExplorer } from './FolderExplorer'
import { UIProvider } from './components/ui'
afterEach(() => {
  cleanup()
  delete window.go
  vi.restoreAllMocks()
})
it('loads folders on demand and opens a file by its root-relative path', async () => {
  const api = {
    ListWorkspaceDirectory: vi.fn().mockImplementation(async (_project, path) =>
      path === '.'
        ? [
            { name: 'docs', directory: true },
            { name: 'notes.txt', directory: false },
          ]
        : [{ name: 'plan.md', directory: false }],
    ),
  }
  window.go = { main: { App: api as unknown as Backend } }
  const open = vi.fn()
  render(
    <UIProvider>
      <FolderExplorer project="project" onOpen={open} />
    </UIProvider>,
  )
  const docs = await screen.findByRole('button', { name: 'docs' })
  expect(api.ListWorkspaceDirectory).not.toHaveBeenCalledWith('project', 'docs')
  fireEvent.click(docs)
  fireEvent.click(await screen.findByRole('button', { name: 'plan.md' }))
  expect(open).toHaveBeenCalledWith('docs/plan.md')
  fireEvent.keyDown(docs, { key: 'ArrowLeft' })
  expect(docs.getAttribute('aria-expanded')).toBe('false')
  expect(screen.queryByRole('button', { name: 'plan.md' })).toBeNull()
})
it('reports an unavailable folder and can retry without clearing the project', async () => {
  const api = {
    ListWorkspaceDirectory: vi
      .fn()
      .mockRejectedValueOnce(new Error('Folder unavailable'))
      .mockResolvedValue([]),
  }
  window.go = { main: { App: api as unknown as Backend } }
  render(
    <UIProvider>
      <FolderExplorer project="project" onOpen={vi.fn()} />
    </UIProvider>,
  )
  await screen.findByText('Folder unavailable')
  fireEvent.click(screen.getByRole('button', { name: 'Refresh project files' }))
  await screen.findByText('No files in this folder.')
  await waitFor(() => expect(screen.queryByText('Folder unavailable')).toBeNull())
})
