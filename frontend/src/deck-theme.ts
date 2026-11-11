export const slideSize = { width: 13.333, height: 7.5 }
export const deckFont = 'Calibri'
export const deckPalette = {
  background: 'FFFFFF',
  ink: '232323',
  body: '333333',
  muted: '595959',
  faint: '8A8A8A',
  accent: '7950CC',
  titleAccent: '6D3FC0',
  chart: ['7950CC', '2F7D62', '3E6FB0', 'C77E1F', 'B42332'],
}
export type SlideRect = { x: number; y: number; w: number; h: number }
export const deckLayout: Record<string, SlideRect & { size?: number }> = {
  strip: { x: 0, y: 0, w: 13.333, h: 0.1 },
  title: { x: 0.66, y: 0.55, w: 11.9, h: 1.2, size: 32 },
  titleSlide: { x: 0.87, y: 1.97, w: 11.5, h: 1.86, size: 44 },
  subtitle: { x: 0.87, y: 4.27, w: 11.5, h: 1.7, size: 24 },
  body: { x: 0.82, y: 2.19, w: 11.6, h: 4.48, size: 22 },
  left: { x: 0.71, y: 2.19, w: 5.58, h: 4.48, size: 22 },
  right: { x: 7.0, y: 2.19, w: 5.58, h: 4.48, size: 22 },
  image: { x: 2.0, y: 1.97, w: 9.3, h: 4.15 },
  caption: { x: 0.71, y: 6.45, w: 11.9, h: 0.7, size: 16 },
  chart: { x: 0.93, y: 2.08, w: 11.5, h: 3.7 },
  number: { x: 12.38, y: 7.0, w: 0.6, h: 0.33, size: 10 },
}
