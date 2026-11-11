import { StrictMode } from 'react'
import type { Editor } from '@tiptap/core'
import { afterEach, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { DocumentPane } from './DocumentPane'
import { UIProvider } from './components/ui'
import { flushDocument } from './document-session'
import type { Backend } from './api'
import type { DocumentRead } from './document-model'
const emptyRect = { top: 0, left: 0, bottom: 0, right: 0, width: 0, height: 0, x: 0, y: 0 }
for (const prototype of [Element.prototype, Range.prototype]) {
  prototype.getClientRects ??= () => [] as unknown as DOMRectList
}
Range.prototype.getBoundingClientRect ??= () => ({ ...emptyRect }) as DOMRect
vi.mock('./DiagramCanvas', () => ({
  default: ({
    scene,
    caption,
    onApply,
  }: {
    scene: string
    caption: string
    onApply(result: { scene: string; svg: string; caption: string }): void
  }) => (
    <button
      onClick={() =>
        onApply({
          scene: scene || '{"type":"excalidraw","elements":[{}]}',
          svg: '<svg height="9"/>',
          caption: caption || 'Flow',
        })
      }
    >
      Apply mock drawing
    </button>
  ),
}))
const data: DocumentRead = {
  document: {
    id: 'doc',
    chatId: 'chat',
    title: 'Plan',
    rev: 1,
    blocks: [
      { id: 'paragraph', h: '12345678', type: 'paragraph', content: [{ text: 'Saved text' }] },
    ],
  },
  markdown: 'Saved text',
  view: {},
}
function setup(value = data) {
  const api = {
    Questions: vi.fn().mockResolvedValue([]),
    AnswerQuestion: vi.fn().mockResolvedValue({}),
    ResolveQuestion: vi.fn().mockResolvedValue(undefined),
    ReadDocument: vi.fn().mockResolvedValue(structuredClone(value)),
    SaveDocumentDraft: vi.fn().mockResolvedValue(undefined),
    SaveDocument: vi
      .fn()
      .mockImplementation(async (_id, rev, blocks) => ({ ...data.document, rev: rev + 1, blocks })),
    SaveDocumentView: vi.fn().mockResolvedValue(undefined),
    DiscardDocumentDraft: vi.fn().mockResolvedValue(undefined),
    CopyDocument: vi.fn().mockResolvedValue(undefined),
    ExportFormats: vi.fn().mockResolvedValue(['md', 'docx', 'pdf']),
    ExportDocumentFormat: vi.fn().mockResolvedValue(true),
  }
  window.go = { main: { App: api as unknown as Backend } }
  const close = vi.fn().mockResolvedValue(undefined)
  render(
    <StrictMode>
      <UIProvider>
        <DocumentPane id="doc" close={close} onState={vi.fn()} />
      </UIProvider>
    </StrictMode>,
  )
  return { api, close }
}
afterEach(() => {
  cleanup()
  delete window.go
  vi.restoreAllMocks()
})
it('opens a real Tiptap document and switches to saved Markdown', async () => {
  setup()
  expect((await screen.findByRole('textbox', { name: 'Document editor' })).textContent).toBe(
    'Saved text',
  )
  fireEvent.click(screen.getByRole('tab', { name: 'Markdown' }))
  await waitFor(() =>
    expect(screen.getByRole('tab', { name: 'Markdown' }).getAttribute('aria-selected')).toBe(
      'true',
    ),
  )
  expect(document.querySelector('.markdown-source')?.textContent).toBe('Saved text')
})
it('recovers a draft and flushes it before navigation', async () => {
  const { api } = setup({
    ...data,
    draft: {
      baseRev: 1,
      blocks: [
        { id: 'paragraph', h: '', type: 'paragraph', content: [{ text: 'Recovered draft' }] },
      ],
    },
  })
  expect((await screen.findByRole('textbox', { name: 'Document editor' })).textContent).toBe(
    'Recovered draft',
  )
  await flushDocument()
  expect(api.SaveDocument).toHaveBeenCalledWith(
    'doc',
    1,
    expect.arrayContaining([expect.objectContaining({ id: 'paragraph' })]),
  )
})
it('keeps the failed draft recoverable after closing a conflicted document', async () => {
  const { api, close } = setup({ ...data, draft: { baseRev: 1, blocks: data.document.blocks } })
  api.SaveDocument.mockRejectedValue(new Error('Revision changed'))
  await screen.findByRole('textbox', { name: 'Document editor' })
  fireEvent.click(screen.getByRole('button', { name: 'Close document' }))
  await screen.findByRole('alert')
  await waitFor(() => expect(close).toHaveBeenCalledTimes(1))
  expect(api.SaveDocument).toHaveBeenCalled()
})

it('hides retry when another writer moved the saved revision', async () => {
  const { api } = setup({ ...data, draft: { baseRev: 1, blocks: data.document.blocks } })
  let advanced = false
  api.ReadDocument.mockImplementation(async () => ({
    ...structuredClone(data),
    document: { ...data.document, rev: advanced ? 2 : 1 },
  }))
  api.SaveDocument.mockRejectedValue(new Error('Revision changed'))
  await screen.findByRole('textbox', { name: 'Document editor' })
  advanced = true
  fireEvent.click(screen.getByRole('button', { name: 'Close document' }))
  await screen.findByRole('alert')
  expect(screen.queryByRole('button', { name: 'Retry save' })).toBeNull()
  expect(screen.getByRole('button', { name: 'Reload saved version' })).toBeTruthy()
})
it('confirms copying with status and keeps the button usable', async () => {
  const { api } = setup()
  await screen.findByRole('textbox', { name: 'Document editor' })
  fireEvent.click(screen.getByRole('button', { name: 'Copy Markdown' }))
  await waitFor(() => expect(api.CopyDocument).toHaveBeenCalledWith('doc'))
  expect(await screen.findByText('Markdown copied')).toBeTruthy()
  expect(screen.queryByRole('alert')).toBeNull()
  expect(screen.getByRole('button', { name: 'Copy Markdown' })).toBeTruthy()
})
it('shows preparing state while an export runs', async () => {
  const { api } = setup()
  let resolve!: (done: boolean) => void
  api.ExportDocumentFormat.mockImplementation(
    () => new Promise<boolean>((done) => void (resolve = done)),
  )
  await screen.findByRole('textbox', { name: 'Document editor' })
  fireEvent.click(screen.getByRole('button', { name: 'Export document' }))
  fireEvent.click(await screen.findByRole('button', { name: 'PDF document' }))
  expect(await screen.findByRole('button', { name: 'Preparing…' })).toBeTruthy()
  resolve(true)
  await waitFor(() => expect(screen.queryByRole('button', { name: 'Preparing…' })).toBeNull())
  expect(await screen.findByText('Exported')).toBeTruthy()
})
it('highlights anchored questions and sends a suggested answer to the original thread', async () => {
  const { api } = setup()
  api.Questions.mockResolvedValue([
    {
      id: 'question',
      documentId: 'doc',
      blockId: 'paragraph',
      start: 0,
      end: 5,
      quote: 'Saved',
      body: 'Keep this wording?',
      suggested: ['Keep it'],
      resolved: false,
    },
  ] as never[])
  await screen.findByText('Keep this wording?')
  expect(document.querySelector('.question-anchor')?.textContent).toBe('Saved')
  fireEvent.click(screen.getByRole('button', { name: 'Keep it' }))
  await waitFor(() =>
    expect(api.AnswerQuestion).toHaveBeenCalledWith('question', 'Keep it', expect.any(String)),
  )
})

it('saves actual editor changes after the typing debounce and keeps the block ID', async () => {
  const { api } = setup()
  const element = await screen.findByRole('textbox', { name: 'Document editor' })
  const editor = (element as HTMLElement & { editor: Editor }).editor
  act(() => {
    editor.commands.insertContentAt(11, ' more')
  })
  await waitFor(() => expect(api.SaveDocumentDraft).toHaveBeenCalled())
  await waitFor(() => expect(api.SaveDocument).toHaveBeenCalled(), { timeout: 2000 })
  expect(api.SaveDocument.mock.calls[0][2][0]).toMatchObject({
    id: 'paragraph',
    content: [{ text: 'Saved text more' }],
  })
  act(() => {
    editor.commands.undo()
  })
  await waitFor(() => expect(api.SaveDocument).toHaveBeenCalledTimes(2), { timeout: 2000 })
  expect(api.SaveDocument.mock.calls[1][2][0].content).toEqual([
    { text: 'Saved text', marks: undefined },
  ])
})

it('resizes safely when React replays updates after the pointer event ends', async () => {
  setup()
  await screen.findByRole('textbox', { name: 'Document editor' })
  class TestPointerEvent extends MouseEvent {
    pointerId: number
    constructor(type: string, init: PointerEventInit = {}) {
      super(type, init)
      this.pointerId = init.pointerId ?? 1
    }
  }
  vi.stubGlobal('PointerEvent', TestPointerEvent)
  const divider = screen.getByRole('separator', { name: 'Document width' })
  const workspace = divider.parentElement!.parentElement!
  const navigation = document.createElement('nav')
  navigation.className = 'rail'
  workspace.prepend(navigation)
  vi.spyOn(navigation, 'getBoundingClientRect').mockReturnValue({ width: 200 } as DOMRect)
  vi.spyOn(divider.parentElement!, 'getBoundingClientRect').mockReturnValue({
    width: 416,
  } as DOMRect)
  vi.spyOn(workspace, 'getBoundingClientRect').mockReturnValue({
    right: 1000,
    width: 1000,
  } as DOMRect)
  Object.defineProperty(divider, 'setPointerCapture', { value: vi.fn() })
  Object.defineProperty(divider, 'hasPointerCapture', { value: () => true })
  Object.defineProperty(divider, 'releasePointerCapture', { value: vi.fn() })
  try {
    act(() => {
      fireEvent.pointerDown(divider, { pointerId: 1, button: 0, buttons: 1, clientX: 584 })
      fireEvent.pointerMove(divider, { pointerId: 1, buttons: 1, clientX: 650 })
      fireEvent.pointerMove(divider, { pointerId: 1, buttons: 1, clientX: 600 })
    })
    await waitFor(() => expect(divider.getAttribute('aria-valuenow')).toBe('50'))
    expect(screen.getByRole('textbox', { name: 'Document editor' })).toBeTruthy()
  } finally {
    vi.unstubAllGlobals()
  }
})
it('does not create a document revision just to initialize table paragraph IDs', async () => {
  const table: DocumentRead = {
    ...data,
    document: {
      ...data.document,
      blocks: [
        {
          id: 'table',
          h: '12345678',
          type: 'table',
          children: [
            {
              id: 'row',
              h: '12345678',
              type: 'row',
              attrs: { header: true },
              children: [
                { id: 'cell', h: '12345678', type: 'cell', content: [{ text: 'A cell' }] },
              ],
            },
          ],
        },
      ],
    },
  }
  const { api } = setup(table)
  await screen.findByRole('textbox', { name: 'Document editor' })
  await act(() => new Promise((resolve) => setTimeout(resolve, 30)))
  await flushDocument()
  expect(api.SaveDocumentDraft).not.toHaveBeenCalled()
  expect(api.SaveDocument).not.toHaveBeenCalled()
})

it('does not let a pending scroll save restore the width before a keyboard resize', async () => {
  const { api } = setup()
  await screen.findByRole('textbox', { name: 'Document editor' })
  fireEvent.scroll(document.querySelector('.document-scroll')!)
  fireEvent.keyDown(screen.getByRole('separator', { name: 'Document width' }), { key: 'ArrowLeft' })
  await waitFor(() =>
    expect(api.SaveDocumentView).toHaveBeenCalledWith(
      'doc',
      expect.objectContaining({ width: 54 }),
    ),
  )
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 350))
  })
  expect(api.SaveDocumentView).toHaveBeenCalledTimes(1)
})

it('draws a new Excalidraw diagram and saves its scene with the document', async () => {
  const { api } = setup()
  const element = await screen.findByRole('textbox', { name: 'Document editor' })
  const editor = (element as HTMLElement & { editor: Editor }).editor
  fireEvent.click(screen.getByRole('button', { name: 'Draw diagram' }))
  fireEvent.click(await screen.findByRole('button', { name: 'Apply mock drawing' }))
  await waitFor(() => expect(api.SaveDocument).toHaveBeenCalled(), { timeout: 2000 })
  const saved = api.SaveDocument.mock.calls[0][2].find(
    (block: { type: string }) => block.type === 'embed',
  )
  expect(saved.attrs).toMatchObject({
    svg: '<svg height="9"/>',
    caption: 'Flow',
    scene: '{"type":"excalidraw","elements":[{}]}',
  })
  expect(saved.attrs.ref).toBeTruthy()
  expect(editor.getJSON().content?.some((node) => node.type === 'diagram')).toBe(true)
})

it('reopens a saved diagram scene and replaces only its drawing', async () => {
  const scene = '{"type":"excalidraw","elements":[{"id":"saved"}]}'
  const { api } = setup({
    ...data,
    document: {
      ...data.document,
      blocks: [
        {
          id: 'figure',
          h: '11111111',
          type: 'embed',
          attrs: { ref: 'r1', svg: '<svg/>', scene, caption: 'Old' },
        },
      ],
    },
  })
  const element = await screen.findByRole('textbox', { name: 'Document editor' })
  const editor = (element as HTMLElement & { editor: Editor }).editor
  act(() => {
    editor.commands.setNodeSelection(0)
  })
  fireEvent.click(await screen.findByRole('button', { name: 'Edit diagram' }))
  expect(screen.queryByRole('button', { name: 'Edit SVG source' })).toBeNull()
  fireEvent.click(await screen.findByRole('button', { name: 'Apply mock drawing' }))
  await waitFor(() => expect(api.SaveDocument).toHaveBeenCalled(), { timeout: 2000 })
  expect(api.SaveDocument.mock.calls[0][2][0]).toMatchObject({
    id: 'figure',
    attrs: { ref: 'r1', scene, caption: 'Old' },
  })
})

it('offers SVG source and Excalidraw redraw for an agent-made diagram without a scene', async () => {
  setup({
    ...data,
    document: {
      ...data.document,
      blocks: [
        {
          id: 'figure',
          h: '11111111',
          type: 'embed',
          attrs: { ref: 'r1', svg: '<svg/>', caption: 'Old' },
        },
      ],
    },
  })
  const element = await screen.findByRole('textbox', { name: 'Document editor' })
  const editor = (element as HTMLElement & { editor: Editor }).editor
  act(() => {
    editor.commands.setNodeSelection(0)
  })
  expect(await screen.findByRole('button', { name: 'Redraw in Excalidraw' })).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Edit SVG source' })).toBeTruthy()
})
