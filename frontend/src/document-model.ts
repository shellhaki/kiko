import type { JSONContent } from '@tiptap/react'
export type Inline = { text: string; marks?: { type: string; href?: string; title?: string }[] }
export type Block = {
  id: string
  h: string
  type: string
  attrs?: Record<string, unknown>
  content?: Inline[]
  children?: Block[]
}
export type SavedDocument = {
  id: string
  chatId: string
  title: string
  rev: number
  blocks: Block[]
}
export type DocumentView = {
  tab?: 'document' | 'markdown'
  width?: number
  focus?: boolean
  scroll?: number
}
export type Suggestion = {
  id: string
  documentId: string
  baseRev: number
  before: string
  after: string
  status: string
}
export type DocumentRead = {
  suggestions?: Suggestion[]
  document: SavedDocument
  markdown: string
  draft?: { baseRev: number; blocks: Block[] }
  view: DocumentView
}
const types: Record<string, string> = {
  embed: 'diagram',
  heading: 'heading',
  paragraph: 'paragraph',
  code: 'codeBlock',
  table: 'table',
  row: 'tableRow',
  cell: 'tableCell',
  pending: 'pending',
  listItem: 'listItem',
}
const blockTypes: Record<string, string> = {
  diagram: 'embed',
  heading: 'heading',
  paragraph: 'paragraph',
  codeBlock: 'code',
  bulletList: 'list',
  orderedList: 'list',
  taskList: 'list',
  listItem: 'listItem',
  taskItem: 'listItem',
  table: 'table',
  tableRow: 'row',
  tableCell: 'cell',
  tableHeader: 'cell',
  pending: 'pending',
}
export function toEditor(blocks: Block[]): JSONContent {
  const convert = (b: Block, header = false, task = false): JSONContent => {
    const attrs = { ...b.attrs, id: b.id }
    let type = types[b.type]
    if (b.type === 'list')
      type = b.children?.some((item) => typeof item.attrs?.checked === 'boolean')
        ? 'taskList'
        : b.attrs?.ordered
          ? 'orderedList'
          : 'bulletList'
    if (b.type === 'listItem' && task) type = 'taskItem'
    if (b.type === 'cell' && header) type = 'tableHeader'
    const content = b.children?.length
      ? b.children.map((child) => convert(child, !!b.attrs?.header, type === 'taskList'))
      : (b.content ?? []).flatMap((inline) =>
          inline.text.split('\n').flatMap((text, index) => [
            ...(index ? [{ type: 'hardBreak' }] : []),
            ...(text
              ? [
                  {
                    type: 'text',
                    text,
                    marks: inline.marks?.map((mark) => ({
                      type: mark.type,
                      ...(mark.type === 'link'
                        ? { attrs: { href: mark.href, title: mark.title } }
                        : {}),
                    })),
                  },
                ]
              : []),
          ]),
        )
    if (b.type === 'code')
      return {
        type,
        attrs,
        content: b.content?.length
          ? [{ type: 'text', text: b.content.map((i) => i.text).join('') }]
          : [],
      }
    if (b.type === 'cell') return { type, attrs, content: [{ type: 'paragraph', content }] }
    return { type, attrs, ...(content.length ? { content } : {}) }
  }
  return {
    type: 'doc',
    content: blocks.length ? blocks.map((b) => convert(b)) : [{ type: 'paragraph' }],
  }
}
export function fromEditor(doc: JSONContent): Block[] {
  const convert = (node: JSONContent): Block => {
    let type = blockTypes[node.type ?? '']
    if (type === 'pending' && node.content?.length) type = 'paragraph'
    if (!type) throw new Error('This document contains unsupported formatting.')
    const { id, ...attrs } = node.attrs ?? {}
    if ((attrs.colspan && attrs.colspan !== 1) || (attrs.rowspan && attrs.rowspan !== 1))
      throw new Error('Merged table cells are not supported. Undo this change to keep all content.')
    if (type === 'paragraph') delete attrs.intent
    delete attrs.colspan
    delete attrs.rowspan
    delete attrs.colwidth
    if (node.type === 'orderedList') {
      attrs.ordered = true
      attrs.start = attrs.start ?? 1
    }
    if (node.type === 'bulletList' || node.type === 'taskList') attrs.ordered = false
    if (node.type === 'tableRow')
      attrs.header = node.content?.every((cell) => cell.type === 'tableHeader') ?? false
    const inline = (children: JSONContent[]): Inline[] =>
      children.flatMap((child) =>
        child.type === 'hardBreak'
          ? [{ text: '\n' }]
          : child.type === 'text'
            ? [
                {
                  text: child.text ?? '',
                  marks: child.marks?.map((mark) => ({
                    type: mark.type,
                    ...(mark.type === 'link'
                      ? {
                          href: String(mark.attrs?.href ?? ''),
                          ...(mark.attrs?.title ? { title: String(mark.attrs.title) } : {}),
                        }
                      : {}),
                  })),
                },
              ]
            : inline(child.content ?? []),
      )
    const content = ['paragraph', 'heading', 'code', 'cell'].includes(type)
      ? inline(node.content ?? [])
      : undefined
    const children = ['list', 'listItem', 'table', 'row'].includes(type)
      ? node.content?.map(convert)
      : undefined
    return {
      id: String(id || crypto.randomUUID()),
      h: '',
      type,
      ...(Object.keys(attrs).length ? { attrs } : {}),
      ...(content ? { content } : {}),
      ...(children ? { children } : {}),
    }
  }
  return (doc.content ?? []).map(convert)
}
