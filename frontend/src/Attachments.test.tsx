import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { Attachments } from './Attachments'
import { UIProvider } from './components/ui'
import type { Backend } from './api'
vi.mock('./PDFViewer', () => ({
  default: () => {
    throw new Error('Browser PDF module unavailable')
  },
}))
afterEach(() => {
  cleanup()
  delete window.go
  vi.restoreAllMocks()
})
it('imports locally, lists the file, and opens extracted text without sending a message', async () => {
  const file = {
    id: 'file',
    chatId: 'chat',
    name: 'notes.txt',
    mime: 'text/plain',
    size: 8,
    pages: 1,
  }
  const api = {
    Attachments: vi.fn().mockResolvedValue([]),
    AttachFile: vi.fn().mockImplementation(async () => {
      api.Attachments.mockResolvedValue([file] as never[])
      return file
    }),
    ReadAttachment: vi.fn().mockResolvedValue([{ name: 'Text', text: 'Local source text' }]),
    AttachmentPreview: vi.fn().mockResolvedValue(''),
  }
  window.go = { main: { App: api as unknown as Backend } }
  const submit = vi.fn()
  render(
    <UIProvider>
      <form
        onSubmit={(event) => {
          event.preventDefault()
          submit()
        }}
      >
        <textarea defaultValue="A draft that must not be sent" />
        <Attachments chat="chat" disabled={false} />
      </form>
    </UIProvider>,
  )
  fireEvent.change(screen.getByLabelText('Choose attachment'), {
    target: { files: [new File(['contents'], 'notes.txt', { type: 'text/plain' })] },
  })
  await screen.findByRole('button', { name: /^notes.txt/ })
  expect(api.AttachFile).toHaveBeenCalledWith('chat', 'notes.txt', expect.any(String))
  fireEvent.click(screen.getByRole('button', { name: /^notes.txt/ }))
  await waitFor(() =>
    expect(document.querySelector('.attachment-text')?.textContent).toContain('Local source text'),
  )
  expect(submit).not.toHaveBeenCalled()
})

it('recovers a failed browser PDF module through local page images', async () => {
  vi.spyOn(console, 'error').mockImplementation(() => {})
  const file = {
    id: 'pdf',
    chatId: 'chat',
    name: 'Source.pdf',
    mime: 'application/pdf',
    size: 100,
    pages: 2,
  }
  const api = {
    Attachments: vi.fn().mockResolvedValue([file]),
    ReadAttachment: vi.fn().mockResolvedValue([{ name: 'Page 1', text: 'Source text' }]),
    AttachmentPreview: vi.fn().mockResolvedValue(''),
    AttachmentBytes: vi.fn().mockResolvedValue('fixture'),
    RenderAttachmentPage: vi.fn().mockResolvedValue('data:image/png;base64,iVBORw0KGgo='),
  }
  window.go = { main: { App: api as unknown as Backend } }
  render(
    <UIProvider>
      <Attachments chat="chat" disabled={false} />
    </UIProvider>,
  )
  fireEvent.click(await screen.findByRole('button', { name: /^Source.pdf/ }))
  expect((await screen.findByRole('alert')).textContent).toContain('Use local page images')
  fireEvent.click(screen.getByRole('button', { name: 'Use local page images' }))
  await screen.findByRole('img', { name: 'PDF page 1' })
  expect(api.RenderAttachmentPage).toHaveBeenCalledWith('chat', 'pdf', 1)
})
