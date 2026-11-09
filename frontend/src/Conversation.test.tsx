import { afterEach, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { Conversation } from './Conversation'
import { UIProvider } from './components/ui'
import type { Backend, ChatMessage, State } from './api'
import { mergeMessages } from './conversation-state'

const message = (patch: Partial<ChatMessage> = {}): ChatMessage => ({
  id: 'reply',
  chatId: 'chat',
  role: 'assistant',
  content: '',
  ordinal: 2,
  runId: 'run',
  status: 'running',
  error: '',
  errorKind: '',
  sequence: 1,
  attempt: 1,
  previous: [],
  ...patch,
})
function setup(patch: Partial<State> = {}) {
  const state: State = {
    messages: [],
    draft: '',
    preferences: {
      nickname: 'Haki',
      stage: 'done',
      theme: 'dark',
      sidebarOpen: true,
      sidebarWidth: 260,
      resetInProgress: false,
    },
    projects: [],
    chats: [],
    documents: [],
    selection: { projectId: '', chatId: 'chat' },
    documentId: '',
    connected: true,
    ...patch,
  }
  const api = {
    AttachFile: vi.fn(),
    Attachments: vi.fn().mockResolvedValue([]),
    ReadAttachment: vi.fn(),
    RemoveAttachment: vi.fn(),
    AttachmentPreview: vi.fn(),
    Conversation: vi.fn().mockResolvedValue({ messages: state.messages, draft: state.draft }),
    SaveDraft: vi.fn().mockResolvedValue(undefined),
    SendMessage: vi.fn().mockResolvedValue(state),
    RetryMessage: vi.fn().mockResolvedValue(state),
    StopRun: vi.fn().mockResolvedValue(undefined),
    OpenLink: vi.fn().mockResolvedValue(undefined),
  }
  window.go = { main: { App: api as unknown as Backend } }
  let callback!: (m: ChatMessage) => void
  const off = vi.fn()
  window.runtime = {
    EventsOnMultiple: vi.fn((_, cb) => {
      callback = cb
      return off
    }),
  }
  const view = render(
    <UIProvider>
      <Conversation
        state={state}
        heading="Hello Haki"
        subtitle="How can I help?"
        onState={vi.fn()}
        run={vi.fn()}
      />
    </UIProvider>,
  )
  return { api, view, off, emit: (m: ChatMessage) => act(() => callback(m)) }
}
afterEach(() => {
  cleanup()
  delete window.go
  delete window.runtime
  vi.restoreAllMocks()
})
it('saves drafts and keeps failed input with the same request identifier', async () => {
  const { api } = setup()
  api.SendMessage.mockRejectedValue(new Error('Offline'))
  const input = screen.getByRole('textbox', { name: 'Message' })
  fireEvent.change(input, { target: { value: 'My question' } })
  fireEvent.keyDown(input, { key: 'Enter' })
  await screen.findByRole('alert')
  expect((input as HTMLTextAreaElement).value).toBe('My question')
  expect(api.SaveDraft).toHaveBeenLastCalledWith('', 'chat', 'My question')
  const first = api.SendMessage.mock.calls[0]
  fireEvent.click(screen.getByRole('button', { name: 'Send message' }))
  await waitFor(() => expect(api.SendMessage).toHaveBeenCalledTimes(2))
  expect(api.SendMessage.mock.calls[1]).toEqual(first)
})
it('handles Shift+Enter and IME and prevents double submit', async () => {
  const { api } = setup()
  api.SendMessage.mockReturnValue(new Promise(() => {}))
  const input = screen.getByRole('textbox', { name: 'Message' })
  fireEvent.change(input, { target: { value: 'こんにちは' } })
  fireEvent.keyDown(input, { key: 'Enter', shiftKey: true })
  fireEvent.compositionStart(input)
  fireEvent.keyDown(input, { key: 'Enter' })
  expect(api.SendMessage).not.toHaveBeenCalled()
  fireEvent.compositionEnd(input)
  fireEvent.keyDown(input, { key: 'Enter' })
  fireEvent.keyDown(input, { key: 'Enter' })
  await waitFor(() => expect(api.SendMessage).toHaveBeenCalledTimes(1))
})
it('filters stale events and other chats and stops the current attempt', async () => {
  const { api, emit, view, off } = setup({ messages: [message()] })
  emit(message({ chatId: 'other', content: 'Wrong chat', sequence: 10 }))
  emit(message({ content: 'Latest text', sequence: 3 }))
  emit(message({ content: 'Outdated text', sequence: 2 }))
  expect(screen.getByText('Latest text')).toBeTruthy()
  expect(screen.queryByText('Wrong chat')).toBeNull()
  expect(screen.queryByText('Outdated text')).toBeNull()
  fireEvent.click(screen.getByRole('button', { name: 'Stop reply' }))
  expect(api.StopRun).toHaveBeenCalledWith('run', 1)
  view.unmount()
  expect(off).toHaveBeenCalled()
})
it('keeps offline history readable and saves drafts without sending', async () => {
  const { api } = setup({
    connected: false,
    messages: [message({ content: 'Saved history', status: 'complete' })],
  })
  expect(screen.getByText('Saved history')).toBeTruthy()
  const input = screen.getByRole('textbox', { name: 'Message' })
  fireEvent.change(input, { target: { value: 'Offline draft' } })
  fireEvent.keyDown(input, { key: 'Enter' })
  await waitFor(() => expect(api.SaveDraft).toHaveBeenCalledWith('', 'chat', 'Offline draft'))
  expect(api.SendMessage).not.toHaveBeenCalled()
})
it('renders Markdown without HTML or remote images and opens links through Go', () => {
  const { api } = setup({
    messages: [
      message({
        status: 'complete',
        content:
          '**Bold** [Meta](https://meta.ai) ![Tracking image](https://example.com/pixel) <script>alert(1)</script>',
      }),
    ],
  })
  expect(screen.getByText('Bold').tagName).toBe('STRONG')
  expect(document.querySelector('img')).toBeNull()
  expect(document.querySelector('script')).toBeNull()
  fireEvent.click(screen.getByRole('link', { name: 'Meta' }))
  expect(api.OpenLink).toHaveBeenCalledWith('https://meta.ai')
})
it('respects manual scrolling and offers jump to latest', () => {
  const { emit } = setup({ messages: [message({ content: 'First text' })] })
  const thread = document.querySelector('.conversation-thread') as HTMLDivElement
  Object.defineProperties(thread, { scrollHeight: { value: 1000 }, clientHeight: { value: 200 } })
  thread.scrollTop = 100
  fireEvent.scroll(thread)
  emit(message({ content: 'More text', sequence: 2 }))
  expect(thread.scrollTop).toBe(100)
  fireEvent.click(screen.getByRole('button', { name: 'Jump to latest' }))
  expect(thread.scrollTop).toBe(1000)
})
it('merges newer snapshots once in message order', () => {
  const existing = [message()]
  expect(mergeMessages(existing, [message()])).toBe(existing)
  const merged = mergeMessages(existing, [
    message({ sequence: 3 }),
    message({ sequence: 2 }),
    message({ id: 'user', role: 'user', ordinal: 1 }),
  ])
  expect(merged.map((m) => m.id)).toEqual(['user', 'reply'])
  expect(merged[1].sequence).toBe(3)
})
