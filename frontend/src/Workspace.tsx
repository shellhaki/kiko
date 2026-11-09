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
  SlidersHorizontal,
  Cpu,
  Globe,
  Database,
  FileText,
  Folder,
  Layers,
  MessageSquare,
  PanelLeft,
  Plus,
  Search,
  Settings2,
  SquarePen,
} from 'lucide-react'
import type { Run, SearchResult, State } from './api'
import { desktop } from './api'
import { Brand, Button, Dialog, Field, Icon, IconButton, ItemMenu, NavRow } from './components/ui'

type Props = {
  onState(value: State): void
  state: State
  busy: boolean
  error: string
  run: Run
}
type EditTarget = { kind: 'chat' | 'project'; id: string; title: string; mode: 'rename' | 'delete' }

export function Workspace({ state, busy, error, run, onState }: Props) {
  const [projectOpen, setProjectOpen] = useState(false)
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
      if (mobile) void appearance(false)
    }
  }
  const createChat = () => {
    setAllProjects(false)
    void run(() => desktop().CreateChat(project?.id ?? ''))
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
  const [replacementKey, setReplacementKey] = useState('')
  const [researchKey, setResearchKey] = useState('')
  const [providerID, setProviderID] = useState('openai')
  const [providerModel, setProviderModel] = useState('')
  const [providerKey, setProviderKey] = useState('')
  const [providerEndpoint, setProviderEndpoint] = useState('http://127.0.0.1:11434/v1')
  const [showKey, setShowKey] = useState(false)
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
          data-resizing={sidebarResize.active || undefined}
          style={{ width: preferences.sidebarOpen ? width : 0 }}
        >
          {preferences.sidebarOpen && (
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
                        icon={Plus}
                        label="New project"
                        onClick={() => setProjectOpen(true)}
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
                    {!state.projects.length && (
                      <button className="sidebar-empty-action" onClick={() => setProjectOpen(true)}>
                        Create a space for your work <Icon icon={Plus} size={14} />
                      </button>
                    )}
                    <button className="all-projects-link" onClick={() => setAllProjects(true)}>
                      All projects <Icon icon={ChevronRight} size={13} />
                    </button>
                  </div>
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
          )}
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
          {chat && (state.artifacts ?? []).some((item) => item.chatId === chat.id) && (
            <div className="artifact-list">
              {state.artifacts
                ?.filter((item) => item.chatId === chat.id)
                .map((item) => (
                  <button
                    className="artifact-chip"
                    key={item.id}
                    onClick={() => void run(() => desktop().OpenArtifact(item.id))}
                  >
                    <Icon icon={FileText} size={16} />
                    {item.title}
                    <small>{item.kind === 'sheet' ? 'Spreadsheet' : 'Slides'}</small>
                  </button>
                ))}
            </div>
          )}
          {chat && state.documents.some((d) => d.chatId === chat.id) && (
            <div className="artifact-list">
              {state.documents
                .filter((d) => d.chatId === chat.id)
                .map((d) => (
                  <button
                    className="artifact-chip"
                    key={d.id}
                    onClick={() => void run(() => desktop().OpenDocument(d.id))}
                  >
                    <Icon icon={FileText} size={16} />
                    {d.title}
                  </button>
                ))}
            </div>
          )}
          <div aria-live="polite" className="work-status">
            {busy && <span>Saving…</span>}
            {error && (
              <p role="alert" className="error">
                {error}
              </p>
            )}
          </div>
          {allProjects ? (
            <section className="projects-view">
              <div className="section-title">
                <div>
                  <h1>Projects</h1>
                  <p className="muted">A space for each idea.</p>
                </div>
                <Button onClick={() => setProjectOpen(true)}>
                  <Icon icon={Plus} />
                  New project
                </Button>
              </div>
              {state.projects.length === 0 ? (
                <p className="empty-list">
                  No projects yet. Create one to organize your chats and documents.
                </p>
              ) : (
                <div className="project-grid">
                  {state.projects.map((p) => (
                    <article className="project-card" key={p.id}>
                      <button className="project-card-main" onClick={() => void select(p.id)}>
                        <Icon icon={Folder} size={24} />
                        <h2>{p.name}</h2>
                        <p>{p.description || 'Your next idea starts here.'}</p>
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
              onPreferences={() => {
                setSettingsSection('General')
                setSettingsOpen(true)
              }}
              onState={onState}
              onReconnect={() => {
                setSettingsSection('AI & models')
                setSettingsOpen(true)
              }}
              run={run}
            />
          )}
        </main>
        {!!state.artifactId && !document && !allProjects && (
          <EditorBoundary
            key={state.artifactId}
            close={async () => {
              await run(() => desktop().CloseArtifact())
            }}
          >
            <ArtifactPane
              key={state.artifactId}
              id={state.artifactId}
              close={async () => {
                await run(() => desktop().CloseArtifact())
              }}
            />
          </EditorBoundary>
        )}
        {document && !allProjects && (
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
                onState={onState}
                close={async () => {
                  await run(() => desktop().CloseDocument())
                }}
              />
            </Suspense>
          </EditorBoundary>
        )}
      </div>
      <Dialog
        open={projectOpen}
        onOpenChange={(open) => {
          if (!busy) setProjectOpen(open)
        }}
        title="New project"
        description="Keep related chats and documents together."
      >
        <form
          onSubmit={async (event) => {
            event.preventDefault()
            const data = new FormData(event.currentTarget)
            if (
              await run(() =>
                desktop().CreateProject(String(data.get('name')), String(data.get('description'))),
              )
            ) {
              setProjectOpen(false)
              setAllProjects(false)
            }
          }}
        >
          <Field
            label="Project name"
            name="name"
            required
            maxLength={120}
            placeholder="e.g. Envi, or your next big idea"
            disabled={busy}
          />
          <label className="field">
            <span>Description (optional)</span>
            <textarea
              name="description"
              maxLength={2000}
              placeholder="A short description of this project…"
              disabled={busy}
            />
          </label>
          {error && (
            <p className="error" role="alert">
              {error}
            </p>
          )}
          <div className="dialog-actions">
            <Button variant="secondary" disabled={busy} onClick={() => setProjectOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={busy}>
              {busy ? 'Creating…' : 'Create project'}
            </Button>
          </div>
        </form>
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
            ? `Deleting “${editing.title}” also removes its ${editing.kind === 'project' ? 'chats, ' : ''}documents and history. This cannot be undone.`
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
        description="This permanently deletes all local projects, chats, documents, drafts, revisions, and preferences, and removes your saved Muse key. You will return to onboarding."
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
                createKind === 'deck'
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
            <select value={createKind} onChange={(event) => setCreateKind(event.target.value)}>
              <option value="document">Document</option>
              <option value="sheet">Spreadsheet</option>
              <option value="deck">Slide deck</option>
            </select>
          </label>
          <Field label="Document title" name="title" required maxLength={120} />
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
      <Dialog
        open={settingsOpen}
        onOpenChange={setSettingsOpen}
        title="Settings"
        className="settings-dialog"
        description="Make Kiko feel like your workspace."
      >
        <div className="settings-layout">
          <nav className="settings-navigation" aria-label="Settings sections">
            {[
              { name: 'General', icon: SlidersHorizontal },
              { name: 'AI & models', icon: Cpu },
              { name: 'Research', icon: Globe },
              { name: 'Data', icon: Database },
            ].map(({ name, icon }) => (
              <button
                key={name}
                aria-current={settingsSection === name ? 'page' : undefined}
                onClick={() => setSettingsSection(name)}
              >
                <Icon icon={icon} size={17} />
                {name}
              </button>
            ))}
            <div className="settings-about">
              <Brand compact />
              <span>
                Kiko<small>Your ideas, made useful.</small>
              </span>
            </div>
          </nav>
          <div className="settings-content">
            <section hidden={settingsSection !== 'General'} aria-label="General settings">
              <h2>General</h2>
              <p className="settings-intro">A workspace that feels like yours.</p>
              {window.kikoWeb && (
                <Button
                  variant="ghost"
                  onClick={() =>
                    void fetch('/api/logout', { method: 'POST' }).then(() =>
                      window.location.reload(),
                    )
                  }
                >
                  Sign out of web workspace
                </Button>
              )}
              <form
                onSubmit={async (event) => {
                  event.preventDefault()
                  const data = new FormData(event.currentTarget)
                  await run(() => desktop().SaveNickname(String(data.get('nickname'))))
                }}
              >
                <Field
                  label="Nickname"
                  name="nickname"
                  defaultValue={preferences.nickname}
                  required
                  disabled={busy}
                />
                <Button type="submit" variant="secondary" disabled={busy}>
                  Save nickname
                </Button>
              </form>
              <label className="suggestion-mode">
                <input
                  type="checkbox"
                  checked={!!preferences.suggestionMode}
                  disabled={busy}
                  onChange={(event) => {
                    const selected = event.target.checked
                    void run(() => desktop().SetSuggestionMode(selected))
                  }}
                />
                <span>Review AI document edits as suggestions</span>
              </label>
              <fieldset className="theme-picker">
                <legend>Appearance</legend>
                {(['dark', 'light', 'system'] as const).map((theme) => (
                  <label key={theme} className={`theme-option ${theme}`}>
                    <span className="theme-preview" aria-hidden="true">
                      <i />
                      <i />
                      <i />
                    </span>
                    <span>
                      <input
                        type="radio"
                        name="theme"
                        value={theme}
                        checked={preferences.theme === theme}
                        disabled={busy}
                        onChange={() => void appearance(preferences.sidebarOpen, width, theme)}
                      />
                      {theme[0].toUpperCase() + theme.slice(1)}
                    </span>
                  </label>
                ))}
              </fieldset>
            </section>
            <section hidden={settingsSection !== 'AI & models'} aria-label="AI and model settings">
              <h2>AI & models</h2>
              <p className="settings-intro">Choose the intelligence behind your work.</p>
              <div className="connection-summary">
                <Icon icon={Settings2} />
                <div>
                  <strong>Meta Muse</strong>
                  <p className="muted">
                    {state.provider?.id && state.provider.id !== 'muse'
                      ? 'Muse saved separately'
                      : state.connected
                        ? 'Connected'
                        : 'Disconnected'}{' '}
                    · Muse Spark 1.3 Contributor
                  </p>
                </div>
              </div>
              <p className="field-hint">
                {window.kikoWeb
                  ? 'Keys are encrypted on this Kiko server. Saved keys are never sent back to your browser.'
                  : 'Keys are managed in your operating system’s secure keychain.'}
              </p>
              <form
                className="key-settings"
                onSubmit={async (event) => {
                  event.preventDefault()
                  if (await run(() => desktop().ConnectMuse(replacementKey))) setReplacementKey('')
                }}
              >
                <Field
                  label={state.connected ? 'Replace Muse API key' : 'Connect Muse API key'}
                  type={showKey ? 'text' : 'password'}
                  value={replacementKey}
                  onChange={(event) => setReplacementKey(event.target.value)}
                  autoComplete="off"
                  disabled={busy}
                />
                <Button variant="ghost" onClick={() => setShowKey(!showKey)}>
                  {showKey ? 'Hide key' : 'Show key'}
                </Button>
                <p className="field-hint">
                  Verifying sends a short request to Meta and may use your API credits. Your
                  existing key stays until the replacement is verified.
                </p>
                <Button type="submit" variant="secondary" disabled={busy || !replacementKey.trim()}>
                  Verify and save key
                </Button>
                {state.connected && (
                  <Button
                    variant="ghost"
                    disabled={busy}
                    onClick={() => void run(() => desktop().RemoveMuseKey())}
                  >
                    Remove saved key
                  </Button>
                )}
              </form>
              <label className="field">
                <span>Model</span>
                <select
                  value="muse-spark-1.3-contributor"
                  onChange={(event) => {
                    const selected = event.target.value
                    void run(() => desktop().SetModel(selected))
                  }}
                >
                  <option value="muse-spark-1.3-contributor">Muse Spark 1.3 Contributor</option>
                </select>
              </label>
              <p className="field-hint">
                Contributor mode offers discounted usage and allows Meta to use submitted content
                and responses for model training.
              </p>
              <section className="key-settings">
                <h2>Model providers</h2>
                <label className="field">
                  <span>Provider for this chat and new chats</span>
                  <select
                    value={state.provider?.id ?? 'muse'}
                    disabled={busy}
                    onChange={(event) => {
                      const selected = event.target.value
                      void run(() => desktop().SelectProvider(selected))
                    }}
                  >
                    {(state.providers ?? [{ id: 'muse', model: 'muse-spark-1.3-contributor' }]).map(
                      (item) => (
                        <option key={item.id} value={item.id}>
                          {item.id} · {item.model}
                        </option>
                      ),
                    )}
                  </select>
                </label>
                <p className="field-hint">
                  Providers never switch automatically after an error. Use a chat/tool-capable model
                  ID available to your account.
                </p>
                {(state.providers ?? [])
                  .filter((item) => item.id === 'openai' || item.id === 'anthropic')
                  .map((item) => (
                    <label className="field-hint" key={item.id}>
                      <input
                        type="checkbox"
                        disabled={busy}
                        checked={preferences.imageModels?.[item.id] === item.model}
                        onChange={(event) => {
                          const enabled = event.target.checked
                          void run(() => desktop().SetDirectImages(item.id, enabled))
                        }}
                      />
                      Send page and diagram images directly to {item.id} · {item.model}
                    </label>
                  ))}
                <p className="field-hint">
                  Enable direct images only for an image-capable model. Rendered source images then
                  go to that chat provider on request, using its credits. Changing the model
                  requires enabling this again. Disabling takes effect before the next model
                  request. Muse uses the separately selected vision helper.
                </p>
                <form
                  onSubmit={async (event) => {
                    event.preventDefault()
                    if (
                      await run(() =>
                        desktop().ConfigureProvider(
                          providerID,
                          providerModel,
                          providerKey,
                          providerEndpoint,
                        ),
                      )
                    )
                      setProviderKey('')
                  }}
                >
                  <label className="field">
                    <span>Add or update provider</span>
                    <select
                      value={providerID}
                      onChange={(event) => setProviderID(event.target.value)}
                    >
                      {['openai', 'openrouter', 'anthropic', 'groq', 'gemini', 'ollama'].map(
                        (id) => (
                          <option value={id} key={id}>
                            {id === 'ollama' ? 'Ollama (local)' : id}
                          </option>
                        ),
                      )}
                    </select>
                  </label>
                  <Field
                    label="Model ID"
                    value={providerModel}
                    onChange={(event) => setProviderModel(event.target.value)}
                    required
                    placeholder="Exact model ID for your account"
                  />
                  {providerID === 'ollama' ? (
                    <Field
                      label="Local Ollama endpoint"
                      value={providerEndpoint}
                      onChange={(event) => setProviderEndpoint(event.target.value)}
                    />
                  ) : (
                    <Field
                      label="Provider API key"
                      type="password"
                      value={providerKey}
                      onChange={(event) => setProviderKey(event.target.value)}
                      autoComplete="off"
                      required
                    />
                  )}
                  <p className="field-hint">
                    Connect sends a short request to this provider and may use credits. Keys are
                    saved separately in secure storage.
                  </p>
                  <Button type="submit" variant="secondary" disabled={busy}>
                    Verify and select provider
                  </Button>
                  {state.providers?.some((item) => item.id === providerID) && (
                    <Button
                      variant="ghost"
                      disabled={busy}
                      onClick={() => void run(() => desktop().RemoveProviderKey(providerID))}
                    >
                      Remove this provider key
                    </Button>
                  )}
                </form>
                <label className="field">
                  <span>Vision helper</span>
                  <select
                    value={preferences.visionProviderId ?? 'muse'}
                    onChange={(event) => {
                      const selected = event.target.value
                      void run(() => desktop().SetVisionProvider(selected))
                    }}
                  >
                    {(state.providers ?? [{ id: 'muse', model: 'muse-spark-1.3-contributor' }])
                      .filter((item) => item.id === 'muse' || item.id === 'openai')
                      .map((item) => (
                        <option key={item.id} value={item.id}>
                          {item.id} · {item.model}
                        </option>
                      ))}
                  </select>
                </label>
                <p className="field-hint">
                  Choose a vision-capable Responses model. Visual requests use this provider’s
                  credits; there is no fallback.
                </p>
              </section>
            </section>
            <section
              hidden={settingsSection !== 'Research'}
              aria-label="Research settings"
              className="key-settings"
            >
              <h2>Web research</h2>
              <p className="field-hint">
                Optional Tavily Basic search and page extraction. Calls use your Tavily credits;
                source excerpts are sent to your selected model. At most five research calls per
                turn.
              </p>
              <p>
                {preferences.tavilyEnabled ? 'Tavily key saved' : 'Not configured'} ·{' '}
                {
                  state.messages
                    .flatMap((message) => message.steps ?? [])
                    .filter((step) => step.tool === 'web_search' || step.tool === 'web_fetch')
                    .length
                }{' '}
                calls in this chat
              </p>
              <form
                onSubmit={async (event) => {
                  event.preventDefault()
                  if (await run(() => desktop().ConfigureResearch(researchKey))) setResearchKey('')
                }}
              >
                <Field
                  label="Tavily API key"
                  type="password"
                  value={researchKey}
                  onChange={(event) => setResearchKey(event.target.value)}
                  autoComplete="off"
                />
                <Button type="submit" variant="secondary" disabled={busy || !researchKey.trim()}>
                  Save research key
                </Button>
                {preferences.tavilyEnabled && (
                  <Button
                    variant="ghost"
                    disabled={busy}
                    onClick={() => void run(() => desktop().RemoveResearch())}
                  >
                    Remove research key
                  </Button>
                )}
              </form>
            </section>
            <section
              hidden={settingsSection !== 'Data'}
              aria-label="Data settings"
              className="settings-data"
            >
              <h2>Data</h2>
              <p className="field-hint">
                Remove all local projects, chats, documents, history, preferences, and the saved
                Muse key.
              </p>
              <Button variant="danger" onClick={() => setResetOpen(true)}>
                Reset Kiko
              </Button>
            </section>
            {error && (
              <p role="alert" className="error">
                {error}
              </p>
            )}
          </div>
        </div>
      </Dialog>
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
