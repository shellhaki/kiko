import { useState } from 'react'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { usePaneResize } from './use-pane-resize'

function Pane({ save }: { save(value: number): Promise<void> }) {
  const [saved, setSaved] = useState(260)
  const resize = usePaneResize({
    value: saved,
    min: 220,
    max: 320,
    measure: (handle, startX) => {
      const initial = Number(handle.getAttribute('aria-valuenow'))
      return (x) => initial + x - startX
    },
    commit: async (value) => {
      await save(value)
      setSaved(value)
    },
  })
  return (
    <div
      role="separator"
      aria-valuenow={resize.value}
      data-resizing={resize.active}
      {...resize.handlers}
      onKeyDown={(event) => {
        if (event.key === 'Escape') resize.cancel()
      }}
    />
  )
}
function setup(save = vi.fn().mockResolvedValue(undefined)) {
  class TestPointerEvent extends MouseEvent {
    pointerId: number
    constructor(type: string, init: PointerEventInit = {}) {
      super(type, init)
      this.pointerId = init.pointerId ?? 1
    }
  }
  vi.stubGlobal('PointerEvent', TestPointerEvent)
  const frames = new Map<number, FrameRequestCallback>()
  let next = 0
  vi.stubGlobal(
    'requestAnimationFrame',
    vi.fn((callback: FrameRequestCallback) => {
      frames.set(++next, callback)
      return next
    }),
  )
  vi.stubGlobal(
    'cancelAnimationFrame',
    vi.fn((id: number) => frames.delete(id)),
  )
  const view = render(<Pane save={save} />)
  const handle = screen.getByRole('separator')
  let captured = false
  handle.setPointerCapture = vi.fn(() => {
    captured = true
  })
  handle.hasPointerCapture = () => captured
  handle.releasePointerCapture = vi.fn(() => {
    captured = false
  })
  const pointer = (
    kind: 'pointerDown' | 'pointerMove' | 'pointerUp' | 'pointerCancel',
    x: number,
    pointerId = 1,
  ) => fireEvent[kind](handle, { pointerId, button: 0, buttons: 1, clientX: x })
  const frame = () =>
    act(() => {
      const callbacks = [...frames.values()]
      frames.clear()
      callbacks.forEach((callback) => callback(0))
    })
  return { handle, save, pointer, frame, frames, view }
}
afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

it('coalesces pointer moves per frame and persists the actual release position once', async () => {
  const { handle, save, pointer, frame, frames } = setup()
  pointer('pointerDown', 100)
  pointer('pointerMove', 110)
  pointer('pointerMove', 130)
  expect(frames.size).toBe(1)
  expect(handle.getAttribute('aria-valuenow')).toBe('260')
  frame()
  expect(handle.getAttribute('aria-valuenow')).toBe('290')
  expect(save).not.toHaveBeenCalled()
  pointer('pointerMove', 140)
  await act(async () => {
    pointer('pointerUp', 147)
  })
  expect(save).toHaveBeenCalledExactlyOnceWith(307)
  expect(frames.size).toBe(0)
  expect(handle.getAttribute('aria-valuenow')).toBe('307')
})
it('clamps sizes and rolls back cancellation without saving', () => {
  const { handle, save, pointer, frame } = setup()
  pointer('pointerDown', 100)
  pointer('pointerMove', 500)
  frame()
  expect(handle.getAttribute('aria-valuenow')).toBe('320')
  pointer('pointerMove', -500)
  frame()
  expect(handle.getAttribute('aria-valuenow')).toBe('220')
  pointer('pointerCancel', -500)
  expect(handle.getAttribute('aria-valuenow')).toBe('260')
  expect(save).not.toHaveBeenCalled()
  expect(handle.releasePointerCapture).toHaveBeenCalledWith(1)
})
it('ignores other pointers and supports Escape during a drag', () => {
  const { handle, pointer, frame, save } = setup()
  pointer('pointerDown', 100)
  pointer('pointerMove', 140, 2)
  frame()
  expect(handle.getAttribute('aria-valuenow')).toBe('260')
  pointer('pointerMove', 120)
  frame()
  fireEvent.keyDown(handle, { key: 'Escape' })
  expect(handle.getAttribute('aria-valuenow')).toBe('260')
  expect(save).not.toHaveBeenCalled()
})
it('releases capture and cancels pending frames when a pane closes', () => {
  const { handle, pointer, view, frames, save } = setup()
  pointer('pointerDown', 100)
  pointer('pointerMove', 120)
  view.unmount()
  expect(frames.size).toBe(0)
  expect(handle.releasePointerCapture).toHaveBeenCalledWith(1)
  expect(save).not.toHaveBeenCalled()
})
it('serializes quick successive drags without clearing the newer preview', async () => {
  const releases: Array<() => void> = []
  const save = vi.fn((_value: number) => new Promise<void>((resolve) => releases.push(resolve)))
  const { handle, pointer } = setup(save)
  pointer('pointerDown', 100)
  await act(async () => {
    pointer('pointerUp', 120)
  })
  pointer('pointerDown', 120)
  await act(async () => {
    pointer('pointerUp', 150)
  })
  expect(save).toHaveBeenCalledTimes(1)
  expect(handle.getAttribute('data-resizing')).toBe('false')
  expect(handle.getAttribute('aria-valuenow')).toBe('310')
  await act(async () => {
    releases[0]()
  })
  expect(save).toHaveBeenCalledTimes(2)
  expect(handle.getAttribute('aria-valuenow')).toBe('310')
  await act(async () => {
    releases[1]()
  })
  expect(handle.getAttribute('data-resizing')).toBe('false')
  expect(save.mock.calls.map((call) => call[0])).toEqual([280, 310])
})
