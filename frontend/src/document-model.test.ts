import { expect, it } from 'vitest'
import { fromEditor, toEditor } from './document-model'
import type { Block } from './document-model'
it('keeps stable identifiers, formatting, nesting, tables and task state through editor conversion', () => {
  const blocks: Block[] = [
    { id: 'h', h: '', type: 'heading', attrs: { level: 2 }, content: [{ text: 'Plan' }] },
    {
      id: 'l',
      h: '',
      type: 'list',
      attrs: { ordered: false },
      children: [
        {
          id: 'i',
          h: '',
          type: 'listItem',
          attrs: { checked: true },
          children: [
            {
              id: 'p',
              h: '',
              type: 'paragraph',
              content: [
                {
                  text: 'Done',
                  marks: [{ type: 'bold' }, { type: 'link', href: 'https://example.com' }],
                },
              ],
            },
          ],
        },
      ],
    },
    {
      id: 't',
      h: '',
      type: 'table',
      children: [
        {
          id: 'r',
          h: '',
          type: 'row',
          attrs: { header: true },
          children: [{ id: 'c', h: '', type: 'cell', content: [{ text: 'Value' }] }],
        },
      ],
    },
  ]
  const restored = fromEditor(toEditor(blocks))
  expect(restored).toEqual(blocks)
})
it('turns an edited pending block into user text without changing its ID', () => {
  const result = fromEditor({
    type: 'doc',
    content: [
      {
        type: 'pending',
        attrs: { id: 'pending', intent: 'Risks' },
        content: [{ type: 'text', text: 'My own risks' }],
      },
    ],
  })
  expect(result[0]).toEqual({
    id: 'pending',
    h: '',
    type: 'paragraph',
    content: [{ text: 'My own risks', marks: undefined }],
  })
})
it('keeps code newlines and assigns new blocks unique IDs', () => {
  const source: Block[] = [{ id: 'code', h: '', type: 'code', content: [{ text: 'one\ntwo' }] }]
  expect(fromEditor(toEditor(source))[0].content).toEqual([{ text: 'one\ntwo', marks: undefined }])
  const doc = fromEditor({ type: 'doc', content: [{ type: 'paragraph' }, { type: 'paragraph' }] })
  expect(doc[0].id).not.toBe(doc[1].id)
})
