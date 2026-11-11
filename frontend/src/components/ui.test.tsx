import { afterEach, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { CopyButton, Select, UIProvider } from './ui'

function renderCopy(text: string) {
  render(
    <UIProvider>
      <CopyButton text={text} label="Copy message" />
    </UIProvider>,
  )
}

afterEach(() => {
  cleanup()
  delete (navigator as unknown as Record<string, unknown>).clipboard
  vi.useRealTimers()
})

const options = [
  { value: 'title', label: 'Title' },
  { value: 'bullets', label: 'Bullets' },
  { value: 'chart', label: 'Chart', disabled: true },
]

it('shows the current option and reports a new choice', async () => {
  const onChange = vi.fn()
  render(<Select label="Layout" value="bullets" onChange={onChange} options={options} />)
  expect(screen.getByRole('button', { name: 'Layout' }).textContent).toContain('Bullets')
  fireEvent.pointerDown(screen.getByRole('button', { name: 'Layout' }))
  fireEvent.click(await screen.findByRole('menuitem', { name: 'Title' }))
  expect(onChange).toHaveBeenCalledWith('title')
})

it('respects disabled state and disabled options', async () => {
  const onChange = vi.fn()
  render(<Select label="Layout" value="title" onChange={onChange} options={options} disabled />)
  expect(screen.getByRole('button', { name: 'Layout' }).hasAttribute('disabled')).toBe(true)
})

it('copies text and confirms transiently', async () => {
  vi.useFakeTimers()
  try {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.assign(navigator, { clipboard: { writeText } })
    renderCopy('hello')
    fireEvent.click(screen.getByRole('button', { name: 'Copy message' }))
    await act(async () => {})
    expect(writeText).toHaveBeenCalledWith('hello')
    expect(screen.getByRole('button', { name: 'Copied' })).toBeTruthy()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1600)
    })
    expect(screen.getByRole('button', { name: 'Copy message' })).toBeTruthy()
  } finally {
    vi.useRealTimers()
  }
})

it('disables copying without text and stays quiet on failure', async () => {
  const writeText = vi.fn().mockRejectedValue(new Error('denied'))
  Object.assign(navigator, { clipboard: { writeText } })
  renderCopy('')
  expect(screen.getByRole('button', { name: 'Copy message' }).hasAttribute('disabled')).toBe(true)
})
