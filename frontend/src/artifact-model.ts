export type Cell = {
  value?: string | number | boolean
  formula?: string
  bold?: boolean
  fill?: string
  format?: string
}
export type SheetTab = { name: string; cells: Record<string, Cell> }
export type Workbook = { tabs: SheetTab[] }
export type Slide = {
  id: string
  layout: string
  title: string
  body?: string[]
  left?: string[]
  right?: string[]
  notes?: string
  imageId?: string
  chart?: { labels: string[]; values: number[] }
}
export type Deck = { slides: Slide[] }
export type ArtifactRecord = {
  id: string
  chatId: string
  kind: 'sheet' | 'deck'
  title: string
  rev: number
  payload: Workbook | Deck
  draft?: { baseRev: number; payload: Workbook | Deck }
  values?: Record<string, Record<string, { value: string; error?: string }>>
}
export function cellInput(value: string): Cell {
  if (value.startsWith('=')) return { formula: value }
  if (value.trim() !== '' && Number.isFinite(Number(value))) return { value: Number(value) }
  return { value }
}
export function columnName(index: number) {
  let name = ''
  for (let value = index + 1; value > 0; value = Math.floor((value - 1) / 26))
    name = String.fromCharCode(65 + ((value - 1) % 26)) + name
  return name
}
