import type { ChatMessage } from './api'
export function mergeMessages(current: ChatMessage[], incoming: ChatMessage[]): ChatMessage[] {
  const byId = new Map(current.map((message) => [message.id, message]))
  let changed = false
  for (const message of incoming) {
    const previous = byId.get(message.id)
    if (!previous || message.sequence > previous.sequence) {
      byId.set(message.id, message)
      changed = true
    }
  }
  if (!changed) return current
  return [...byId.values()].sort((a, b) => a.ordinal - b.ordinal || a.id.localeCompare(b.id))
}
export function requestId() {
  return (
    crypto.randomUUID?.() ??
    [...crypto.getRandomValues(new Uint8Array(16))]
      .map((value) => value.toString(16).padStart(2, '0'))
      .join('')
  )
}
