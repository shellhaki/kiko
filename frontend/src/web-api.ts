import type { Backend, ChatMessage } from './api'
export class WebAuthenticationError extends Error {}
let source: EventSource | undefined
const listeners = new Set<(message: ChatMessage) => void>()
async function rpc(method: string, args: unknown[]) {
  const response = await fetch('/api/rpc', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ method, args }),
    credentials: 'same-origin',
  })
  if (response.status === 401) {
    throw new WebAuthenticationError('Sign in to your personal Kiko workspace.')
  }
  if (!response.ok)
    throw new Error('Couldn’t complete this action. Check your connection and retry.')
  const value = (await response.json()) as { result: unknown; error: string }
  if (value.error) throw new Error(value.error)
  return value.result
}
function events() {
  if (source) return
  source = new EventSource('/api/events')
  source.addEventListener('kiko:message', (event) => {
    try {
      const message = JSON.parse((event as MessageEvent).data) as ChatMessage
      for (const listener of listeners) listener(message)
    } catch {}
  })
}
async function download(kind: string, id: string, format: string, tab = '') {
  const response = await fetch('/api/export?' + new URLSearchParams({ kind, id, format, tab }), {
    credentials: 'same-origin',
  })
  if (!response.ok) throw new Error(await response.text())
  const blob = await response.blob()
  const link = document.createElement('a')
  const url = URL.createObjectURL(blob)
  link.href = url
  const disposition = response.headers.get('Content-Disposition') ?? ''
  const match = /filename\*=UTF-8''(.+)/.exec(disposition)
  link.download = match ? decodeURIComponent(match[1]) : `Kiko.${format}`
  document.body.append(link)
  link.click()
  link.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
  return true
}
export function downloadBlob(name: string, bytes: Uint8Array, mime: string) {
  const link = document.createElement('a')
  const url = URL.createObjectURL(new Blob([bytes.slice().buffer], { type: mime }))
  link.href = url
  link.download = name
  document.body.append(link)
  link.click()
  link.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
const special: Record<string, (...args: unknown[]) => Promise<unknown>> = {
  OpenLink: async (value) => {
    const url = new URL(String(value))
    if (url.protocol !== 'https:' && url.protocol !== 'http:') throw new Error('Unsupported link.')
    window.open(url.href, '_blank', 'noopener,noreferrer')
  },
  OpenMuseGuide: async () => {
    window.open('https://dev.meta.ai/docs/authentication', '_blank', 'noopener,noreferrer')
  },
  CopyDocument: async (id) => {
    const value = (await rpc('ReadDocument', [id, {}])) as { markdown: string; draft?: unknown }
    if (value.draft) throw new Error('Save the draft before copying.')
    await navigator.clipboard.writeText(value.markdown)
  },
  ExportDocument: async (id) => download('document', String(id), 'md'),
  ExportDocumentFormat: async (id, format) => download('document', String(id), String(format)),
  ExportArtifact: async (id, format, tab) =>
    download('artifact', String(id), String(format), String(tab ?? '')),
}
const api = new Proxy(
  {},
  {
    get: (_target, name) =>
      typeof name === 'string'
        ? (special[name] ?? ((...args: unknown[]) => rpc(name, args)))
        : undefined,
  },
) as Backend
export function webBackend(): Backend {
  if (!window.runtime)
    window.runtime = {
      EventsOnMultiple: (_name, callback) => {
        listeners.add(callback)
        events()
        return () => {
          listeners.delete(callback)
          if (!listeners.size) {
            source?.close()
            source = undefined
          }
        }
      },
    }
  return api
}
export async function webLogin(token: string) {
  const response = await fetch('/api/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token }),
    credentials: 'same-origin',
  })
  if (!response.ok) throw new Error(await response.text())
}
