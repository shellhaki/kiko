import { expect, it } from 'vitest'
import { convertToExcalidrawElements, exportToSvg } from '@excalidraw/excalidraw'
import { sceneElements, staticSvg } from './diagram-scene'

class FakeFontFace {
  load() {
    return Promise.resolve(this)
  }
}
Object.assign(globalThis, { FontFace: FakeFontFace })
Object.defineProperty(document, 'fonts', {
  value: { add() {}, check: () => true, load: () => Promise.resolve([]), ready: Promise.resolve() },
  configurable: true,
})

it('turns a real Excalidraw export into static SVG without styles or metadata', async () => {
  const elements = convertToExcalidrawElements([
    { type: 'rectangle', x: 0, y: 0, width: 160, height: 60, label: { text: 'Client' }, id: 'a' },
    { type: 'rectangle', x: 300, y: 0, width: 160, height: 60, label: { text: 'Server' }, id: 'b' },
    { type: 'arrow', x: 160, y: 30, width: 140, height: 0, start: { id: 'a' }, end: { id: 'b' } },
  ])
  const drawing = await exportToSvg({
    elements,
    appState: { exportBackground: true, viewBackgroundColor: '#ffffff' },
    files: null,
    skipInliningFonts: true,
  })
  expect(drawing.querySelector('style, metadata, mask')).not.toBeNull()
  const svg = staticSvg(drawing)
  expect(svg).toContain('Client')
  expect(svg).toContain('Server')
  expect(svg.match(/xmlns/g)).toHaveLength(1)
  expect(svg).not.toMatch(/<style|<metadata|<mask|style="|direction=|version=/)
}, 30000)

it('converts an agent skeleton into plain, bound Excalidraw elements', () => {
  const scene = JSON.stringify({
    type: 'excalidraw',
    skeleton: true,
    elements: [
      { type: 'rectangle', id: 'a', x: 0, y: 0, width: 160, height: 60, label: { text: 'Client' } },
      {
        type: 'diamond',
        id: 'b',
        x: 300,
        y: 100,
        width: 160,
        height: 80,
        label: { text: 'Valid?' },
      },
      {
        type: 'arrow',
        x: 160,
        y: 30,
        width: 140,
        height: 110,
        start: { id: 'a' },
        end: { id: 'b' },
      },
      { type: 'line', x: 0, y: 200, width: 100, height: -50 },
      { type: 'text', x: 0, y: 250, text: 'Note' },
    ],
  })
  const elements = sceneElements(scene)
  const byType = (type: string) =>
    elements.filter((element: { type: string }) => element.type === type)
  expect(byType('rectangle')[0].roughness).toBe(0)
  expect(
    byType('text')
      .map((element: { text: string }) => element.text)
      .sort(),
  ).toEqual(['Client', 'Note', 'Valid?'])
  const [rectangle, diamond] = [byType('rectangle')[0], byType('diamond')[0]]
  const arrow = byType('arrow')[0]
  expect(arrow.startBinding.elementId).toBe(rectangle.id)
  expect(arrow.endBinding.elementId).toBe(diamond.id)
  const [from, to] = arrow.points
  expect(Math.abs(to[0] - from[0] - 140)).toBeLessThan(2)
  expect(Math.abs(to[1] - from[1] - 110)).toBeLessThan(2)
  expect(byType('line')[0].points[1]).toEqual([100, -50])
})

it('returns saved scenes untouched and empty scenes as no elements', () => {
  const saved = [{ id: 'kept', type: 'rectangle' }]
  expect(sceneElements(JSON.stringify({ type: 'excalidraw', elements: saved }))).toEqual(saved)
  expect(sceneElements('')).toEqual([])
})
