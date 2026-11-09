import { DeckEditor } from './DeckEditor'
import { useCallback, useEffect, useRef, useState } from 'react'
import { Copy, Download, FileSpreadsheet, Presentation, X } from 'lucide-react'
import { desktop } from './api'
import { Button, Icon, IconButton, PaneHeader } from './components/ui'
import { registerDocumentSave } from './document-session'
import type { ArtifactRecord, Workbook, Deck } from './artifact-model'
import { cellInput, columnName } from './artifact-model'

export function ArtifactPane({ id, close }: { id: string; close(): Promise<void> }) {
  const [data, setData] = useState<ArtifactRecord>()
  const [error, setError] = useState('')
  useEffect(() => {
    let active = true
    void desktop()
      .ReadArtifact(id)
      .then((value) => {
        if (active) setData(value)
      })
      .catch((problem) => {
        if (active) setError(String(problem))
      })
    return () => {
      active = false
    }
  }, [id])
  if (!data)
    return (
      <aside className="document-pane">
        <p className="document-notice" role="status">
          {error || 'Opening artifact…'}
        </p>
        <Button onClick={() => void close()}>Return to chat</Button>
      </aside>
    )
  return <ArtifactEditor key={id} initial={data} close={close} />
}
function ArtifactEditor({ initial, close }: { initial: ArtifactRecord; close(): Promise<void> }) {
  const [record, setRecord] = useState(initial)
  const [payload, setPayload] = useState(initial.draft?.payload ?? initial.payload)
  const [status, setStatus] = useState(initial.draft ? 'Draft recovered' : 'Saved')
  const [error, setError] = useState('')
  const [selectedTab, setTab] = useState(0)
  const [rowStart, setRowStart] = useState(0)
  const [columnStart, setColumnStart] = useState(0)
  const [selected, setSelected] = useState('A1')
  const current = useRef(initial)
  const value = useRef(initial.draft?.payload ?? initial.payload)
  const revision = useRef(initial.draft?.baseRev ?? initial.rev)
  const dirty = useRef(!!initial.draft)
  const generation = useRef(0)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const queue = useRef<Promise<void>>(Promise.resolve())
  const saving = useRef<Promise<void> | null>(null)
  const mounted = useRef(true)
  const flush = useCallback(async () => {
    if (saving.current) return saving.current
    if (!dirty.current) return
    clearTimeout(timer.current)
    const task = (async () => {
      try {
        while (dirty.current) {
          setStatus('Saving…')
          await queue.current
          const snapshot = structuredClone(value.current)
          const number = generation.current
          const saved = await desktop().SaveArtifact(current.current.id, revision.current, snapshot)
          current.current = saved
          revision.current = saved.rev
          if (number === generation.current) dirty.current = false
          if (mounted.current) {
            setRecord(saved)
            setStatus(dirty.current ? 'Unsaved changes' : 'Saved')
            setError('')
          }
        }
      } catch (problem) {
        if (mounted.current) {
          setError(String(problem))
          setStatus('Not saved')
        }
        throw problem
      }
    })()
    saving.current = task
    try {
      await task
    } finally {
      saving.current = null
    }
  }, [])
  useEffect(() => registerDocumentSave(flush), [flush])
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
      clearTimeout(timer.current)
    }
  }, [])
  useEffect(() => {
    const poll = setInterval(() => {
      if (dirty.current || saving.current) return
      void desktop()
        .ReadArtifact(initial.id)
        .then((next) => {
          if (!mounted.current || dirty.current || next.rev === current.current.rev) return
          current.current = next
          revision.current = next.rev
          value.current = next.payload
          setRecord(next)
          setPayload(next.payload)
        })
        .catch(() => {})
    }, 1500)
    return () => clearInterval(poll)
  }, [initial.id])
  const change = (next: Workbook | Deck) => {
    value.current = next
    setPayload(next)
    dirty.current = true
    generation.current++
    setStatus('Unsaved changes')
    const snapshot = structuredClone(next)
    queue.current = queue.current
      .catch(() => {})
      .then(() => desktop().SaveArtifactDraft(record.id, revision.current, snapshot))
    void queue.current.catch((problem) => setError(String(problem)))
    clearTimeout(timer.current)
    timer.current = setTimeout(() => void flush().catch(() => {}), 600)
  }
  const book: Workbook =
    record.kind === 'sheet' ? (payload as Workbook) : { tabs: [{ name: '', cells: {} }] }
  const tab = Math.min(selectedTab, book.tabs.length - 1)
  const activeTab = book.tabs[tab]
  const edit = (address: string, text: string) => {
    const next = structuredClone(book)
    const old = next.tabs[tab].cells?.[address] ?? {}
    next.tabs[tab].cells = {
      ...next.tabs[tab].cells,
      [address]: { ...old, ...cellInput(text), formula: text.startsWith('=') ? text : undefined },
    }
    change(next)
  }
  const format = (style: Partial<Workbook['tabs'][number]['cells'][string]>) => {
    const next = structuredClone(book)
    next.tabs[tab].cells = {
      ...next.tabs[tab].cells,
      [selected]: { ...next.tabs[tab].cells?.[selected], ...style },
    }
    change(next)
  }
  const exportFile = async (format: string) => {
    try {
      await flush()
      if (await desktop().ExportArtifact(record.id, format, activeTab.name)) setStatus('Exported')
    } catch (problem) {
      setError(String(problem))
    }
  }
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(JSON.stringify(value.current, null, 2))
      setStatus('Draft copied')
    } catch {
      setError('Select the current values and copy them manually.')
    }
  }
  return (
    <aside
      className="document-pane artifact-pane"
      aria-label={record.kind === 'sheet' ? 'Spreadsheet pane' : 'Slide deck pane'}
    >
      <PaneHeader>
        <Icon icon={record.kind === 'sheet' ? FileSpreadsheet : Presentation} />
        <strong className="document-title">{record.title}</strong>
        <small className="save-indicator" role="status">
          {status} · rev {record.rev}
        </small>
        <span className="spacer" />
        <IconButton icon={Copy} label="Copy artifact draft" onClick={() => void copy()} />
        <IconButton
          icon={Download}
          label={record.kind === 'sheet' ? 'Export XLSX' : 'Export PPTX'}
          onClick={() => void exportFile(record.kind === 'sheet' ? 'xlsx' : 'pptx')}
        />
        <Button
          variant="ghost"
          onClick={() => void exportFile(record.kind === 'sheet' ? 'csv' : 'pdf')}
        >
          {record.kind === 'sheet' ? 'CSV' : 'PDF'}
        </Button>
        <IconButton
          icon={X}
          label="Close artifact"
          onClick={() =>
            void flush()
              .then(close)
              .catch(() => {})
          }
        />
      </PaneHeader>
      {error && (
        <div className="document-notice" role="alert">
          <p>{error}</p>
          <Button onClick={() => void flush().catch(() => {})}>Retry save</Button>
          <Button
            variant="secondary"
            onClick={async () => {
              if (
                !window.confirm(
                  'Discard this local draft and load the saved artifact? Copy it first if you need it.',
                )
              )
                return
              clearTimeout(timer.current)
              try {
                await saving.current?.catch(() => {})
                await queue.current.catch(() => {})
                await desktop().DiscardArtifactDraft(record.id)
                const next = await desktop().ReadArtifact(record.id)
                dirty.current = false
                current.current = next
                revision.current = next.rev
                value.current = next.payload
                setRecord(next)
                setPayload(next.payload)
                setTab(0)
                setError('')
                setStatus('Saved version loaded')
              } catch (problem) {
                setError(String(problem))
              }
            }}
          >
            Discard draft and reload
          </Button>
        </div>
      )}
      {record.kind === 'deck' ? (
        <DeckEditor deck={payload as Deck} chat={record.chatId} onChange={change} />
      ) : (
        <>
          <div className="sheet-tabs" role="tablist" aria-label="Workbook tabs">
            {book.tabs.map((item, index) => (
              <button
                role="tab"
                aria-selected={index === tab}
                key={item.name}
                onClick={() => setTab(index)}
              >
                {item.name}
              </button>
            ))}
            <Button
              variant="ghost"
              onClick={() => {
                const next = structuredClone(book)
                let number = next.tabs.length + 1
                while (next.tabs.some((t) => t.name === 'Sheet' + number)) number++
                next.tabs.push({ name: 'Sheet' + number, cells: {} })
                change(next)
                setTab(next.tabs.length - 1)
              }}
            >
              Add tab
            </Button>
          </div>
          <div className="sheet-formula">
            <label>
              Tab name{' '}
              <input
                aria-label="Tab name"
                value={activeTab.name}
                onChange={(event) => {
                  const next = structuredClone(book)
                  next.tabs[tab].name = event.target.value
                  change(next)
                }}
              />
            </label>
            <Button
              variant="ghost"
              disabled={book.tabs.length === 1}
              onClick={() => {
                if (!window.confirm('Delete this tab and its cells?')) return
                const next = structuredClone(book)
                next.tabs.splice(tab, 1)
                setTab(0)
                change(next)
              }}
            >
              Delete tab
            </Button>
            <label>
              Number format{' '}
              <select
                aria-label="Number format"
                value={activeTab.cells?.[selected]?.format ?? ''}
                onChange={(event) => format({ format: event.target.value })}
              >
                <option value="">General</option>
                <option value="0.00">Decimal</option>
                <option value="0%">Percent</option>
                <option value="$#,##0.00">Currency</option>
                <option value="yyyy-mm-dd">Date</option>
              </select>
            </label>
            <label>
              Cell fill{' '}
              <input
                aria-label="Cell fill"
                type="color"
                value={'#' + (activeTab.cells?.[selected]?.fill || 'ffffff')}
                onChange={(event) => format({ fill: event.target.value.slice(1) })}
              />
            </label>
            <Button variant="ghost" onClick={() => format({ fill: '' })}>
              Clear fill
            </Button>
          </div>
          <div className="sheet-formula">
            <strong>{selected}</strong>
            <input
              aria-label="Formula or value"
              value={String(
                activeTab.cells?.[selected]?.formula ?? activeTab.cells?.[selected]?.value ?? '',
              )}
              onChange={(event) => edit(selected, event.target.value)}
            />
            <Button
              variant="ghost"
              onClick={() => format({ bold: !activeTab.cells?.[selected]?.bold })}
            >
              Bold
            </Button>
          </div>
          <div className="sheet-scroll">
            <table className="sheet-grid">
              <thead>
                <tr>
                  <th />
                  {Array.from({ length: 12 }, (_, index) => (
                    <th key={index}>{columnName(columnStart + index)}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {Array.from({ length: 30 }, (_, r) => (
                  <tr key={r}>
                    <th>{rowStart + r + 1}</th>
                    {Array.from({ length: 12 }, (_, c) => {
                      const address = columnName(columnStart + c) + (rowStart + r + 1)
                      const cell = activeTab.cells?.[address]
                      const display = record.values?.[activeTab.name]?.[address]
                      return (
                        <td
                          key={address}
                          data-selected={selected === address}
                          style={{
                            backgroundColor: cell?.fill ? '#' + cell.fill : undefined,
                            fontWeight: cell?.bold ? 600 : 400,
                          }}
                        >
                          <input
                            aria-label={`${activeTab.name} ${address}`}
                            title={display?.error || cell?.formula || ''}
                            value={
                              selected === address
                                ? String(cell?.formula ?? cell?.value ?? '')
                                : (display?.value ?? String(cell?.value ?? ''))
                            }
                            onFocus={() => setSelected(address)}
                            onChange={(event) => edit(address, event.target.value)}
                            onKeyDown={(event) => {
                              if (event.key === 'Enter') {
                                event.preventDefault()
                                void flush().catch(() => {})
                              }
                            }}
                          />
                        </td>
                      )
                    })}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="sheet-paging">
            <Button
              variant="ghost"
              disabled={rowStart === 0}
              onClick={() => setRowStart(Math.max(0, rowStart - 30))}
            >
              Previous rows
            </Button>
            <span>
              {rowStart + 1}–{rowStart + 30}
            </span>
            <Button
              variant="ghost"
              disabled={rowStart >= 970}
              onClick={() => setRowStart(Math.min(970, rowStart + 30))}
            >
              Next rows
            </Button>
            <Button
              variant="ghost"
              disabled={columnStart === 0}
              onClick={() => setColumnStart(Math.max(0, columnStart - 12))}
            >
              Previous columns
            </Button>
            <Button
              variant="ghost"
              disabled={columnStart >= 88}
              onClick={() => setColumnStart(Math.min(88, columnStart + 12))}
            >
              Next columns
            </Button>
          </div>
        </>
      )}
    </aside>
  )
}
