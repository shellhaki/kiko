import { FolderExplorer } from './FolderExplorer'
import { WorkspaceFilePane } from './WorkspaceFilePane'
import { SourceStatus } from './SourceStatus'
import { SettingsPage } from './SettingsPage'
import { useMobile } from './use-mobile'
import { usePaneResize } from './use-pane-resize'
import { ArtifactPane } from './ArtifactPane'
import { EditorBoundary } from './EditorBoundary'
import { lazy, Suspense } from 'react'
const DocumentPane = lazy(() =>
  import('./DocumentPane').then((module) => ({ default: module.DocumentPane })),
)
import { Conversation } from './Conversation'
import { useEffect, useState } from 'react'
import {
  ChevronRight,
  Home,
  FolderOpen,
  FileText,
  Folder,
  Layers,
  MessageSquare,
  PanelLeft,
  Plus,
  Search,
  SquarePen,
} from 'lucide-react'
import type { Run, SearchResult, State } from './api'
import { desktop } from './api'
import { Button, Dialog, Field, Icon, IconButton, ItemMenu, NavRow, Select } from './components/ui'

type Props = {
  onState(value: State): void
  state: State
  busy: boolean
  error: string
  run: Run
}
type EditTarget = { kind: 'chat' | 'project'; id: string; title: string; mode: 'rename' | 'delete' }

export function Workspace({ state, busy, error, run: perform, onState }: Props) {
  const [fileError, setFileError] = useState('')
  const [searchOpen, setSearchOpen] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [settingsSection, setSettingsSection] = useState('General')
  useEffect(() => {
    const searchShortcut = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        setSearchOpen(true)
      }
    }
    window.addEventListener('keydown', searchShortcut)
    return () => window.removeEventListener('keydown', searchShortcut)
  }, [])
  const [file, setFile] = useState<{ project: string; path: string } | null>(null)
  const run: Run = (operation) =>
    perform(async () => {
      const next = await operation()
      if (
        next.documentId ||
        next.artifactId ||
        next.selection.projectId !== state.selection.projectId ||
        next.selection.chatId !== state.selection.chatId
      )
        setFile(null)
      return next
    })
  const [allProjects, setAllProjects] = useState(false)
  const [editing, setEditing] = useState<EditTarget | null>(null)
  const project = state.projects.find((p) => p.id === state.selection.projectId)
  const chat = state.chats.find((c) => c.id === state.selection.chatId)
  const document = state.documents.find((d) => d.id === state.documentId)
  const mobile = useMobile()
  const { preferences } = state
  const sidebarResize = usePaneResize({
    value: preferences.sidebarWidth,
    min: 220,
    max: 320,
    measure: (handle, startX) => {
      const initial =
        handle.parentElement?.getBoundingClientRect().width ?? preferences.sidebarWidth
      return (x) => Math.round(initial + x - startX)
    },
    commit: (next) => run(() => desktop().SaveAppearance(preferences.theme, true, next)),
  })
  const width = sidebarResize.value
  const select = async (projectId: string, chatId = '') => {
    if (await run(() => desktop().Select(projectId, chatId))) {
      setAllProjects(false)
      setFile(null)
      if (mobile) void appearance(false)
    }
  }
  const createChat = () => {
    setAllProjects(false)
    void run(() => desktop().CreateChat(project?.id ?? '')).then((ok) => {
      if (ok) setFile(null)
    })
  }
  const appearance = (open: boolean, size = width, theme = preferences.theme) =>
    run(() => desktop().SaveAppearance(theme, open, size))
  const toggleSidebar = async (open: boolean) => {
    if (await appearance(open)) {
      requestAnimationFrame(() => {
        window.document
          .querySelector<HTMLButtonElement>(
            `button[aria-label="${open ? 'Collapse' : 'Expand'} sidebar"]`,
          )
          ?.focus({ preventScroll: true })
      })
    }
  }
  const [documentOpen, setDocumentOpen] = useState(false)
  const [createKind, setCreateKind] = useState('document')
  const [resetOpen, setResetOpen] = useState(false)
  const openFile = async (path: string) => {
    if (!project) return
    let plain = false
    setFileError('')
    if (
      await run(async () => {
        const entry = await desktop()
          .ReadWorkspaceEntry(project.id, path)
          .catch((problem) => {
            setFileError(String(problem instanceof Error ? problem.message : problem))
            throw problem
          })
        if (entry.sourceId)
          return desktop().ImportWorkspaceFile(project.id, chat?.id ?? '', path, entry.hash)
        plain = true
        if (state.documentId) await desktop().CloseDocument()
        if (state.artifactId) await desktop().CloseArtifact()
        return desktop().Bootstrap()
      })
    ) {
      setFile(plain ? { project: project.id, path } : null)
      setAllProjects(false)
      if (mobile) void appearance(false)
    }
  }
  const closeFile = () =>
    void run(() => desktop().Bootstrap()).then((ok) => {
      if (ok) setFile(null)
    })
  const rowActions = (kind: 'chat' | 'project', id: string, title: string) => (
    <ItemMenu
      label={title}
      onRename={() => setEditing({ kind, id, title, mode: 'rename' })}
      onDelete={() => setEditing({ kind, id, title, mode: 'delete' })}
    />
  )
  return (
    <div className="workspace" aria-busy={busy}>
      <div
        className="workspace-body"
        data-resizing={sidebarResize.active || undefined}
        style={
          {
            '--navigation-width': `${preferences.sidebarOpen ? width + 60 : 60}px`,
          } as React.CSSProperties
        }
      >
        <nav className="rail" aria-label="Main navigation">
          <IconButton
            icon={Home}
            label="Home"
            aria-pressed={!chat && !project && !allProjects}
            onClick={() => void select('')}
          />
          <IconButton
            icon={Search}
            label={preferences.sidebarOpen ? 'Search chats' : 'Search'}
            onClick={() => setSearchOpen(true)}
          />
          <IconButton
            icon={Layers}
            label="Browse projects"
            aria-pressed={allProjects}
            onClick={() => setAllProjects(true)}
          />
          <div className="rail-rule" />
          {!preferences.sidebarOpen && (
            <IconButton
              icon={SquarePen}
              label="New chat"
              data-new-chat
              onClick={createChat}
              disabled={busy}
            />
          )}
          {chat && (
            <IconButton
              icon={FileText}
              label="New document"
              onClick={() => setDocumentOpen(true)}
            />
          )}
          <span className="spacer" />
          <IconButton
            icon={PanelLeft}
            label={preferences.sidebarOpen ? 'Collapse sidebar' : 'Expand sidebar'}
            aria-expanded={preferences.sidebarOpen}
            onClick={() => void toggleSidebar(!preferences.sidebarOpen)}
          />
          <button
            className="rail-profile"
            aria-label="Settings"
            title="Settings"
            onClick={() => setSettingsOpen(true)}
          >
            {preferences.nickname.slice(0, 1).toUpperCase() || 'K'}
          </button>
        </nav>
        <div
          className="workspace-navigation"
          inert={!preferences.sidebarOpen}
          aria-hidden={!preferences.sidebarOpen}
          data-resizing={sidebarResize.active || undefined}
          style={{ width: preferences.sidebarOpen ? width : 0 }}
        >
          <>
            <aside className="sidebar" style={{ width }} aria-label="Chats and projects">
              <div className="sidebar-heading">
                <span className="sidebar-wordmark">Kiko</span>
                <IconButton icon={Search} label="Search" onClick={() => setSearchOpen(true)} />
              </div>
              <Button
                className="new-chat"
                data-new-chat
                variant="ghost"
                onClick={createChat}
                disabled={busy}
              >
                <Icon icon={SquarePen} /> New chat
              </Button>
              <div className="sidebar-scroll">
                <div className="sidebar-group">
                  <div className="sidebar-section-heading">
                    <h2>Projects</h2>
                    <IconButton
                      icon={FolderOpen}
                      label="Choose project folder"
                      disabled={busy || !!window.kikoWeb}
                      onClick={() => void run(() => desktop().ChooseFolder(''))}
                    />
                  </div>
                  {state.projects.map((p) => (
                    <div key={p.id} className="project-branch">
                      <div className="nav-item">
                        <NavRow
                          active={project?.id === p.id && !allProjects}
                          icon={project?.id === p.id ? FolderOpen : Folder}
                          onClick={() => void select(p.id)}
                        >
                          {p.name}
                        </NavRow>
                        {rowActions('project', p.id, p.name)}
                      </div>
                      {project?.id === p.id && (
                        <div className="project-chats" aria-label={`${p.name} chats`}>
                          {state.chats
                            .filter((c) => c.projectId === p.id)
                            .map((c) => (
                              <div className="nav-item" key={c.id}>
                                <NavRow
                                  active={chat?.id === c.id && !allProjects}
                                  onClick={() => void select('', c.id)}
                                >
                                  {c.title}
                                </NavRow>
                                {rowActions('chat', c.id, c.title)}
                              </div>
                            ))}
                          {!state.chats.some((c) => c.projectId === p.id) && (
                            <p className="empty-list">Start a conversation in this project.</p>
                          )}
                        </div>
                      )}
                    </div>
                  ))}
                  <button className="all-projects-link" onClick={() => setAllProjects(true)}>
                    All projects <Icon icon={ChevronRight} size={13} />
                  </button>
                </div>
                {project?.root && (
                  <FolderExplorer
                    key={project.id}
                    project={project.id}
                    onOpen={(path) => void openFile(path)}
                  />
                )}
                <div className="sidebar-group">
                  <h2>Recents</h2>
                  {state.chats
                    .filter((c) => !project || c.projectId !== project.id)
                    .map((c) => (
                      <div className="nav-item" key={c.id}>
                        <NavRow
                          active={chat?.id === c.id && !allProjects}
                          onClick={() => void select('', c.id)}
                        >
                          {c.title}
                        </NavRow>
                        {rowActions('chat', c.id, c.title)}
                      </div>
                    ))}
                  {!state.chats.some((c) => !project || c.projectId !== project.id) && (
                    <p className="empty-list">Your recent conversations will appear here.</p>
                  )}
                </div>
              </div>
            </aside>
            <div
              className="sidebar-divider"
              role="separator"
              tabIndex={0}
              aria-label="Sidebar width"
              aria-orientation="vertical"
              aria-valuemin={220}
              aria-valuemax={320}
              aria-valuenow={width}
              onKeyDown={(event) => {
                if (event.key === 'Escape') {
                  sidebarResize.cancel()
                  return
                }
                const next =
                  event.key === 'ArrowLeft'
                    ? width - 8
                    : event.key === 'ArrowRight'
                      ? width + 8
                      : event.key === 'Home'
                        ? 220
                        : event.key === 'End'
                          ? 320
                          : null
                if (next !== null) {
                  event.preventDefault()
                  void appearance(true, Math.max(220, Math.min(320, next)))
                }
              }}
              {...sidebarResize.handlers}
            />
          </>
        </div>
        {preferences.sidebarOpen && mobile && (
          <button
            className="navigation-backdrop"
            aria-label="Close navigation"
            onClick={() => void toggleSidebar(false)}
          />
        )}
        <main className="main-content" inert={mobile && preferences.sidebarOpen}>
          {(chat || allProjects) && (
            <header className="workspace-toolbar">
              <div className="workspace-location">
                <Icon icon={project ? Folder : MessageSquare} size={16} />
                {project && (
                  <>
                    <button className="breadcrumb" onClick={() => void select(project.id)}>
                      {project.name}
                    </button>
                    <Icon icon={ChevronRight} size={13} />
                  </>
                )}
                <strong>{allProjects ? 'Projects' : (chat?.title ?? 'Home')}</strong>
              </div>
              <span className="spacer" />
            </header>
          )}
          <div aria-live="polite" className="work-status">
            {error && (
              <p role="alert" className="error">
                {error}
              </p>
            )}
          </div>
          {settingsOpen ? (
            <SettingsPage
              state={state}
              busy={busy}
              error={error}
              run={run}
              section={settingsSection}
              onSectionChange={setSettingsSection}
              onBack={() => setSettingsOpen(false)}
              onReset={() => setResetOpen(true)}
            />
          ) : allProjects ? (
            <section className="projects-view">
              <div className="section-title">
                <div>
                  <h1>Projects</h1>
                  <p className="muted">Folders and their conversations.</p>
                </div>
                <Button
                  disabled={!!window.kikoWeb || busy}
                  onClick={() => void run(() => desktop().ChooseFolder(''))}
                >
                  <Icon icon={Plus} />
                  Choose folder
                </Button>
              </div>
              {state.projects.length === 0 ? (
                <p className="empty-list">Choose a folder from the composer to start a project.</p>
              ) : (
                <div className="project-grid">
                  {state.projects.map((p) => (
                    <article className="project-card" key={p.id}>
                      <button className="project-card-main" onClick={() => void select(p.id)}>
                        <Icon icon={Folder} size={24} />
                        <h2>{p.name}</h2>
                        <p>{p.root || p.description}</p>
                        <small>
                          {state.chats.filter((c) => c.projectId === p.id).length} chats
                        </small>
                      </button>
                      {rowActions('project', p.id, p.name)}
                    </article>
                  ))}
                </div>
              )}
            </section>
          ) : (
            <Conversation
              key={chat ? `chat:${chat.id}` : `project:${project?.id ?? ''}`}
              state={state}
              heading={
                project
                  ? `What should we create in ${project.name}?`
                  : 'What would you like to create?'
              }
              subtitle=""
              onOpenFile={(path) => void openFile(path)}
              onPreferences={() => {
                setSettingsSection('General')
                setSettingsOpen(true)
              }}
              onState={onState}
              run={run}
            />
          )}
        </main>
        {!!state.artifactId && !document && !allProjects && !file && (
          <EditorBoundary
            key={state.artifactId}
            close={async () => {
              await run(() => desktop().CloseArtifact())
            }}
          >
            <ArtifactPane
              key={state.artifactId}
              id={state.artifactId}
              sourceControls={
                project?.root ? (
                  <SourceStatus key={state.artifactId} id={state.artifactId} run={run} />
                ) : undefined
              }
              close={async () => {
                await run(() => desktop().CloseArtifact())
              }}
            />
          </EditorBoundary>
        )}
        {document && !allProjects && !file && (
          <EditorBoundary
            key={document.id}
            close={async () => {
              await run(() => desktop().CloseDocument())
            }}
          >
            <Suspense
              fallback={
                <aside className="document-pane">
                  <p className="document-notice">Opening editor…</p>
                </aside>
              }
            >
              <DocumentPane
                key={document.id}
                id={document.id}
                sourceControls={
                  project?.root ? (
                    <SourceStatus key={document.id} id={document.id} run={run} />
                  ) : undefined
                }
                onState={onState}
                close={async () => {
                  await run(() => desktop().CloseDocument())
                }}
              />
            </Suspense>
          </EditorBoundary>
        )}
        {file && !allProjects && (
          <EditorBoundary
            key={`${file.project}:${file.path}`}
            close={async () => {
              closeFile()
            }}
          >
            <WorkspaceFilePane
              key={`${file.project}:${file.path}`}
              project={file.project}
              chat={chat?.id ?? ''}
              path={file.path}
              close={closeFile}
              run={run}
              onImported={() => setFile(null)}
            />
          </EditorBoundary>
        )}
      </div>
      <Dialog
        open={!!fileError}
        onOpenChange={(open) => {
          if (!open) setFileError('')
        }}
        title="Couldn’t open this file"
        description={fileError}
      >
        <div className="dialog-actions">
          <Button onClick={() => setFileError('')}>Close</Button>
        </div>
      </Dialog>
      <Dialog
        open={editing !== null}
        onOpenChange={(open) => {
          if (!open && !busy) setEditing(null)
        }}
        title={
          editing?.mode === 'delete'
            ? `Delete ${editing.kind}?`
            : `Rename ${editing?.kind ?? 'item'}`
        }
        description={
          editing?.mode === 'delete'
            ? `Deleting “${editing.title}” also removes its ${editing.kind === 'project' ? 'chats, ' : ''}documents and history from Kiko. Files in your selected folder are kept. This cannot be undone.`
            : 'Choose a clear name you can find later.'
        }
      >
        {editing && (
          <form
            key={editing.id}
            onSubmit={async (event) => {
              event.preventDefault()
              const data = new FormData(event.currentTarget)
              if (
                await run(() =>
                  editing.mode === 'delete'
                    ? desktop().Delete(editing.kind, editing.id)
                    : desktop().Rename(editing.kind, editing.id, String(data.get('name'))),
                )
              )
                setEditing(null)
            }}
          >
            {editing.mode === 'rename' && (
              <Field
                label="Name"
                name="name"
                defaultValue={editing.title}
                required
                maxLength={120}
                disabled={busy}
              />
            )}
            {error && (
              <p role="alert" className="error">
                {error}
              </p>
            )}
            <div className="dialog-actions">
              <Button variant="secondary" disabled={busy} onClick={() => setEditing(null)}>
                Cancel
              </Button>
              <Button
                type="submit"
                variant={editing.mode === 'delete' ? 'danger' : 'primary'}
                disabled={busy}
              >
                {editing.mode === 'delete' ? 'Delete' : 'Save name'}
              </Button>
            </div>
          </form>
        )}
      </Dialog>
      <Dialog
        open={resetOpen}
        onOpenChange={setResetOpen}
        title="Reset Kiko?"
        description="This permanently deletes all local projects, chats, documents, drafts, revisions, and preferences, and removes your saved Muse key. Files in selected folders are kept. You will return to onboarding."
      >
        <form
          onSubmit={async (event) => {
            event.preventDefault()
            if (await run(() => desktop().ResetData())) {
              setResetOpen(false)
              setSettingsOpen(false)
            }
          }}
        >
          <Field
            label="Type RESET to confirm"
            name="confirmation"
            required
            pattern="RESET"
            autoComplete="off"
          />
          <div className="dialog-actions">
            <Button variant="secondary" onClick={() => setResetOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" variant="danger" disabled={busy}>
              Delete all local data
            </Button>
          </div>
        </form>
      </Dialog>
      <Dialog
        open={documentOpen}
        onOpenChange={setDocumentOpen}
        title="New artifact"
        description="Start writing alongside your conversation."
      >
        <form
          onSubmit={async (event) => {
            event.preventDefault()
            const data = new FormData(event.currentTarget)
            if (
              chat &&
              (await run(() =>
                project?.root && String(data.get('path') ?? '').trim()
                  ? desktop().CreateArtifactAt(
                      chat.id,
                      createKind,
                      String(data.get('title')),
                      String(data.get('path')).trim(),
                    )
                  : createKind === 'deck'
                    ? desktop().CreateDeck(chat.id, String(data.get('title')))
                    : createKind === 'sheet'
                      ? desktop().CreateSheet(chat.id, String(data.get('title')))
                      : desktop().CreateDocument(
                          chat.id,
                          String(data.get('title')),
                          '# ' + String(data.get('title')),
                        ),
              ))
            ) {
              setDocumentOpen(false)
            }
          }}
        >
          <label className="field">
            <span>Type</span>
            <Select
              label="Type"
              value={createKind}
              onChange={setCreateKind}
              options={[
                { value: 'document', label: 'Document' },
                { value: 'sheet', label: 'Spreadsheet' },
                { value: 'deck', label: 'Slide deck' },
              ]}
            />
          </label>
          <Field label="Document title" name="title" required maxLength={120} />
          {project?.root && (
            <Field
              key={createKind}
              label="Save path (optional)"
              name="path"
              placeholder={
                createKind === 'document' ? 'docs/plan.md' : `docs/file.${createKind}.json`
              }
              maxLength={500}
            />
          )}
          {project?.root && (
            <p className="field-hint">
              Relative to {project.name}. Leave blank for an automatic filename.
            </p>
          )}
          {error && (
            <p role="alert" className="error">
              {error}
            </p>
          )}
          <div className="dialog-actions">
            <Button variant="secondary" onClick={() => setDocumentOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={busy}>
              Create document
            </Button>
          </div>
        </form>
      </Dialog>
      <SearchDialog open={searchOpen} setOpen={setSearchOpen} run={run} />
    </div>
  )
}

function SearchDialog({
  open,
  setOpen,
  run,
}: {
  open: boolean
  setOpen(open: boolean): void
  run: Run
}) {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<SearchResult[]>([])
  const [searched, setSearched] = useState(false)
  const [searching, setSearching] = useState(false)
  const [error, setError] = useState('')
  return (
    <Dialog
      open={open}
      onOpenChange={setOpen}
      title="Search"
      description="Find your saved chats and documents."
    >
      <form
        className="search-form"
        onSubmit={async (event) => {
          event.preventDefault()
          setSearching(true)
          setError('')
          try {
            setResults(await desktop().Search(query))
            setSearched(true)
          } catch {
            setError('Couldn’t search local data. Try again.')
          } finally {
            setSearching(false)
          }
        }}
      >
        <Field
          label="Search query"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Search titles and document text"
        />
        <Button type="submit" disabled={!query.trim() || searching}>
          {searching ? 'Searching…' : 'Search'}
        </Button>
      </form>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      <div className="search-results" aria-live="polite">
        {searched && !results.length && (
          <p className="muted">No matches found. Try another phrase.</p>
        )}
        {results.map((result) => (
          <button
            key={`${result.kind}-${result.id}`}
            className="search-result"
            onClick={async () => {
              if (
                await run(() =>
                  result.kind === 'document'
                    ? desktop().OpenDocument(result.id)
                    : desktop().Select(result.projectId, result.chatId),
                )
              )
                setOpen(false)
            }}
          >
            <Icon icon={result.kind === 'document' ? FileText : MessageSquare} />
            <span>
              {result.title}
              <small>{result.kind === 'document' ? 'Document' : 'Chat'}</small>
            </span>
            <Icon icon={ChevronRight} size={14} />
          </button>
        ))}
      </div>
    </Dialog>
  )
}
