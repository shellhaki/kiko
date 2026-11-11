import { useEffect, useState } from 'react'
import { desktop, type FilePermission } from './api'
import { Button } from './components/ui'

const deleteTitles: Record<string, { title: string; action: string; label: string }> = {
  delete: { title: 'Delete this file?', action: 'Delete file', label: 'Approve file deletion' },
  document: {
    title: 'Delete this document?',
    action: 'Delete document',
    label: 'Approve document deletion',
  },
  sheet: {
    title: 'Delete this spreadsheet?',
    action: 'Delete spreadsheet',
    label: 'Approve spreadsheet deletion',
  },
  deck: {
    title: 'Delete this slide deck?',
    action: 'Delete slide deck',
    label: 'Approve slide deck deletion',
  },
}
export function DeleteApproval({ chat }: { chat: string }) {
  const [pending, setPending] = useState<FilePermission[]>([])
  const [busy, setBusy] = useState(false)
  useEffect(() => {
    if (!chat) return
    let live = true
    const load = async () => {
      try {
        const rows = await desktop().FilePermissions(chat)
        if (live) setPending(rows.filter((row) => row.kind !== 'write'))
      } catch {
        if (live) setPending([])
      }
    }
    void load()
    const timer = window.setInterval(load, 2000)
    return () => {
      live = false
      window.clearInterval(timer)
    }
  }, [chat])
  const current = pending[0]
  if (!current || current.chatId !== chat) return null
  const copy = deleteTitles[current.kind] ?? deleteTitles.delete
  const decide = async (approved: boolean) => {
    setBusy(true)
    try {
      await desktop().ResolveFilePermission(current.id, approved)
    } finally {
      setPending((rows) => rows.filter((row) => row.id !== current.id))
      setBusy(false)
    }
  }
  return (
    <div className="delete-popover" role="alertdialog" aria-label={copy.label}>
      <div>
        <div className="delete-pop-title">{copy.title}</div>
        <div className="delete-pop-path">{current.path}</div>
      </div>
      <div className="delete-pop-actions">
        <Button variant="ghost" disabled={busy} onClick={() => void decide(false)}>
          Keep
        </Button>
        <Button variant="danger" disabled={busy} onClick={() => void decide(true)}>
          {copy.action}
        </Button>
      </div>
    </div>
  )
}
