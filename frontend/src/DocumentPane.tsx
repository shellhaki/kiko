import { usePaneResize } from './use-pane-resize'
import type { Question, State } from './api'
import { questionHighlights } from './document-questions'
import { requestId } from './conversation-state'
import { useCallback, useEffect, useRef, useState } from 'react'
import { EditorContent, useEditor } from '@tiptap/react'
import { Node } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import { TableKit } from '@tiptap/extension-table'
import TaskList from '@tiptap/extension-task-list'
import TaskItem from '@tiptap/extension-task-item'
import UniqueID from '@tiptap/extension-unique-id'
import {
  Bold,
  Check,
  Code,
  Copy,
  FileText,
  Download,
  History,
  Undo2,
  Italic,
  List,
  Maximize2,
  Minimize2,
  Save,
  X,
} from 'lucide-react'
import { desktop } from './api'
import { Button, Dialog, Icon, IconButton, PaneHeader } from './components/ui'
import { fromEditor, toEditor } from './document-model'
import type { Block, DocumentRead, DocumentView } from './document-model'
import { registerDocumentSave } from './document-session'

const Diagram = Node.create({
  name: 'diagram',
  group: 'block',
  atom: true,
  addAttributes: () => ({ svg: { default: '' }, caption: { default: '' }, ref: { default: '' } }),
  parseHTML: () => [{ tag: 'figure[data-diagram]' }],
  renderHTML: ({ HTMLAttributes }) => [
    'figure',
    { 'data-diagram': true, 'data-id': HTMLAttributes.id },
    [
      'img',
      {
        src: 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(String(HTMLAttributes.svg)),
        alt: HTMLAttributes.caption,
      },
    ],
    ['figcaption', {}, HTMLAttributes.caption],
  ],
})
const Pending = Node.create({
  name: 'pending',
  group: 'block',
  content: 'inline*',
  addAttributes: () => ({ intent: { default: '' } }),
  parseHTML: () => [{ tag: 'p[data-pending]' }],
  renderHTML: ({ HTMLAttributes }) => [
    'p',
    { ...HTMLAttributes, 'data-pending': true, 'data-intent': HTMLAttributes.intent },
    0,
  ],
})
const blockNames = [
  'diagram',
  'heading',
  'paragraph',
  'codeBlock',
  'bulletList',
  'orderedList',
  'taskList',
  'listItem',
  'taskItem',
  'table',
  'tableRow',
  'tableCell',
  'tableHeader',
  'pending',
]

export function DocumentPane({
  id,
  close,
  onState,
}: {
  id: string
  close(): Promise<void>
  onState(value: State): void
}) {
  const [data, setData] = useState<DocumentRead>()
  const [error, setError] = useState('')
  const load = useCallback(() => {
    void desktop()
      .ReadDocument(id, {})
      .then(setData)
      .catch(() => setError('Couldn’t open this document. Your saved data is still on disk.'))
  }, [id])
  useEffect(load, [load])
  if (!data)
    return (
      <aside className="document-pane">
        <PaneHeader>
          <strong>Document</strong>
          <span className="spacer" />
          <IconButton icon={X} label="Close document" onClick={() => void close()} />
        </PaneHeader>
        {error ? (
          <div className="document-notice">
            <p role="alert">{error}</p>
            <Button onClick={load}>Retry opening</Button>
          </div>
        ) : (
          <p className="document-notice" role="status">
            Opening document…
          </p>
        )}
      </aside>
    )
  return <EditableDocument key={id} initial={data} close={close} onState={onState} />
}
function EditableDocument({
  initial,
  close,
  onState,
}: {
  initial: DocumentRead
  close(): Promise<void>
  onState(value: State): void
}) {
  const [saved, setSaved] = useState(initial.document)
  const [suggestions, setSuggestions] = useState(initial.suggestions ?? [])
  const [exportFormats, setExportFormats] = useState<string[] | null>(null)
  const [diagramDraft, setDiagramDraft] = useState<{ svg: string; caption: string } | null>(null)
  const [diagramError, setDiagramError] = useState('')
  const [questions, setQuestions] = useState<Question[]>([])
  const [answerError, setAnswerError] = useState('')
  const [answering, setAnswering] = useState(false)
  const [questionSource] = useState(() => ({ current: [] as Question[] }))
  const [highlights] = useState(() => questionHighlights(questionSource))
  const [markdown, setMarkdown] = useState(initial.markdown)
  const [view, setView] = useState<DocumentView>(initial.view ?? {})
  const [status, setStatus] = useState(initial.draft ? 'Unsaved draft recovered' : 'Saved')
  const [error, setError] = useState('')
  const [history, setHistory] = useState<
    { rev: number; author: string; createdAt: number }[] | null
  >(null)
  const [historical, setHistorical] = useState<{ rev: number; markdown: string } | null>(null)
  const [dirty, setDirty] = useState(!!initial.draft)
  const current = useRef(initial.document)
  const blocks = useRef<Block[]>(initial.draft?.blocks ?? initial.document.blocks)
  const baseRev = useRef(initial.draft?.baseRev ?? initial.document.rev)
  const dirtyRef = useRef(!!initial.draft)
  const generation = useRef(0)
  const queue = useRef<Promise<void>>(Promise.resolve())
  const saving = useRef<Promise<void> | null>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const scroll = useRef<HTMLDivElement>(null)
  const viewTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const mounted = useRef(true)
  const editor = useEditor({
    extensions: [
      highlights,
      StarterKit.configure({
        blockquote: false,
        horizontalRule: false,
        underline: false,
        link: { openOnClick: false, autolink: false },
      }),
      TableKit.configure({ table: { resizable: false } }),
      TaskList,
      TaskItem.configure({ nested: true }),
      Pending,
      Diagram,
      UniqueID.configure({ types: blockNames }),
    ],
    content: toEditor(initial.draft?.blocks ?? initial.document.blocks),
    editorProps: {
      attributes: {
        'aria-label': 'Document editor',
        role: 'textbox',
        'aria-multiline': 'true',
        class: 'document-editor prose',
        spellcheck: 'true',
      },
    },
    onUpdate: ({ editor }) => {
      if (!editor.isInitialized) return
      const transaction = editor.state.tr
      editor.state.doc.descendants((node, pos) => {
        if (node.type.name === 'pending' && node.textContent)
          transaction.setNodeMarkup(pos, editor.schema.nodes.paragraph, { id: node.attrs.id })
      })
      if (transaction.docChanged) {
        editor.view.dispatch(transaction)
        return
      }
      try {
        blocks.current = fromEditor(editor.getJSON())
        generation.current++
        dirtyRef.current = true
        setDirty(true)
        setStatus('Unsaved changes')
        setError('')
        const snapshot = structuredClone(blocks.current)
        queue.current = queue.current
          .catch(() => {})
          .then(() => desktop().SaveDocumentDraft(current.current.id, baseRev.current, snapshot))
        void queue.current.catch(() => {
          if (mounted.current)
            setError('Couldn’t save the draft locally. Keep this window open and retry.')
        })
        clearTimeout(timer.current)
        timer.current = setTimeout(() => void flush().catch(() => {}), 600)
      } catch (problem) {
        setError(String(problem))
      }
    },
  })
  const flush = useCallback(async () => {
    if (saving.current) return saving.current
    if (!dirtyRef.current) return
    clearTimeout(timer.current)
    const work = (async () => {
      try {
        while (dirtyRef.current) {
          if (mounted.current) setStatus('Saving…')
          await queue.current
          const snapshot = structuredClone(blocks.current)
          const number = generation.current
          const next = await desktop().SaveDocument(current.current.id, baseRev.current, snapshot)
          current.current = next
          baseRev.current = next.rev
          if (number === generation.current) {
            dirtyRef.current = false
            if (mounted.current) setDirty(false)
          }
          if (mounted.current) {
            setSaved(next)
            setStatus(dirtyRef.current ? 'Unsaved changes' : 'Saved')
            setError('')
          }
        }
        const next = await desktop().ReadDocument(current.current.id, {})
        if (mounted.current) setMarkdown(next.markdown)
      } catch (problem) {
        if (mounted.current) {
          setStatus('Not saved')
          setError(String(problem instanceof Error ? problem.message : problem))
        }
        throw problem
      }
    })()
    saving.current = work
    try {
      await work
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
      clearTimeout(viewTimer.current)
    }
  }, [])
  useEffect(() => {
    if (scroll.current) scroll.current.scrollTop = initial.view?.scroll ?? 0
  }, [initial.view?.scroll])
  useEffect(() => {
    const loadQuestions = () => {
      void desktop()
        .Questions(current.current.id)
        .then((value) => {
          if (!mounted.current) return
          questionSource.current = value
          setQuestions(value)
          editor?.view.dispatch(editor.state.tr.setMeta('questions', true))
        })
        .catch(() => {})
    }
    loadQuestions()
    const poll = setInterval(() => {
      loadQuestions()
      if (dirtyRef.current || saving.current) return
      void desktop()
        .ReadDocument(current.current.id, {})
        .then((next) => {
          if (!mounted.current || dirtyRef.current) return
          setSuggestions(next.suggestions ?? [])
          if (next.document.rev === current.current.rev) return
          const selection = editor?.state.selection
          const known = new Set<string>()
          const collect = (list: Block[]) => {
            for (const block of list) {
              known.add(block.id)
              collect(block.children ?? [])
            }
          }
          collect(current.current.blocks)
          const bookmark = (position: number) => {
            if (!editor) return null
            const resolved = editor.state.doc.resolve(position)
            for (let depth = resolved.depth; depth > 0; depth--) {
              const node = resolved.node(depth)
              if (node.attrs.id && known.has(node.attrs.id))
                return { id: String(node.attrs.id), offset: position - resolved.before(depth) - 1 }
            }
            return null
          }
          const from = selection ? bookmark(selection.from) : null
          const to = selection ? bookmark(selection.to) : null
          current.current = next.document
          baseRev.current = next.document.rev
          blocks.current = next.document.blocks
          editor?.commands.setContent(toEditor(next.document.blocks), { emitUpdate: false })
          if (selection && editor) {
            const locate = (mark: { id: string; offset: number } | null, fallback: number) => {
              let value = Math.min(fallback, editor.state.doc.content.size)
              if (mark)
                editor.state.doc.descendants((node, pos) => {
                  if (node.attrs.id === mark.id)
                    value = pos + 1 + Math.min(mark.offset, node.content.size)
                })
              return value
            }
            editor.commands.setTextSelection({
              from: locate(from, selection.from),
              to: locate(to, selection.to),
            })
          }
          setSaved(next.document)
          setMarkdown(next.markdown)
        })
        .catch(() => {})
    }, 1500)
    return () => clearInterval(poll)
  }, [editor, questionSource])
  const changeView = async (patch: DocumentView) => {
    clearTimeout(viewTimer.current)
    try {
      await flush()
      const next = { ...view, ...patch, scroll: scroll.current?.scrollTop ?? 0 }
      await desktop().SaveDocumentView(saved.id, next)
      clearTimeout(viewTimer.current)
      setView(next)
    } catch {}
  }
  const paneResize = usePaneResize({
    value: view.width ?? 52,
    min: 35,
    max: 75,
    measure: (handle, startX) => {
      const pane = handle.parentElement
      const workspace = pane?.parentElement
      if (!pane || !workspace) return null
      const navigation = [...workspace.children]
        .filter((item) => item.matches('.rail, .workspace-navigation'))
        .reduce((total, item) => total + item.getBoundingClientRect().width, 0)
      const available = workspace.getBoundingClientRect().width - navigation
      if (available <= 0) return null
      const initial = pane.getBoundingClientRect().width
      return (x) => ((initial + startX - x) / available) * 100
    },
    commit: (width) => changeView({ width }),
  })
  const copy = async () => {
    try {
      await flush()
      await desktop().CopyDocument(saved.id)
      setStatus('Markdown copied')
    } catch {
      setError('Couldn’t copy. Save changes and try again.')
    }
  }
  const reload = async () => {
    if (!window.confirm('Discard this unsaved draft and open the latest saved version?')) return
    try {
      await desktop().DiscardDocumentDraft(saved.id)
      const next = await desktop().ReadDocument(saved.id, {})
      current.current = next.document
      baseRev.current = next.document.rev
      blocks.current = next.document.blocks
      dirtyRef.current = false
      generation.current++
      editor?.commands.setContent(toEditor(next.document.blocks), { emitUpdate: false })
      setSaved(next.document)
      setMarkdown(next.markdown)
      setDirty(false)
      setError('')
      setStatus('Saved')
    } catch {
      setError('Couldn’t reload the saved document.')
    }
  }
  const answer = async (id: string, text: string) => {
    setAnswering(true)
    setAnswerError('')
    try {
      await flush()
      const state = await desktop().AnswerQuestion(id, text, requestId())
      setQuestions((value) => value.filter((q) => q.id !== id))
      onState(state)
    } catch (problem) {
      setAnswerError(String(problem))
    } finally {
      setAnswering(false)
    }
  }
  return (
    <aside
      className={`document-pane ${view.focus ? 'focused' : ''}`}
      data-resizing={paneResize.active || undefined}
      style={{
        width: `calc((100% - var(--navigation-width, 64px)) * ${view.focus ? 1 : paneResize.value / 100})`,
      }}
      aria-label="Document pane"
    >
      <div
        className="document-resizer"
        role="separator"
        tabIndex={0}
        aria-label="Document width"
        aria-orientation="vertical"
        aria-valuenow={Math.round(paneResize.value)}
        aria-valuemin={35}
        aria-valuemax={75}
        onKeyDown={(event) => {
          if (event.key === 'Escape') {
            paneResize.cancel()
            return
          }
          if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
            event.preventDefault()
            void changeView({
              width: Math.max(
                35,
                Math.min(75, (view.width ?? 52) + (event.key === 'ArrowLeft' ? 2 : -2)),
              ),
            })
          }
        }}
        {...paneResize.handlers}
      />
      <PaneHeader>
        <Icon icon={FileText} />
        <strong className="document-title">{saved.title}</strong>
        <span className="save-indicator" role="status">
          {!dirty && <Icon icon={Check} size={12} />} {status}
        </span>
        <span className="spacer" />
        <IconButton
          icon={Undo2}
          label="Undo edit"
          onClick={() => editor?.chain().focus().undo().run()}
        />
        <IconButton
          icon={History}
          label="Version history"
          onClick={() =>
            void flush()
              .then(() => desktop().DocumentHistory(saved.id))
              .then(setHistory)
              .catch(() => {})
          }
        />
        <IconButton
          icon={Download}
          label="Export document"
          onClick={() =>
            void flush()
              .then(() => desktop().ExportFormats())
              .then(setExportFormats)
              .catch((problem) => setError(String(problem)))
          }
        />
        <IconButton icon={Copy} label="Copy Markdown" onClick={() => void copy()} />
        <IconButton
          icon={view.focus ? Minimize2 : Maximize2}
          label={view.focus ? 'Restore split view' : 'Focus document'}
          onClick={() => void changeView({ focus: !view.focus })}
        />
        <IconButton
          icon={X}
          label="Close document"
          onClick={() =>
            void flush()
              .then(close)
              .catch(() => {})
          }
        />
      </PaneHeader>
      <div className="document-tabs" role="tablist" aria-label="Document view">
        <button
          role="tab"
          aria-selected={view.tab !== 'markdown'}
          onClick={() => void changeView({ tab: 'document' })}
        >
          Document
        </button>
        <button
          role="tab"
          aria-selected={view.tab === 'markdown'}
          onClick={() => void changeView({ tab: 'markdown' })}
        >
          Markdown
        </button>
        <span className="spacer" />
        <small>Revision {saved.rev}</small>
      </div>
      {error && (
        <div className="document-notice" role="alert">
          <p>{error}</p>
          <Button variant="secondary" onClick={() => void flush().catch(() => {})}>
            <Icon icon={Save} size={14} />
            Retry save
          </Button>
          <Button variant="ghost" onClick={() => void reload()}>
            Reload saved version
          </Button>
        </div>
      )}
      {view.tab !== 'markdown' && (
        <div className="format-toolbar" aria-label="Text formatting">
          {editor?.isActive('diagram') && (
            <Button
              variant="ghost"
              onClick={() => {
                const attrs = editor.getAttributes('diagram')
                setDiagramDraft({ svg: String(attrs.svg), caption: String(attrs.caption) })
                setDiagramError('')
              }}
            >
              Edit diagram
            </Button>
          )}
          <IconButton
            icon={Bold}
            label="Bold"
            aria-pressed={editor?.isActive('bold')}
            onClick={() => editor?.chain().focus().toggleBold().run()}
          />
          <IconButton
            icon={Italic}
            label="Italic"
            aria-pressed={editor?.isActive('italic')}
            onClick={() => editor?.chain().focus().toggleItalic().run()}
          />
          <button onClick={() => editor?.chain().focus().toggleHeading({ level: 2 }).run()}>
            Heading
          </button>
          <IconButton
            icon={List}
            label="Bullet list"
            onClick={() => editor?.chain().focus().toggleBulletList().run()}
          />
          <IconButton
            icon={Code}
            label="Code block"
            onClick={() => editor?.chain().focus().toggleCodeBlock().run()}
          />
        </div>
      )}
      <div
        className="document-scroll"
        ref={scroll}
        onScroll={() => {
          clearTimeout(viewTimer.current)
          if (paneResize.active) return
          viewTimer.current = setTimeout(() => {
            void desktop()
              .SaveDocumentView(saved.id, { ...view, scroll: scroll.current?.scrollTop ?? 0 })
              .catch(() => {})
          }, 300)
        }}
      >
        {view.tab === 'markdown' ? (
          <pre className="markdown-source" tabIndex={0}>
            {markdown}
          </pre>
        ) : (
          <EditorContent editor={editor} />
        )}
      </div>
      {!!suggestions.length && (
        <section className="suggestion-panel" aria-label="Suggested changes">
          <h2>Suggested changes</h2>
          {suggestions.map((proposal) => (
            <details key={proposal.id}>
              <summary>Review change from revision {proposal.baseRev}</summary>
              <div className="suggestion-comparison">
                <div>
                  <strong>Current at proposal</strong>
                  <pre>{proposal.before}</pre>
                </div>
                <div>
                  <strong>Proposed</strong>
                  <pre>{proposal.after}</pre>
                </div>
              </div>
              <Button
                variant="secondary"
                onClick={() =>
                  void flush()
                    .then(() => desktop().AcceptSuggestion(proposal.id))
                    .then((next) => {
                      current.current = next
                      baseRev.current = next.rev
                      blocks.current = next.blocks
                      setSaved(next)
                      editor?.commands.setContent(toEditor(next.blocks), { emitUpdate: false })
                      setSuggestions((value) => value.filter((item) => item.id !== proposal.id))
                      setStatus('Suggestion accepted')
                      void desktop()
                        .ReadDocument(saved.id, {})
                        .then((value) => setMarkdown(value.markdown))
                    })
                    .catch((problem) => setError(String(problem)))
                }
              >
                Accept change
              </Button>
              <Button
                variant="ghost"
                onClick={() =>
                  void desktop()
                    .RejectSuggestion(proposal.id)
                    .then(() =>
                      setSuggestions((value) => value.filter((item) => item.id !== proposal.id)),
                    )
                    .catch((problem) => setError(String(problem)))
                }
              >
                Reject
              </Button>
            </details>
          ))}
        </section>
      )}
      {questions.some((q) => !q.resolved) && (
        <section className="question-panel" aria-label="Document questions">
          {questions
            .filter((q) => !q.resolved)
            .map((q) => (
              <article key={q.id}>
                <button
                  className="question-quote"
                  onClick={() => {
                    const element = editor?.view.dom.querySelector(`[data-question="${q.id}"]`)
                    element?.scrollIntoView({ block: 'center' })
                  }}
                >
                  “{q.quote}”{!q.blockId && ' · passage changed'}
                </button>
                <p>{q.body}</p>
                <div className="suggested-replies">
                  {q.suggested?.map((reply) => (
                    <Button
                      key={reply}
                      variant="secondary"
                      disabled={answering}
                      onClick={() => void answer(q.id, reply)}
                    >
                      {reply}
                    </Button>
                  ))}
                </div>
                <form
                  onSubmit={(event) => {
                    event.preventDefault()
                    void answer(q.id, String(new FormData(event.currentTarget).get('answer')))
                  }}
                >
                  <input aria-label="Answer question" name="answer" required maxLength={4000} />
                  <Button type="submit" disabled={answering}>
                    Reply
                  </Button>
                  <Button
                    variant="ghost"
                    onClick={() =>
                      void desktop()
                        .ResolveQuestion(q.id)
                        .then(() =>
                          setQuestions((value) => value.filter((item) => item.id !== q.id)),
                        )
                    }
                  >
                    Resolve
                  </Button>
                </form>
              </article>
            ))}
          {answerError && <p role="alert">{answerError}</p>}
        </section>
      )}
      <Dialog
        open={diagramDraft !== null}
        onOpenChange={(open) => {
          if (!open) setDiagramDraft(null)
        }}
        title="Edit diagram"
        description="Static SVG only. Scripts, external resources and embedded HTML are rejected."
      >
        {diagramDraft && (
          <form
            onSubmit={async (event) => {
              event.preventDefault()
              try {
                const svg = await desktop().ValidateDiagram(diagramDraft.svg)
                editor
                  ?.chain()
                  .focus()
                  .updateAttributes('diagram', { svg, caption: diagramDraft.caption })
                  .run()
                setDiagramDraft(null)
              } catch (problem) {
                setDiagramError(String(problem))
              }
            }}
          >
            <label className="field">
              <span>SVG</span>
              <textarea
                rows={10}
                value={diagramDraft.svg}
                onChange={(event) => setDiagramDraft({ ...diagramDraft, svg: event.target.value })}
              />
            </label>
            <label className="field">
              <span>Caption</span>
              <input
                value={diagramDraft.caption}
                onChange={(event) =>
                  setDiagramDraft({ ...diagramDraft, caption: event.target.value })
                }
              />
            </label>
            {diagramError && <p role="alert">{diagramError}</p>}
            <Button type="submit">Apply diagram</Button>
          </form>
        )}
      </Dialog>
      <Dialog
        open={exportFormats !== null}
        onOpenChange={(open) => {
          if (!open) setExportFormats(null)
        }}
        title="Export document"
        description="Choose a format for the current saved revision."
      >
        <div className="export-options">
          {exportFormats?.map((format) => (
            <Button
              key={format}
              variant="secondary"
              onClick={() =>
                void flush()
                  .then(() => desktop().ExportDocumentFormat(saved.id, format))
                  .then((done) => {
                    setExportFormats(null)
                    if (done) setStatus('Exported')
                  })
                  .catch((problem) => setError(String(problem)))
              }
            >
              {format === 'md' ? 'Markdown' : format === 'docx' ? 'Word document' : 'PDF document'}
            </Button>
          ))}
        </div>
      </Dialog>
      <Dialog
        open={history !== null}
        onOpenChange={(open) => {
          if (!open) {
            setHistory(null)
            setHistorical(null)
          }
        }}
        title="Version history"
        description="Restoring creates a new revision and keeps every earlier version."
      >
        <div className="revision-list">
          {history?.map((item) => (
            <button
              key={item.rev}
              onClick={() =>
                void desktop()
                  .ReadDocument(saved.id, { atRev: item.rev })
                  .then((value) => setHistorical({ rev: item.rev, markdown: value.markdown }))
              }
            >
              Revision {item.rev} · {item.author} · {new Date(item.createdAt).toLocaleString()}
            </button>
          ))}
        </div>
        {historical && (
          <>
            <pre className="revision-preview">{historical.markdown}</pre>
            <Button
              onClick={() =>
                void flush()
                  .then(() =>
                    desktop().RestoreDocument(saved.id, current.current.rev, historical.rev),
                  )
                  .then((next) => {
                    current.current = next
                    baseRev.current = next.rev
                    blocks.current = next.blocks
                    editor?.commands.setContent(toEditor(next.blocks), { emitUpdate: false })
                    setSaved(next)
                    setHistory(null)
                    setHistorical(null)
                    setStatus('Restored')
                    void desktop()
                      .ReadDocument(saved.id, {})
                      .then((value) => setMarkdown(value.markdown))
                      .catch(() => {})
                  })
                  .catch((problem) => setError(String(problem)))
              }
            >
              Restore revision {historical.rev}
            </Button>
          </>
        )}
      </Dialog>
    </aside>
  )
}
