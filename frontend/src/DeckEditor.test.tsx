import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { DeckEditor } from './DeckEditor'
import { UIProvider } from './components/ui'
import type { Backend } from './api'
import type { Deck } from './artifact-model'
afterEach(() => {
  cleanup()
  delete window.go
})
it('keeps editing the remaining slide when another writer removes the selected slide', async () => {
  window.go = {
    main: { App: { Attachments: vi.fn().mockResolvedValue([]) } as unknown as Backend },
  }
  const onChange = vi.fn()
  const initial: Deck = {
    slides: [
      { id: 'one', layout: 'title', title: 'First' },
      { id: 'two', layout: 'title', title: 'Second' },
    ],
  }
  const view = render(
    <UIProvider>
      <DeckEditor deck={initial} chat="chat" onChange={onChange} />
    </UIProvider>,
  )
  fireEvent.click(screen.getByRole('button', { name: /Second/ }))
  view.rerender(
    <UIProvider>
      <DeckEditor deck={{ slides: [initial.slides[0]] }} chat="chat" onChange={onChange} />
    </UIProvider>,
  )
  fireEvent.change(screen.getByLabelText('Slide title'), { target: { value: 'Remaining slide' } })
  expect(onChange).toHaveBeenCalledWith({
    slides: [{ id: 'one', layout: 'title', title: 'Remaining slide' }],
  })
})
it('previews every slide layout with the shared export theme', async () => {
  window.go = {
    main: {
      App: {
        Attachments: vi.fn().mockResolvedValue([{ id: 'img', name: 'art.png', mime: 'image/png' }]),
        AttachmentPreview: vi.fn().mockResolvedValue('data:image/png;base64,preview'),
      } as unknown as Backend,
    },
  }
  const deck: Deck = {
    slides: [
      { id: 'one', layout: 'title', title: 'First', body: ['Subtitle point'] },
      { id: 'two', layout: 'bullets', title: 'Points', body: ['Alpha', 'Beta'] },
      { id: 'three', layout: 'columns', title: 'Split', left: ['West'], right: ['East'] },
      { id: 'four', layout: 'image', title: 'Art', imageId: 'img', body: ['Caption line'] },
      {
        id: 'five',
        layout: 'chart',
        title: 'Reach',
        chart: { labels: ['2024', '2025'], values: [40, 90] },
      },
    ],
  }
  render(
    <UIProvider>
      <DeckEditor deck={deck} chat="chat" onChange={vi.fn()} />
    </UIProvider>,
  )
  const preview = screen.getByLabelText('Slide 1 preview')
  expect(preview.className).toContain('layout-title')
  expect(preview.querySelector('.slide-strip')).toBeTruthy()
  expect(preview.querySelector('.slide-title')?.textContent).toBe('First')
  expect(preview.textContent).toContain('Subtitle point')
  fireEvent.click(screen.getByRole('button', { name: /Points/ }))
  const bullets = screen.getByLabelText('Slide 2 preview')
  expect(bullets.textContent).toContain('Alpha')
  expect(bullets.textContent).toContain('Beta')
  fireEvent.click(screen.getByRole('button', { name: /Split/ }))
  const columns = screen.getByLabelText('Slide 3 preview')
  expect(columns.textContent).toContain('West')
  expect(columns.textContent).toContain('East')
  fireEvent.click(screen.getByRole('button', { name: /Art/ }))
  const image = await screen.findByLabelText('Slide 4 preview')
  expect(image.textContent).toContain('Caption line')
  expect(image.querySelector('img')).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: /Reach/ }))
  const chart = screen.getByLabelText('Slide 5 preview')
  expect(chart.textContent).toContain('2025')
  expect(chart.textContent).toContain('90')
  expect(chart.querySelectorAll('.slide-chart > div')).toHaveLength(2)
  expect(chart.querySelector('.slide-number')?.textContent).toBe('5')
})
