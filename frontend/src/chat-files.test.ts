import { expect, it } from 'vitest'
import { chatFiles, recentFiles, replyFiles } from './chat-files'
import type { ChatMessage, State } from './api'

it('places created files with their first successful reply and keeps manual files separate', () => {
  const state = {
    selection: { chatId: 'chat' },
    documents: [
      { id: 'doc', chatId: 'chat', title: 'Plan' },
      { id: 'manual', chatId: 'chat', title: 'Notes' },
      { id: 'private', chatId: 'other', title: 'Private' },
    ],
    artifacts: [{ id: 'sheet', chatId: 'chat', title: 'Budget', kind: 'sheet' }],
  } as State
  const messages = [
    {
      id: 'reply',
      steps: [
        { status: 'ok', outputId: 'doc' },
        { status: 'refused', outputId: 'sheet' },
      ],
    },
    {
      id: 'later',
      steps: [
        { status: 'ok', outputId: 'doc' },
        { status: 'ok', outputId: 'sheet' },
      ],
    },
  ] as ChatMessage[]
  expect(chatFiles(state).map((file) => file.id)).toEqual(['doc', 'manual', 'sheet'])
  expect(replyFiles(state, messages, messages[0]).map((file) => file.id)).toEqual(['doc'])
  expect(replyFiles(state, messages, messages[1]).map((file) => file.id)).toEqual(['sheet'])
  expect(replyFiles(state, messages).map((file) => file.id)).toEqual(['manual'])
})

it('returns the newest files across chats for the home recents row', () => {
  const state = {
    selection: { chatId: 'chat' },
    documents: [
      { id: 'one', chatId: 'chat', title: 'One' },
      { id: 'two', chatId: 'other', title: 'Two' },
      { id: 'three', chatId: 'chat', title: 'Three' },
    ],
    artifacts: [{ id: 'sheet', chatId: 'other', title: 'Budget', kind: 'sheet' }],
  } as State
  expect(recentFiles(state, 2).map((file) => file.id)).toEqual(['sheet', 'three'])
  expect(recentFiles(state).map((file) => file.id)).toEqual(['sheet', 'three', 'two', 'one'])
})
