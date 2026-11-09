import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { EditorBoundary } from './EditorBoundary'

let crash = false
function Editor() {
  if (crash) throw new Error('Synthetic editor failure')
  return <p>Working document</p>
}
afterEach(() => {
  cleanup()
  crash = false
  vi.restoreAllMocks()
})
it('keeps the conversation visible and remounts a failed editor on retry', () => {
  vi.spyOn(console, 'error').mockImplementation(() => {})
  crash = true
  render(
    <>
      <p>Conversation remains available</p>
      <EditorBoundary close={vi.fn()}>
        <Editor />
      </EditorBoundary>
    </>,
  )
  expect(screen.getByRole('alert').textContent).toContain('The editor stopped unexpectedly')
  expect(screen.getByText('Conversation remains available')).toBeTruthy()
  crash = false
  fireEvent.click(screen.getByRole('button', { name: 'Retry editor' }))
  expect(screen.getByText('Working document')).toBeTruthy()
})
it('can close a failed editor without discarding stored content', () => {
  vi.spyOn(console, 'error').mockImplementation(() => {})
  crash = true
  const close = vi.fn().mockResolvedValue(undefined)
  render(
    <EditorBoundary close={close}>
      <Editor />
    </EditorBoundary>,
  )
  fireEvent.click(screen.getByRole('button', { name: 'Return to chat' }))
  expect(close).toHaveBeenCalledTimes(1)
})
