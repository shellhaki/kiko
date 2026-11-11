import { afterEach, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { Conversation } from './Conversation'
import { greeting } from './chat-files'
import { UIProvider } from './components/ui'
import type { Backend, ChatMessage, State } from './api'
import { mergeMessages } from './conversation-state'

it('copies assistant responses and user messages', async () => {
  const writeText = vi.fn().mockResolvedValue(undefined)
  Object.assign(navigator, { clipboard: { writeText } })
  try {
    setup({
      messages: [
        message({
          id: 'user',
          role: 'user',
          ordinal: 1,
          content: 'Do the thing',
          status: 'complete',
        }),
        message({ id: 'reply', content: 'Did the thing', status: 'complete' }),
      ],
    })
    fireEvent.click(await screen.findByRole('button', { name: 'Copy message' }))
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('Do the thing'))
    fireEvent.click(await screen.findByRole('button', { name: 'Copy response' }))
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('Did the thing'))
  } finally {
    delete (navigator as unknown as Record<string, unknown>).clipboard
  }
})

it('greets by time of day with the saved nickname', () => {
  vi.useFakeTimers()
  try {
    vi.setSystemTime(new Date(2026, 5, 10, 9, 30))
    expect(greeting('Haki')).toBe('Good morning, Haki')
    vi.setSystemTime(new Date(2026, 5, 10, 15, 0))
    expect(greeting('Haki')).toBe('Good afternoon, Haki')
    vi.setSystemTime(new Date(2026, 5, 10, 21, 0))
    expect(greeting('  ')).toBe('Good evening, there')
  } finally {
    vi.useRealTimers()
  }
})

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
    model: 'muse-spark-1.3-contributor',
    models: [
      {
        id: 'muse-spark-1.3-contributor',
        name: 'Muse Spark 1.3',
        mode: 'contributor',
        description: 'Lower price.',
      },
      { id: 'muse-ember-1.0', name: 'Muse Ember', mode: 'standard', description: 'Fast.' },
    ],
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
  vi.unstubAllGlobals()
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
it('never sends a pasted draft and tells the user Enter sends', async () => {
  const { api } = setup()
  const pasted = 'A long pasted draft that should wait for an explicit send.'
  fireEvent.change(screen.getByRole('textbox', { name: 'Message' }), { target: { value: pasted } })
  await waitFor(() => expect(api.SaveDraft).toHaveBeenCalledWith('', 'chat', pasted))
  expect(api.SendMessage).not.toHaveBeenCalled()
  expect(screen.getByText(/Enter to send/)).toBeTruthy()
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

it('updates typing immediately and coalesces draft writes, flushing the last edit on unmount', async () => {
  vi.useFakeTimers()
  try {
    const { api, view } = setup()
    const input = screen.getByRole('textbox', { name: 'Message' }) as HTMLTextAreaElement
    fireEvent.change(input, { target: { value: 'A' } })
    fireEvent.change(input, { target: { value: 'A thoughtful' } })
    fireEvent.change(input, { target: { value: 'A thoughtful draft' } })
    expect(input.value).toBe('A thoughtful draft')
    expect(api.SaveDraft).not.toHaveBeenCalled()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(200)
    })
    expect(api.SaveDraft).toHaveBeenCalledTimes(1)
    expect(api.SaveDraft).toHaveBeenLastCalledWith('', 'chat', 'A thoughtful draft')
    fireEvent.change(input, { target: { value: 'Last edit' } })
    view.unmount()
    await act(async () => {
      await Promise.resolve()
    })
    expect(api.SaveDraft).toHaveBeenLastCalledWith('', 'chat', 'Last edit')
  } finally {
    vi.useRealTimers()
  }
})
it('uses smooth jump scrolling and honors reduced motion', () => {
  setup({ messages: [message({ content: 'First text' })] })
  const thread = document.querySelector('.conversation-thread') as HTMLDivElement
  Object.defineProperties(thread, { scrollHeight: { value: 1000 }, clientHeight: { value: 200 } })
  const scroll = vi.fn()
  thread.scrollTo = scroll
  vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({ matches: false }))
  thread.scrollTop = 100
  fireEvent.scroll(thread)
  fireEvent.click(screen.getByRole('button', { name: 'Jump to latest' }))
  expect(scroll).toHaveBeenLastCalledWith({ top: 1000, behavior: 'smooth' })
  vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({ matches: true }))
  fireEvent.wheel(thread)
  thread.scrollTop = 100
  fireEvent.scroll(thread)
  fireEvent.click(screen.getByRole('button', { name: 'Jump to latest' }))
  expect(scroll).toHaveBeenLastCalledWith({ top: 1000, behavior: 'instant' })
  vi.unstubAllGlobals()
})

it('shows the chat model and locks the model choice while a reply is running', () => {
  setup()
  const idle = screen.getByRole('button', { name: 'Model' }) as HTMLButtonElement
  expect(idle.textContent).toContain('Muse Spark 1.3 · Contributor')
  expect(idle.disabled).toBe(false)
  cleanup()
  setup({ messages: [message()] })
  expect((screen.getByRole('button', { name: 'Model' }) as HTMLButtonElement).disabled).toBe(true)
})
