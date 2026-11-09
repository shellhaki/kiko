import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { PDFPageImage } from './PDFPageImage'
import { UIProvider } from './components/ui'
import type { Backend } from './api'
afterEach(() => {
  cleanup()
  delete window.go
})
it('renders selected pages through the scoped local worker without inference', async () => {
  const image = 'data:image/png;base64,iVBORw0KGgo='
  const api = { RenderAttachmentPage: vi.fn().mockResolvedValue(image) }
  window.go = { main: { App: api as unknown as Backend } }
  render(
    <UIProvider>
      <PDFPageImage chat="chat" id="pdf" pages={2} />
    </UIProvider>,
  )
  expect((await screen.findByRole('img', { name: 'PDF page 1' })).getAttribute('src')).toBe(image)
  expect(api.RenderAttachmentPage).toHaveBeenCalledWith('chat', 'pdf', 1)
  fireEvent.click(screen.getByRole('button', { name: 'Zoom in page image' }))
  expect(screen.getByLabelText('Page image zoom').textContent).toBe('125%')
  expect(api.RenderAttachmentPage).toHaveBeenCalledTimes(1)
  fireEvent.click(screen.getByRole('button', { name: 'Next page' }))
  await screen.findByRole('img', { name: 'PDF page 2' })
  expect(api.RenderAttachmentPage).toHaveBeenLastCalledWith('chat', 'pdf', 2)
  expect((screen.getByRole('button', { name: 'Next page' }) as HTMLButtonElement).disabled).toBe(
    true,
  )
})
it('reports a failed local page without crashing the preview', async () => {
  const renderPage = vi
    .fn()
    .mockRejectedValueOnce(new Error('Page unavailable'))
    .mockResolvedValue('data:image/png;base64,iVBORw0KGgo=')
  window.go = { main: { App: { RenderAttachmentPage: renderPage } as unknown as Backend } }
  render(
    <UIProvider>
      <PDFPageImage chat="chat" id="pdf" pages={1} />
    </UIProvider>,
  )
  expect((await screen.findByRole('alert')).textContent).toContain('Page unavailable')
  fireEvent.click(screen.getByRole('button', { name: 'Retry page' }))
  await screen.findByRole('img', { name: 'PDF page 1' })
})
