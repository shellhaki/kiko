let flush: (() => Promise<void>) | undefined
export function registerDocumentSave(save: () => Promise<void>) {
  flush = save
  return () => {
    if (flush === save) flush = undefined
  }
}
export async function flushDocument() {
  await flush?.()
}
