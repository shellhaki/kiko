import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { SourceStatus } from './SourceStatus'
import { UIProvider } from './components/ui'
import type { Backend, Run, WorkspaceSource } from './api'
afterEach(() => {
  cleanup()
  delete window.go
  vi.restoreAllMocks()
})
it('shows both versions and reloads only on an explicit guarded action', async () => {
  const source: WorkspaceSource = {
    id: 'doc',
    chatId: 'chat',
    kind: 'document',
    title: 'Plan',
    path: 'docs/plan.md',
    rev: 4,
    content: '# Kiko copy',
    diskContent: '# External copy',
    diskHash: 'current-hash',
    status: 'changed',
    issue: '',
    draft: false,
    running: false,
  }
  const api = {
    WorkspaceSource: vi.fn().mockResolvedValue(source),
    ReloadWorkspaceSource: vi.fn().mockResolvedValue({}),
  }
  window.go = { main: { App: api as unknown as Backend } }
  const run: Run = async (operation) => {
    await operation()
    return true
  }
  render(
    <UIProvider>
      <SourceStatus id="doc" run={run} />
    </UIProvider>,
  )
  fireEvent.click(await screen.findByRole('button', { name: 'Compare external changes' }))
  await screen.findByRole('dialog', { name: 'Compare workspace versions' })
  expect(screen.getByText('# Kiko copy')).toBeTruthy()
  expect(screen.getByText('# External copy')).toBeTruthy()
  expect(api.ReloadWorkspaceSource).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: 'Load folder version' }))
  await waitFor(() =>
    expect(api.ReloadWorkspaceSource).toHaveBeenCalledWith('doc', 'current-hash', 4),
  )
})
it('keeps the comparison open and reports a stale-file failure', async () => {
  const source = {
    id: 'doc',
    chatId: 'chat',
    kind: 'document',
    path: 'plan.md',
    rev: 1,
    content: 'Local',
    diskContent: 'Disk',
    diskHash: 'hash',
    status: 'changed',
  }
  const api = {
    WorkspaceSource: vi.fn().mockResolvedValue(source),
    ReloadWorkspaceSource: vi.fn().mockRejectedValue(new Error('File changed again')),
  }
  window.go = { main: { App: api as unknown as Backend } }
  const run: Run = async (operation) => {
    try {
      await operation()
      return true
    } catch {
      return false
    }
  }
  render(
    <UIProvider>
      <SourceStatus id="doc" run={run} />
    </UIProvider>,
  )
  fireEvent.click(await screen.findByRole('button', { name: 'Compare external changes' }))
  await screen.findByRole('dialog')
  fireEvent.click(screen.getByRole('button', { name: 'Load folder version' }))
  await screen.findAllByText('Error: File changed again')
  expect(screen.getByRole('dialog')).toBeTruthy()
})
