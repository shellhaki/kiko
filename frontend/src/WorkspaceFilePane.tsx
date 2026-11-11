import { useCallback, useEffect, useRef, useState } from 'react'
import { FileText, RefreshCw, Save, X } from 'lucide-react'
import type { Run, WorkspaceEntry } from './api'
import { desktop } from './api'
import { registerDocumentSave } from './document-session'
import { Button, Dialog, Icon, IconButton, PaneHeader } from './components/ui'

export function WorkspaceFilePane({
  project,
  chat,
  path,
  close,
  run,
  onImported,
}: {
  project: string
  chat: string
  path: string
  close(): void
  run: Run
  onImported(): void
}) {
  const [initial, setInitial] = useState<WorkspaceEntry>()
  const [error, setError] = useState('')
  useEffect(() => {
    let active = true
    void desktop()
      .ReadWorkspaceEntry(project, path)
      .then((entry) => {
        if (active) setInitial(entry)
      })
      .catch((problem) => {
        if (active) setError(String(problem))
      })
    return () => {
      active = false
    }
  }, [project, path])
  if (!initial)
    return (
      <aside className="document-pane workspace-file-pane">
        <PaneHeader>
          <strong>{path}</strong>
          <IconButton icon={X} label="Close file" onClick={close} />
        </PaneHeader>
        <p className="document-notice" role="status">
          {error || 'Opening file…'}
        </p>
      </aside>
    )
  return (
    <EditableWorkspaceFile
      initial={initial}
      project={project}
      chat={chat}
      close={close}
      run={run}
      onImported={onImported}
    />
  )
}
function EditableWorkspaceFile({
  initial,
  project,
  chat,
  close,
  run,
  onImported,
}: {
  initial: WorkspaceEntry
  project: string
  chat: string
  close(): void
  run: Run
  onImported(): void
}) {
  const [text, setText] = useState(initial.draft?.content ?? initial.content)
  const [status, setStatus] = useState(initial.draft ? 'Recovered draft' : 'Saved')
  const [error, setError] = useState('')
  const [external, setExternal] = useState<WorkspaceEntry | null>(
    initial.draft && initial.draft.baseHash !== initial.hash ? initial : null,
  )
  const [comparison, setComparison] = useState<WorkspaceEntry | null>(null)
  const [busy, setBusy] = useState(false)
  const value = useRef(text)
  const baseline = useRef(initial.draft?.baseHash ?? initial.hash)
  const dirty = useRef(!!initial.draft)
  const generation = useRef(0)
  const queue = useRef<Promise<void>>(Promise.resolve())
  const pendingDraft = useRef<(() => void) | null>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const saving = useRef<Promise<void> | null>(null)
  const mounted = useRef(true)
  const flushDraft = useCallback(async () => {
    clearTimeout(timer.current)
    pendingDraft.current?.()
    await queue.current
  }, [])
  const flush = useCallback(async () => {
    if (saving.current) return saving.current
    if (!dirty.current) return
    const task = (async () => {
      try {
        while (dirty.current) {
          await flushDraft()
          const content = value.current
          const version = generation.current
          if (mounted.current) setStatus('Saving…')
          const next = await desktop().SaveWorkspaceText(
            project,
            chat,
            initial.path,
            baseline.current,
            content,
          )
          baseline.current = next.hash
          if (version === generation.current) dirty.current = false
          if (mounted.current) {
            setStatus(dirty.current ? 'Unsaved changes' : 'Saved')
            setError('')
            setExternal(null)
          }
        }
      } catch (problem) {
        if (mounted.current) {
          setStatus('Not saved')
          setError(String(problem))
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
  }, [chat, flushDraft, initial.path, project])
  useEffect(() => registerDocumentSave(flush), [flush])
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
      clearTimeout(timer.current)
      pendingDraft.current?.()
    }
  }, [])
  useEffect(() => {
    let active = true
    const poll = () =>
      void desktop()
        .ReadWorkspaceEntry(project, initial.path)
        .then((entry) => {
          if (active) setExternal(entry.hash !== baseline.current ? entry : null)
        })
        .catch(() => {
          if (active) setError('The folder file is unavailable. Your draft is kept in Kiko.')
        })
    const interval = setInterval(() => {
      if (!document.hidden) poll()
    }, 2000)
    return () => {
      active = false
      clearInterval(interval)
    }
  }, [initial.path, project])
  const change = (content: string) => {
    value.current = content
    dirty.current = true
    generation.current++
    setText(content)
    setStatus('Unsaved changes')
    clearTimeout(timer.current)
    pendingDraft.current = () => {
      pendingDraft.current = null
      const hash = baseline.current
      queue.current = queue.current
        .catch(() => {})
        .then(() => desktop().SaveWorkspaceTextDraft(project, initial.path, hash, content))
      void queue.current.catch((problem) => {
        if (mounted.current) setError(String(problem))
      })
    }
    timer.current = setTimeout(() => pendingDraft.current?.(), 200)
  }
  const compare = async () => {
    setBusy(true)
    setError('')
    try {
      await flushDraft()
      setComparison(await desktop().ReadWorkspaceEntry(project, initial.path))
    } catch (problem) {
      setError(String(problem))
    } finally {
      setBusy(false)
    }
  }
  return (
    <aside className="document-pane workspace-file-pane" aria-label="Workspace file">
      <PaneHeader>
        <Icon icon={FileText} />
        <strong title={initial.path}>{initial.path}</strong>
        <span className="save-indicator" role="status">
          {status}
        </span>
        <IconButton
          icon={Save}
          label="Save file"
          disabled={busy || status === 'Saved'}
          onClick={() => void flush().catch(() => {})}
        />
        <IconButton icon={X} label="Close file" onClick={close} />
      </PaneHeader>
      {/\.md$|\.(sheet|deck)\.json$/i.test(initial.path) && (
        <div className="workspace-file-toolbar">
          <span>{/\.md$/i.test(initial.path) ? 'Markdown source' : 'Artifact source'}</span>
          <Button
            variant="ghost"
            disabled={busy}
            onClick={async () => {
              setBusy(true)
              setError('')
              try {
                if (
                  await run(async () => {
                    const latest = await desktop().ReadWorkspaceEntry(project, initial.path)
                    return desktop()
                      .ImportWorkspaceFile(project, chat, initial.path, latest.hash)
                      .catch((problem) => {
                        setError(String(problem))
                        throw problem
                      })
                  })
                )
                  onImported()
              } catch (problem) {
                setError(String(problem))
              } finally {
                setBusy(false)
              }
            }}
          >
            Open as editable artifact
          </Button>
        </div>
      )}
      {external && (
        <div className="external-file-notice" role="status">
          <span>This file changed outside Kiko.</span>
          <Button variant="secondary" disabled={busy} onClick={() => void compare()}>
            Compare versions
          </Button>
        </div>
      )}
      {error && (
        <p className="document-notice error" role="alert">
          {error}
        </p>
      )}
      <textarea
        className="workspace-text-editor"
        aria-label="File editor"
        value={text}
        onChange={(event) => change(event.target.value)}
        spellCheck={false}
        onKeyDown={(event) => {
          if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 's') {
            event.preventDefault()
            void flush().catch(() => {})
          }
        }}
        maxLength={1024 * 1024}
      />
      <Dialog
        open={!!comparison}
        onOpenChange={(open) => {
          if (!open) setComparison(null)
        }}
        title="Compare file versions"
        description="Your draft stays on the left. Load the folder version only if you want to replace this text draft."
        className="source-compare-dialog"
      >
        <p className="folder-path">{initial.path}</p>
        <div className="source-comparison">
          <section>
            <h3>Your draft</h3>
            <pre>{text}</pre>
          </section>
          <section>
            <h3>Folder version</h3>
            <pre>{comparison?.content}</pre>
          </section>
        </div>
        {error && (
          <p role="alert" className="error">
            {error}
          </p>
        )}
        <div className="dialog-actions">
          <Button variant="secondary" onClick={() => setComparison(null)}>
            Keep draft
          </Button>
          <Button
            variant="secondary"
            disabled={busy}
            onClick={async () => {
              if (!comparison) return
              setBusy(true)
              setError('')
              try {
                await flushDraft()
                const next = await desktop().SaveWorkspaceText(
                  project,
                  chat,
                  initial.path,
                  comparison.hash,
                  value.current,
                )
                baseline.current = next.hash
                dirty.current = false
                setStatus('Saved')
                setExternal(null)
                setComparison(null)
              } catch (problem) {
                setError(String(problem))
              } finally {
                setBusy(false)
              }
            }}
          >
            Save my draft
          </Button>
          <Button
            disabled={busy}
            onClick={async () => {
              if (!comparison) return
              setBusy(true)
              setError('')
              try {
                await flushDraft()
                await desktop().DiscardWorkspaceTextDraft(project, initial.path, comparison.hash)
                baseline.current = comparison.hash
                value.current = comparison.content
                dirty.current = false
                generation.current++
                setText(comparison.content)
                setStatus('Saved')
                setExternal(null)
                setComparison(null)
              } catch (problem) {
                setError(String(problem))
              } finally {
                setBusy(false)
              }
            }}
          >
            <Icon icon={RefreshCw} size={14} />
            Load folder version
          </Button>
        </div>
      </Dialog>
    </aside>
  )
}
