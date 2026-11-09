import { useEffect, useState } from 'react'
import { desktop } from './api'
import { Button, IconButton } from './components/ui'
import { Minus, Plus } from 'lucide-react'

export function PDFPageImage({ chat, id, pages }: { chat: string; id: string; pages: number }) {
  const [page, setPage] = useState(1)
  const [attempt, setAttempt] = useState(0)
  const [zoom, setZoom] = useState(1)
  const [result, setResult] = useState({ page: 0, attempt: -1, image: '', error: '' })
  useEffect(() => {
    let active = true
    void desktop()
      .RenderAttachmentPage(chat, id, page)
      .then((image) => {
        if (active) setResult({ page, attempt, image, error: '' })
      })
      .catch((problem) => {
        if (active) setResult({ page, attempt, image: '', error: String(problem) })
      })
    return () => {
      active = false
    }
  }, [chat, id, page, attempt])
  const busy = result.page !== page || result.attempt !== attempt
  return (
    <div className="pdf-preview">
      <div className="pdf-controls">
        <Button
          variant="ghost"
          disabled={busy || page <= 1}
          onClick={() => setPage((value) => value - 1)}
        >
          Previous page
        </Button>
        <span>
          Page {page} of {pages}
        </span>
        <Button
          variant="ghost"
          disabled={busy || page >= pages}
          onClick={() => setPage((value) => value + 1)}
        >
          Next page
        </Button>
      </div>
      <div className="pdf-controls">
        <IconButton
          icon={Minus}
          label="Zoom out page image"
          disabled={zoom <= 0.75}
          onClick={() => setZoom((value) => Math.max(0.75, value - 0.25))}
        />
        <output aria-label="Page image zoom">{Math.round(zoom * 100)}%</output>
        <IconButton
          icon={Plus}
          label="Zoom in page image"
          disabled={zoom >= 3}
          onClick={() => setZoom((value) => Math.min(3, value + 0.25))}
        />
        <Button variant="ghost" onClick={() => setZoom(1)}>
          Fit width
        </Button>
      </div>
      {busy ? (
        <p role="status">Rendering this page locally…</p>
      ) : result.error ? (
        <>
          <p role="alert">{result.error}</p>
          <Button variant="secondary" onClick={() => setAttempt((value) => value + 1)}>
            Retry page
          </Button>
        </>
      ) : (
        <div
          className="pdf-page-scroll"
          tabIndex={0}
          role="region"
          aria-label="Scrollable PDF page image"
        >
          <img style={{ width: `${zoom * 100}%` }} src={result.image} alt={`PDF page ${page}`} />
        </div>
      )}
    </div>
  )
}
