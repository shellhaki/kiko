import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DeleteApproval } from './DeleteApproval'
import type { Backend } from './api'

const api = {
  FilePermissions: vi.fn(),
  ResolveFilePermission: vi.fn(),
}

const rows = [{ id: 'd1', chatId: 'c1', path: 'docs/old.md', content: '', kind: 'delete' }]

beforeEach(() => {
  window.go = { main: { App: api as unknown as Backend } }
  api.FilePermissions.mockReset().mockResolvedValue(rows)
  api.ResolveFilePermission.mockReset().mockResolvedValue(undefined)
})

afterEach(() => {
  cleanup()
  delete window.go
})

describe('DeleteApproval', () => {
  it('shows the pending delete path', async () => {
    render(<DeleteApproval chat="c1" />)
    expect(await screen.findByText('Delete this file?')).toBeTruthy()
    expect(screen.getByText('docs/old.md')).toBeTruthy()
  })

  it('shows document deletions with document copy', async () => {
    api.FilePermissions.mockResolvedValue([
      { id: 'doc1', chatId: 'c1', path: 'Old Plan', content: 'doc-id', kind: 'document' },
    ])
    render(<DeleteApproval chat="c1" />)
    expect(await screen.findByText('Delete this document?')).toBeTruthy()
    expect(screen.getByText('Old Plan')).toBeTruthy()
    fireEvent.click(await screen.findByRole('button', { name: 'Delete document' }))
    expect(api.ResolveFilePermission).toHaveBeenCalledWith('doc1', true)
  })

  it('hides legacy non-delete requests', async () => {
    api.FilePermissions.mockResolvedValue([
      { id: 'w1', chatId: 'c1', path: 'docs/a.md', content: 'x', kind: 'write' },
    ])
    render(<DeleteApproval chat="c1" />)
    await waitFor(() => expect(api.FilePermissions).toHaveBeenCalled())
    expect(screen.queryByText('Delete this file?')).toBeNull()
  })

  it('renders nothing without requests', async () => {
    api.FilePermissions.mockResolvedValue([])
    const { container } = render(<DeleteApproval chat="c1" />)
    await waitFor(() => expect(api.FilePermissions).toHaveBeenCalled())
    expect(container.firstChild).toBeNull()
  })

  it('approves the deletion', async () => {
    render(<DeleteApproval chat="c1" />)
    fireEvent.click(await screen.findByRole('button', { name: 'Delete file' }))
    expect(api.ResolveFilePermission).toHaveBeenCalledWith('d1', true)
  })

  it('keeps the file on deny', async () => {
    render(<DeleteApproval chat="c1" />)
    fireEvent.click(await screen.findByRole('button', { name: 'Keep' }))
    expect(api.ResolveFilePermission).toHaveBeenCalledWith('d1', false)
  })
})
