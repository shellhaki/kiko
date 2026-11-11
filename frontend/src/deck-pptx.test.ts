import { expect, it } from 'vitest'
import JSZip from 'jszip'
import { buildDeckPptx } from './deck-pptx'
import type { Deck } from './artifact-model'

const pixel =
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=='
const deck: Deck = {
  slides: [
    { id: 'one', layout: 'title', title: 'Care Compass', body: ['Gentle care coordination'] },
    {
      id: 'two',
      layout: 'bullets',
      title: 'Problem',
      body: ['Fifty million caregivers', 'Invisible labor'],
    },
    {
      id: 'three',
      layout: 'columns',
      title: 'How it works',
      left: ['Pillar one'],
      right: ['Pillar two'],
    },
    { id: 'four', layout: 'image', title: 'Design', imageId: 'img', body: ['Calm by default'] },
    {
      id: 'five',
      layout: 'chart',
      title: 'Reach',
      chart: { labels: ['2024', '2025'], values: [40, 90] },
    },
  ],
}

async function parts() {
  const bytes = await buildDeckPptx('Care Compass', deck, {
    img: { mime: 'image/png', base64: pixel },
  })
  expect(bytes[0]).toBe(0x50)
  expect(bytes[1]).toBe(0x4b)
  return JSZip.loadAsync(bytes)
}

it('builds one editable slide per deck slide', async () => {
  const zip = await parts()
  const slides = Object.keys(zip.files).filter((name) => /^ppt\/slides\/slide\d+\.xml$/.test(name))
  expect(slides).toHaveLength(5)
  const first = await zip.file('ppt/slides/slide1.xml')?.async('string')
  expect(first).toContain('Care Compass')
  expect(first).toContain('Gentle care coordination')
})

it('keeps bullets, charts and images native and editable', async () => {
  const zip = await parts()
  const bullets = await zip.file('ppt/slides/slide2.xml')?.async('string')
  expect(bullets).toContain('buChar')
  expect(bullets).not.toContain('•')
  const charts = Object.keys(zip.files).filter((name) => name.startsWith('ppt/charts/chart'))
  expect(charts.length).toBeGreaterThan(0)
  const chart = await zip.file('ppt/slides/slide5.xml')?.async('string')
  expect(chart).toContain('c:chart')
  const chartData = await zip.file(charts[0])?.async('string')
  expect(chartData).toContain('2025')
  expect(chartData).toContain('90')
  const media = Object.keys(zip.files).filter((name) => name.startsWith('ppt/media/'))
  expect(media.length).toBeGreaterThan(0)
  const image = await zip.file('ppt/slides/slide4.xml')?.async('string')
  expect(image).toContain('Calm by default')
})

it('applies the shared theme and keeps every shape on the canvas', async () => {
  const zip = await parts()
  const theme = await zip.file('ppt/theme/theme1.xml')?.async('string')
  expect(theme).toContain('Calibri')
  const wide = 12192000 + 1000
  const tall = 6858000 + 1000
  const frame = /<a:xfrm><a:off x="(\d+)" y="(\d+)"\/><a:ext cx="(\d+)" cy="(\d+)"\/>/g
  let frames = 0
  for (const name of Object.keys(zip.files)) {
    if (!/^ppt\/slides\/slide\d+\.xml$/.test(name)) continue
    const xml = (await zip.file(name)?.async('string')) ?? ''
    for (const match of xml.matchAll(frame)) {
      frames += 1
      const [x, y, w, h] = match.slice(1, 5).map(Number)
      expect(x + w).toBeLessThanOrEqual(wide)
      expect(y + h).toBeLessThanOrEqual(tall)
    }
  }
  expect(frames).toBeGreaterThan(0)
})

it('refuses decks without slides and missing images', async () => {
  await expect(buildDeckPptx('Empty', { slides: [] }, {})).rejects.toThrow('no slides')
  await expect(buildDeckPptx('Missing', { slides: [deck.slides[3]] }, {})).rejects.toThrow(
    'Image is unavailable',
  )
})
