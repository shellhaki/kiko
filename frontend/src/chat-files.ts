import type { ChatMessage, State } from './api'
export function greeting(name: string): string {
  const hour = new Date().getHours()
  const part = hour < 5 ? 'evening' : hour < 12 ? 'morning' : hour < 18 ? 'afternoon' : 'evening'
  return `Good ${part}, ${name.trim() || 'there'}`
}
export type CreatedFile = { id: string; title: string; type: string; kind: string }
export function chatFiles(state: State): CreatedFile[] {
  const chat = state.selection.chatId
  return [
    ...state.documents
      .filter((file) => file.chatId === chat)
      .map((file) => ({ id: file.id, title: file.title, type: 'Document', kind: 'document' })),
    ...(state.artifacts ?? [])
      .filter((file) => file.chatId === chat)
      .map((file) => ({
        id: file.id,
        title: file.title,
        type: file.kind === 'sheet' ? 'Spreadsheet' : 'Slides',
        kind: file.kind,
      })),
  ]
}
export function recentFiles(state: State, limit = 4): CreatedFile[] {
  return [
    ...state.documents.map((file) => ({
      id: file.id,
      title: file.title,
      type: 'Document',
      kind: 'document',
    })),
    ...(state.artifacts ?? []).map((file) => ({
      id: file.id,
      title: file.title,
      type: file.kind === 'sheet' ? 'Spreadsheet' : 'Slides',
      kind: file.kind,
    })),
  ]
    .slice(-limit)
    .reverse()
}
export function replyFiles(state: State, messages: ChatMessage[], message?: ChatMessage) {
  const owners = new Map<string, string>()
  for (const reply of messages)
    for (const step of reply.steps ?? [])
      if (step.status === 'ok' && step.outputId && !owners.has(step.outputId))
        owners.set(step.outputId, reply.id)
  return chatFiles(state).filter((file) =>
    message ? owners.get(file.id) === message.id : !owners.has(file.id),
  )
}
