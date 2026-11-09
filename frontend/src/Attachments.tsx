import type { ReactNode } from 'react'
import { PDFPageImage } from './PDFPageImage'
import { Component, lazy, Suspense, useCallback, useEffect, useRef, useState } from 'react'
import { FileText, Plus, Trash2 } from 'lucide-react'
import { desktop } from './api'
import type { Attachment } from './api'
import { Button, Dialog, Icon, IconButton } from './components/ui'

const PDFViewer = lazy(() => import('./PDFViewer'))
export function Attachments({ chat, disabled }: { chat: string; disabled: boolean }) {
  const [files, setFiles] = useState<Attachment[]>([])
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [localPDF, setLocalPDF] = useState(false)
  const [preview, setPreview] = useState<{
    id: string
    pages: number
    name: string
    text: string
    image: string
    pdf?: string
  } | null>(null)
  const input = useRef<HTMLInputElement>(null)
  const refresh = useCallback(() => {
    if (chat)
      void desktop()
        .Attachments(chat)
        .then(setFiles)
        .catch(() => setError('Couldn’t load attachments.'))
  }, [chat])
  useEffect(refresh, [refresh])
  const upload = async (file: File) => {
    if (!chat) return
    if (file.size > 20 * 1024 * 1024) {
      setError('Choose a file up to 20 MB.')
      return
    }
    setBusy(true)
    setError('')
    try {
      const encoded = await new Promise<string>((resolve, reject) => {
        const reader = new FileReader()
        reader.onload = () => resolve(String(reader.result).split(',')[1])
        reader.onerror = () => reject(new Error('Couldn’t read this file.'))
        reader.readAsDataURL(file)
      })
      await desktop().AttachFile(chat, file.name, encoded)
      refresh()
    } catch (problem) {
      setError(String(problem))
    } finally {
      setBusy(false)
      if (input.current) input.current.value = ''
    }
  }
  const open = async (file: Attachment) => {
    try {
      const pages = await desktop().ReadAttachment(chat, file.id, 0)
      const image = await desktop().AttachmentPreview(chat, file.id)
      setLocalPDF(false)
      setPreview({
        id: file.id,
        pages: file.pages,
        pdf:
          file.mime === 'application/pdf'
            ? await desktop().AttachmentBytes(chat, file.id)
            : undefined,
        name: file.name,
        image,
        text: pages.map((page) => `## ${page.name}\n\n${page.text}`).join('\n\n'),
      })
    } catch (problem) {
      setError(String(problem))
    }
  }
  return (
    <div className="attachments">
      <div className="attachment-controls">
        <IconButton
          icon={Plus}
          label={!chat ? 'Start a chat to attach files' : busy ? 'Importing file' : 'Attach file'}
          disabled={!chat || disabled || busy}
          onClick={() => input.current?.click()}
        />
        <input
          ref={input}
          type="file"
          hidden
          aria-label="Choose attachment"
          accept=".png,.jpg,.jpeg,.webp,.pdf,.docx,.xlsx,.pptx,.csv,.md,.txt,.go,.js,.ts,.tsx,.py,.rs,.json,.yaml,.sql,.css"
          onChange={(event) => {
            const file = event.target.files?.[0]
            if (file) void upload(file)
          }}
        />
        <small>
          {busy
            ? 'Importing locally…'
            : files.length
              ? `${files.length} attached · available to the model on send`
              : chat
                ? 'Images, PDF, Office, or text'
                : 'Start a chat to attach files'}
        </small>
      </div>
      {files.length > 0 && (
        <div className="attachment-chips">
          {files.map((file) => (
            <div className="attachment-chip" key={file.id}>
              <button type="button" onClick={() => void open(file)}>
                <Icon icon={FileText} size={14} />
                {file.name}
                <small>
                  {file.pages} {file.mime.startsWith('image/') ? 'image' : 'pages/sheets'}
                </small>
              </button>
              <IconButton
                icon={Trash2}
                label={`Remove ${file.name}`}
                disabled={disabled || busy}
                onClick={() => {
                  if (window.confirm(`Remove “${file.name}” from this chat?`))
                    void desktop()
                      .RemoveAttachment(chat, file.id)
                      .then(refresh)
                      .catch((problem) => setError(String(problem)))
                }}
              />
            </div>
          ))}
        </div>
      )}
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      <Dialog
        open={preview !== null}
        onOpenChange={(open) => {
          if (!open) setPreview(null)
        }}
        title={preview?.name ?? 'Attachment'}
        description="Local preview. Extracted text may omit images, charts, or complex formatting."
      >
        {preview?.pdf ? (
          <>
            <Button variant="secondary" onClick={() => setLocalPDF((value) => !value)}>
              {localPDF ? 'Use browser PDF preview' : 'Use local page images'}
            </Button>
            {localPDF ? (
              <PDFPageImage key={preview.id} chat={chat} id={preview.id} pages={preview.pages} />
            ) : (
              <PDFPreviewBoundary key={preview.id}>
                <Suspense fallback={<p>Opening PDF…</p>}>
                  <PDFViewer encoded={preview.pdf} />
                </Suspense>
              </PDFPreviewBoundary>
            )}
          </>
        ) : preview?.image ? (
          <img className="attachment-image" src={preview.image} alt={preview.name} />
        ) : (
          <pre className="attachment-text">{preview?.text}</pre>
        )}
        {preview?.pdf && (
          <details>
            <summary>Extracted text</summary>
            <pre className="attachment-text">{preview.text}</pre>
          </details>
        )}
        <Button variant="secondary" onClick={() => setPreview(null)}>
          Close preview
        </Button>
      </Dialog>
    </div>
  )
}

class PDFPreviewBoundary extends Component<{ children: ReactNode }, { failed: boolean }> {
  state = { failed: false }
  static getDerivedStateFromError() {
    return { failed: true }
  }
  render() {
    return this.state.failed ? (
      <p role="alert">
        The browser preview could not load. Use local page images or extracted text.
      </p>
    ) : (
      this.props.children
    )
  }
}
