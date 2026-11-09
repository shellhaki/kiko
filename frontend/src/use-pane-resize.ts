import { useEffect, useRef, useState } from 'react'
import type { PointerEvent as ReactPointerEvent } from 'react'

type Options = {
  value: number
  min: number
  max: number
  measure(handle: HTMLElement, startX: number): ((x: number) => number) | null
  commit(value: number): Promise<unknown>
}
type Drag = {
  handle: HTMLElement
  pointerId: number
  measure(x: number): number
  value: number
}

export function usePaneResize({ value, min, max, measure, commit }: Options) {
  const [preview, setPreview] = useState<number>()
  const [active, setActive] = useState(false)
  const drag = useRef<Drag | null>(null)
  const frame = useRef<number | null>(null)
  const generation = useRef(0)
  const pending = useRef<Promise<unknown>>(Promise.resolve())
  const clamp = (next: number) => Math.max(min, Math.min(max, next))
  const stopFrame = () => {
    if (frame.current !== null) cancelAnimationFrame(frame.current)
    frame.current = null
  }
  const release = () => {
    const active = drag.current
    drag.current = null
    setActive(false)
    if (active?.handle.hasPointerCapture(active.pointerId))
      active.handle.releasePointerCapture(active.pointerId)
  }
  const cancel = () => {
    if (!drag.current) return
    generation.current++
    stopFrame()
    release()
    setPreview(undefined)
  }
  useEffect(
    () => () => {
      generation.current++
      if (frame.current !== null) cancelAnimationFrame(frame.current)
      const active = drag.current
      drag.current = null
      if (active?.handle.hasPointerCapture(active.pointerId))
        active.handle.releasePointerCapture(active.pointerId)
    },
    [],
  )

  return {
    value: preview ?? value,
    active,
    cancel,
    handlers: {
      onPointerDown(event: ReactPointerEvent<HTMLElement>) {
        if (event.button !== 0 || drag.current) return
        const position = measure(event.currentTarget, event.clientX)
        if (!position || !Number.isFinite(position(event.clientX))) return
        event.preventDefault()
        event.currentTarget.focus({ preventScroll: true })
        event.currentTarget.setPointerCapture(event.pointerId)
        generation.current++
        const initial = clamp(position(event.clientX))
        drag.current = {
          handle: event.currentTarget,
          pointerId: event.pointerId,
          measure: position,
          value: initial,
        }
        setPreview(initial)
        setActive(true)
      },
      onPointerMove(event: ReactPointerEvent<HTMLElement>) {
        const active = drag.current
        if (!active || active.pointerId !== event.pointerId) return
        active.value = clamp(active.measure(event.clientX))
        if (frame.current === null)
          frame.current = requestAnimationFrame(() => {
            frame.current = null
            if (drag.current === active) setPreview(active.value)
          })
      },
      onPointerUp(event: ReactPointerEvent<HTMLElement>) {
        const active = drag.current
        if (!active || active.pointerId !== event.pointerId) return
        const finalValue = clamp(active.measure(event.clientX))
        const version = generation.current
        stopFrame()
        release()
        setPreview(finalValue)
        pending.current = pending.current.catch(() => {}).then(() => commit(finalValue))
        void pending.current
          .catch(() => {})
          .finally(() => {
            if (generation.current === version) setPreview(undefined)
          })
      },
      onPointerCancel(event: ReactPointerEvent<HTMLElement>) {
        if (drag.current?.pointerId === event.pointerId) cancel()
      },
      onLostPointerCapture(event: ReactPointerEvent<HTMLElement>) {
        if (drag.current?.pointerId === event.pointerId) cancel()
      },
    },
  }
}
