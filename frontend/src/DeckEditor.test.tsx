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
