import { useCallback, useEffect, useState } from 'react'
import { FileText, RefreshCw } from 'lucide-react'
import type { Run, WorkspaceSource } from './api'
import { desktop } from './api'
import { flushDocument } from './document-session'
import { Button, Dialog, Field, Icon } from './components/ui'

export function SourceStatus({ id, run }: { id: string; run: Run }) {
  const [source, setSource] = useState<WorkspaceSource | null>(null)
  const [comparison, setComparison] = useState<WorkspaceSource | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [path, setPath] = useState('')
  const refresh = useCallback(async () => {
    const next = await desktop().WorkspaceSource(id)
    setSource(next)
    return next
  }, [id])
  useEffect(() => {
    let active = true
    const load = () =>
      void desktop()
        .WorkspaceSource(id)
        .then((value) => {
          if (active) setSource(value)
        })
        .catch(() => {
          if (active) setError('Couldn’t check the workspace source file.')
        })
    load()
    const timer = setInterval(() => {
      if (!document.hidden) load()
    }, 2000)
    return () => {
      active = false
      clearInterval(timer)
    }
  }, [id])
  if (!source)
    return error ? (
      <p className="source-status error" role="status">
        {error}
      </p>
    ) : null
  const compare = async () => {
    setBusy(true)
    setError('')
    try {
      await flushDocument()
      setComparison(await refresh())
    } catch (problem) {
      setError(String(problem))
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className="source-location">
      <div className="source-status">
        <Icon icon={FileText} size={13} />
        <span title={source.path}>{source.path || 'Not saved to a workspace file'}</span>
        {source.status === 'changed' && (
          <Button variant="ghost" disabled={busy} onClick={() => void compare()}>
            Compare external changes
          </Button>
        )}
        {source.status === 'synced' && <small>In sync</small>}
        {source.status === 'pending' && <small>Folder save pending</small>}
        {source.status === 'missing' && <small>File is missing</small>}
        {source.status === 'unavailable' && <small>Folder file unavailable</small>}
        {(source.status === 'pending' || source.status === 'missing') && (
          <Button variant="ghost" disabled={busy} onClick={() => void compare()}>
            Compare versions
          </Button>
        )}
      </div>
      {source.status === 'unlinked' && (
        <form
          className="source-path-form"
          onSubmit={async (event) => {
            event.preventDefault()
            setBusy(true)
            setError('')
            try {
              if (
                await run(() =>
                  desktop()
                    .SetArtifactPath(source.chatId, id, path)
                    .catch((problem) => {
                      setError(String(problem))
                      throw problem
                    }),
                )
              )
                await refresh()
            } catch (problem) {
              setError(String(problem))
            } finally {
              setBusy(false)
            }
          }}
        >
          <Field
            label="Source file path"
            value={path}
            onChange={(event) => setPath(event.target.value)}
            placeholder={
              source.kind === 'document' ? 'docs/plan.md' : `docs/file.${source.kind}.json`
            }
            required
          />
          <Button type="submit" variant="secondary" disabled={busy}>
            Save to folder
          </Button>
        </form>
      )}
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      <Dialog
        open={!!comparison}
        onOpenChange={(open) => {
          if (!open) setComparison(null)
        }}
        title="Compare workspace versions"
        description="The left version is saved in Kiko. The right version is the current file. Loading the file creates a revision. Saving the Kiko version replaces the folder content shown here."
        className="source-compare-dialog"
      >
        <p className="folder-path">{comparison?.path}</p>
        <div className="source-comparison">
          <section>
            <h3>Kiko version</h3>
            <pre>{comparison?.content}</pre>
          </section>
          <section>
            <h3>Folder version</h3>
            <pre>
              {comparison?.diskHash ? comparison.diskContent : 'File is missing or unreadable.'}
            </pre>
          </section>
        </div>
        {comparison?.running && (
          <p className="field-hint">Stop the current reply before loading this version.</p>
        )}
        {error && (
          <p role="alert" className="error">
            {error}
          </p>
        )}
        <div className="dialog-actions">
          <Button variant="secondary" onClick={() => setComparison(null)}>
            Close comparison
          </Button>
          <Button
            variant="secondary"
            disabled={busy || comparison?.running || comparison?.status === 'unavailable'}
            onClick={async () => {
              if (!comparison) return
              setBusy(true)
              setError('')
              try {
                if (
                  await run(() =>
                    desktop()
                      .SaveWorkspaceSourceVersion(id, comparison.diskHash, comparison.rev)
                      .catch((problem) => {
                        setError(String(problem))
                        throw problem
                      }),
                  )
                ) {
                  setComparison(null)
                  await refresh()
                }
              } catch (problem) {
                setError(String(problem))
              } finally {
                setBusy(false)
              }
            }}
          >
            Save Kiko version
          </Button>
          <Button
            disabled={busy || !comparison?.diskHash || comparison.running}
            onClick={async () => {
              if (!comparison) return
              setBusy(true)
              setError('')
              try {
                if (
                  await run(() =>
                    desktop()
                      .ReloadWorkspaceSource(id, comparison.diskHash, comparison.rev)
                      .catch((problem) => {
                        setError(String(problem))
                        throw problem
                      }),
                  )
                ) {
                  setComparison(null)
                  await refresh()
                }
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
    </div>
  )
}
