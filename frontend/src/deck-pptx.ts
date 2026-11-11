import PptxGenJS from 'pptxgenjs'
import type { Deck } from './artifact-model'
import { deckFont, deckLayout, deckPalette } from './deck-theme'

export type DeckImage = { mime: string; base64: string }

type TextRun = { text: string; options: Record<string, unknown> }

function runs(
  lines: string[] | undefined,
  size: number,
  color: string,
  bulleted: boolean,
): TextRun[] {
  return (lines ?? []).map((line) => ({
    text: line,
    options: {
      fontSize: size,
      color,
      fontFace: deckFont,
      bullet: bulleted ? { indent: 12 } : false,
      breakLine: true,
      paraSpaceAfter: bulleted ? 6 : 2,
      margin: 0,
    },
  }))
}

function box(rect: { x: number; y: number; w: number; h: number }) {
  return { x: rect.x, y: rect.y, w: rect.w, h: rect.h, margin: 0 }
}

export function toBase64(bytes: Uint8Array): string {
  let text = ''
  for (let i = 0; i < bytes.length; i += 0x8000)
    text += String.fromCharCode(...bytes.subarray(i, i + 0x8000))
  return btoa(text)
}

export async function buildDeckPptx(
  title: string,
  deck: Deck,
  images: Record<string, DeckImage>,
): Promise<Uint8Array> {
  if (!deck?.slides?.length) throw new Error('The deck has no slides.')
  const pptx = new PptxGenJS()
  pptx.layout = 'LAYOUT_WIDE'
  pptx.theme = { headFontFace: deckFont, bodyFontFace: deckFont }
  pptx.title = title || 'Slides'
  deck.slides.forEach((slide, index) => {
    const page = pptx.addSlide()
    page.background = { color: deckPalette.background }
    const strip = deckLayout.strip
    page.addShape('rect', {
      ...box(strip),
      fill: { color: deckPalette.accent },
      line: { color: deckPalette.background },
    })
    if (slide.layout === 'title') {
      const head = deckLayout.titleSlide
      page.addText(slide.title, {
        ...box(head),
        fontSize: head.size,
        bold: true,
        color: deckPalette.titleAccent,
        fontFace: deckFont,
        align: 'left',
        valign: 'top',
      })
      const sub = deckLayout.subtitle
      page.addText(runs(slide.body, sub.size ?? 24, deckPalette.muted, false), {
        ...box(sub),
        align: 'left',
        valign: 'top',
      })
    } else {
      const head = deckLayout.title
      page.addText(slide.title, {
        ...box(head),
        fontSize: head.size,
        bold: true,
        color: deckPalette.ink,
        fontFace: deckFont,
        align: 'left',
        valign: 'top',
      })
      const dense = (slide.body ?? []).length > 6 || (slide.left ?? []).length > 6
      const size = dense ? 16 : (deckLayout.body.size ?? 22)
      if (slide.layout === 'bullets') {
        page.addText(runs(slide.body, size, deckPalette.body, true), {
          ...box(deckLayout.body),
          align: 'left',
          valign: 'top',
        })
      } else if (slide.layout === 'columns') {
        page.addText(runs(slide.left, size, deckPalette.body, true), {
          ...box(deckLayout.left),
          align: 'left',
          valign: 'top',
        })
        page.addText(runs(slide.right, size, deckPalette.body, true), {
          ...box(deckLayout.right),
          align: 'left',
          valign: 'top',
        })
      } else if (slide.layout === 'image') {
        const image = slide.imageId ? images[slide.imageId] : undefined
        if (!image || !image.mime.startsWith('image/')) throw new Error('Image is unavailable.')
        page.addImage({
          data: `data:${image.mime};base64,${image.base64}`,
          ...box(deckLayout.image),
          sizing: { type: 'contain', w: deckLayout.image.w, h: deckLayout.image.h },
        })
        const caption = deckLayout.caption
        page.addText(runs(slide.body, caption.size ?? 16, deckPalette.muted, false), {
          ...box(caption),
          align: 'left',
          valign: 'top',
        })
      } else if (slide.layout === 'chart' && slide.chart) {
        page.addChart(
          'bar',
          [{ name: 'Values', labels: slide.chart.labels, values: slide.chart.values }],
          {
            ...box(deckLayout.chart),
            chartColors: [deckPalette.accent as string],
            showValue: true,
            showLegend: false,
            dataLabelColor: deckPalette.body,
            dataLabelFontSize: 11,
          },
        )
      }
    }
    const number = deckLayout.number
    page.addText(String(index + 1), {
      ...box(number),
      fontSize: number.size,
      color: deckPalette.faint,
      fontFace: deckFont,
      align: 'right',
      valign: 'top',
    })
    if (slide.notes) page.addNotes(slide.notes)
  })
  const output = await pptx.write({ outputType: 'uint8array' })
  if (!(output instanceof Uint8Array)) throw new Error('Slide export failed.')
  return output
}
