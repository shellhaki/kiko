import type { CreatedFile } from './chat-files'
import { useEffect, useState } from 'react'
import { ArrowUpRight, FileSpreadsheet, FileText, Presentation } from 'lucide-react'
import type { Run, WorkspaceFile } from './api'
import { desktop } from './api'
import { Dialog, Icon } from './components/ui'

export function FileCards({ files, run }: { files: CreatedFile[]; run: Run }) {
  return (
    <div className="chat-files">
      {files.map((file) => (
        <button
          type="button"
          key={file.id}
          className="chat-file"
          onClick={() =>
            void run(() =>
              file.kind === 'document'
                ? desktop().OpenDocument(file.id)
                : desktop().OpenArtifact(file.id),
            )
          }
        >
          <span className="chat-file-icon">
            <Icon
              icon={
                file.kind === 'sheet'
                  ? FileSpreadsheet
                  : file.kind === 'deck'
                    ? Presentation
                    : FileText
              }
              size={21}
            />
          </span>
          <span>
            <strong>{file.title}</strong>
            <small>{file.type}</small>
          </span>
          <Icon icon={ArrowUpRight} size={16} />
        </button>
      ))}
    </div>
  )
}
export function WorkspaceFiles({
  chat,
  stamp,
  onOpen,
}: {
  chat: string
  stamp: string
  onOpen?(path: string): void
}) {
  const [files, setFiles] = useState<WorkspaceFile[]>([])
  const [preview, setPreview] = useState<{ path: string; content: string } | null>(null)
  const [issues, setIssues] = useState<{ sourceId: string; message: string }[]>([])
  const [retrying, setRetrying] = useState('')
  const [error, setError] = useState('')
  useEffect(() => {
    let active = true
    const refresh = () => {
      if (chat) {
        void desktop()
          .WorkspaceIssues(chat)
          .then((value) => {
            if (active) setIssues(value)
          })
          .catch(() => {
            if (active) setError('Couldn’t check workspace file saves.')
          })
        void desktop()
          .WorkspaceFiles(chat)
          .then((value) => {
            if (active) setFiles(value.filter((file) => !file.sourceId))
          })
          .catch(() => {
            if (active) setError('Couldn’t load workspace files.')
          })
      }
    }
    refresh()
    const timer = setInterval(refresh, 2000)
    return () => {
      clearInterval(timer)
      active = false
    }
  }, [chat, stamp])
  return (
    <>
      {issues.map((issue) => (
        <div className="workspace-sync-issue" key={issue.sourceId} role="status">
          <p>Saved in Kiko. Folder copy needs attention.</p>
          <small>{issue.message}</small>
          <button
            type="button"
            disabled={!!retrying}
            onClick={async () => {
              setRetrying(issue.sourceId)
              setError('')
              try {
                await desktop().RetryWorkspaceSync(chat, issue.sourceId)
                setIssues(await desktop().WorkspaceIssues(chat))
              } catch (problem) {
                setError(String(problem))
              } finally {
                setRetrying('')
              }
            }}
          >
            {retrying === issue.sourceId ? 'Saving…' : 'Retry folder save'}
          </button>
        </div>
      ))}
      {!!files.length && (
        <div className="chat-files" aria-label="Workspace files">
          {files.map((file) => (
            <button
              type="button"
              className="chat-file"
              key={file.path}
              onClick={() =>
                onOpen
                  ? onOpen(file.path)
                  : void desktop()
                      .ReadWorkspaceFile(chat, file.path)
                      .then(setPreview)
                      .catch((problem) => setError(String(problem)))
              }
            >
              <Icon icon={FileText} />
              <span>
                <strong>{file.path}</strong>
                <small>Saved in project folder</small>
              </span>
              <Icon icon={ArrowUpRight} size={16} />
            </button>
          ))}
        </div>
      )}
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      <Dialog
        open={!!preview}
        onOpenChange={(open) => {
          if (!open) setPreview(null)
        }}
        title={preview?.path ?? 'File'}
        description="Current file content in your project folder."
      >
        <pre className="permission-preview">{preview?.content}</pre>
      </Dialog>
    </>
  )
}
