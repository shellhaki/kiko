import { useEffect, useRef, useState } from 'react'
import { getDocument, GlobalWorkerOptions } from 'pdfjs-dist'
import workerURL from 'pdfjs-dist/build/pdf.worker.min.mjs?url'
import { Button } from './components/ui'
GlobalWorkerOptions.workerSrc = workerURL
export default function PDFViewer({ encoded }: { encoded: string }) {
  const canvas = useRef<HTMLCanvasElement>(null)
  const [page, setPage] = useState(1)
  const [count, setCount] = useState(1)
  const [error, setError] = useState('')
  useEffect(() => {
    let disposed = false
    const task = getDocument({
      data: Uint8Array.from(atob(encoded), (character) => character.charCodeAt(0)),
      enableXfa: false,
      maxImageSize: 16000000,
      standardFontDataUrl: '/pdfjs/standard_fonts/',
      cMapUrl: '/pdfjs/cmaps/',
      cMapPacked: true,
      wasmUrl: '/pdfjs/wasm/',
    })
    void task.promise
      .then(async (document) => {
        if (disposed) return
        setCount(document.numPages)
        const source = await document.getPage(page)
        if (disposed || !canvas.current) return
        const bounds = source.getViewport({ scale: 1 })
        if (
          !Number.isFinite(bounds.width * bounds.height) ||
          bounds.width <= 0 ||
          bounds.height <= 0
        )
          throw new Error('Invalid page size')
        const scale = Math.min(1.25, 2048 / bounds.width, 2048 / bounds.height)
        const viewport = source.getViewport({ scale })
        const target = canvas.current
        target.width = viewport.width
        target.height = viewport.height
        await source.render({ canvas: target, viewport }).promise
      })
      .catch(() => {
        if (!disposed) setError('Couldn’t render this page. Extracted text is still available.')
      })
    return () => {
      disposed = true
      void task.destroy()
    }
  }, [encoded, page])
  return (
    <div className="pdf-preview">
      <div className="pdf-controls">
        <Button variant="ghost" disabled={page <= 1} onClick={() => setPage((value) => value - 1)}>
          Previous
        </Button>
        <span>
          Page {page} of {count}
        </span>
        <Button
          variant="ghost"
          disabled={page >= count}
          onClick={() => setPage((value) => value + 1)}
        >
          Next
        </Button>
      </div>
      {error && <p role="alert">{error}</p>}
      <canvas ref={canvas} aria-label={`PDF page ${page}`} />
    </div>
  )
}
