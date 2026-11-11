let flushConversation: (() => Promise<void>) | undefined
export function registerConversationSave(save: () => Promise<void>) {
  flushConversation = save
  return () => {
    if (flushConversation === save) flushConversation = undefined
  }
}
let flush: (() => Promise<void>) | undefined
export function registerDocumentSave(save: () => Promise<void>) {
  flush = save
  return () => {
    if (flush === save) flush = undefined
  }
}
export async function flushDocument() {
  await flush?.()
  await flushConversation?.()
}
