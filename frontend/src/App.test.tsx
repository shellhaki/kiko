import { WebAuthenticationError } from './web-api'
import { afterEach, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import App from './App'
import type { Backend, MuseModel, State } from './api'

export const models: MuseModel[] = [
  {
    id: 'muse-spark-1.3-contributor',
    name: 'Muse Spark 1.3',
    mode: 'contributor',
    description: 'Latest Spark at a lower price.',
  },
  { id: 'muse-ember-1.0', name: 'Muse Ember', mode: 'standard', description: 'Fast and low cost.' },
]

export function fixture(stage: 'nickname' | 'muse' | 'done' = 'nickname'): State {
  return {
    messages: [],
    draft: '',
    preferences: {
      nickname: stage === 'nickname' ? '' : 'Haki',
      stage,
      theme: 'dark',
      sidebarOpen: true,
      sidebarWidth: 260,
      resetInProgress: false,
    },
    projects: [],
    chats: [],
    documents: [],
    selection: { projectId: '', chatId: '' },
    documentId: '',
    connected: stage === 'done',
    model: models[0].id,
    models,
  }
}
function backend(state = fixture()): Backend {
  const api = {
    ListWorkspaceDirectory: vi.fn().mockResolvedValue([]),
    ReadWorkspaceEntry: vi.fn(),
    SaveWorkspaceTextDraft: vi.fn().mockResolvedValue(undefined),
    SaveWorkspaceText: vi.fn(),
    DiscardWorkspaceTextDraft: vi.fn().mockResolvedValue(undefined),
    ImportWorkspaceFile: vi.fn(),
    SaveWorkspaceSourceVersion: vi.fn(),
    WorkspaceSource: vi.fn().mockResolvedValue(null),
    ReloadWorkspaceSource: vi.fn(),
    CreateArtifactAt: vi.fn(),
    SetArtifactPath: vi.fn(),
    WorkspaceIssues: vi.fn().mockResolvedValue([]),
    RetryWorkspaceSync: vi.fn().mockResolvedValue(undefined),
    ChooseFolder: vi.fn().mockResolvedValue(state),
    WorkspaceFiles: vi.fn().mockResolvedValue([]),
    FilePermissions: vi.fn().mockResolvedValue([]),
    ResolveFilePermission: vi.fn().mockResolvedValue(undefined),
    ReadWorkspaceFile: vi.fn(),
    CreateDeck: vi.fn(),
    ConfigureProvider: vi.fn(),
    SelectProvider: vi.fn(),
    SetVisionProvider: vi.fn(),
    SetDirectImages: vi.fn(),
    RenderAttachmentPage: vi.fn(),
    RemoveProviderKey: vi.fn(),
    ReadArtifact: vi.fn(),
    SaveArtifact: vi.fn(),
    SaveArtifactDraft: vi.fn(),
    DiscardArtifactDraft: vi.fn(),
    OpenArtifact: vi.fn(),
    CloseArtifact: vi.fn(),
    CreateSheet: vi.fn(),
    ExportArtifact: vi.fn(),
    ExportDeckPptx: vi.fn(),
    AttachmentBytes: vi.fn(),
    ValidateDiagram: vi.fn(),
    AttachFile: vi.fn(),
    Attachments: vi.fn().mockResolvedValue([]),
    ReadAttachment: vi.fn(),
    RemoveAttachment: vi.fn(),
    AttachmentPreview: vi.fn(),
    ConfigureResearch: vi.fn(),
    RemoveResearch: vi.fn(),
    ExportFormats: vi.fn(),
    ExportDocumentFormat: vi.fn(),
    CopyDocument: vi.fn(),
    RemoveMuseKey: vi.fn(),
    ResetData: vi.fn(),
    SetModel: vi.fn(),
    Questions: vi.fn().mockResolvedValue([]),
    ResolveQuestion: vi.fn(),
    AnswerQuestion: vi.fn(),
    DocumentHistory: vi.fn(),
    RestoreDocument: vi.fn(),
    ExportDocument: vi.fn(),
    CreateDocument: vi.fn().mockResolvedValue(state),
    ReadDocument: vi.fn(),
    SaveDocument: vi.fn(),
    SaveDocumentDraft: vi.fn(),
    DiscardDocumentDraft: vi.fn(),
    SaveDocumentView: vi.fn(),
    SendMessage: vi.fn().mockResolvedValue(state),
    RetryMessage: vi.fn().mockResolvedValue(state),
    StopRun: vi.fn().mockResolvedValue(undefined),
    Conversation: vi.fn().mockResolvedValue({ messages: state.messages, draft: state.draft }),
    SaveDraft: vi.fn().mockResolvedValue(undefined),
    ReconnectMuse: vi.fn().mockResolvedValue(fixture('muse')),
    OpenLink: vi.fn().mockResolvedValue(undefined),
    Bootstrap: vi.fn().mockResolvedValue(state),
    SaveNickname: vi.fn().mockResolvedValue({
      ...state,
      preferences: { ...state.preferences, nickname: 'Haki', stage: 'muse' },
    }),
    Back: vi.fn().mockResolvedValue(fixture()),
    ConnectMuse: vi.fn().mockResolvedValue(fixture('done')),
    SaveAppearance: vi.fn().mockResolvedValue(state),
    CreateProject: vi.fn().mockResolvedValue(state),
    CreateChat: vi.fn().mockResolvedValue(state),
    Select: vi.fn().mockResolvedValue(state),
    Rename: vi.fn().mockResolvedValue(state),
    Delete: vi.fn().mockResolvedValue(state),
    Search: vi.fn().mockResolvedValue([]),
    OpenDocument: vi.fn().mockResolvedValue(state),
    CloseDocument: vi.fn().mockResolvedValue(state),
    OpenMuseGuide: vi.fn().mockResolvedValue(undefined),
  }
  window.go = { main: { App: api } }
  return api
}
afterEach(() => {
  cleanup()
  delete window.go
  delete window.kikoWeb
  delete document.documentElement.dataset.theme
  vi.restoreAllMocks()
})

it('shows the splash only while actual startup is pending', async () => {
  const api = backend()
  let resolve!: (state: State) => void
  vi.mocked(api.Bootstrap).mockReturnValue(
    new Promise<State>((done) => {
      resolve = done
    }),
  )
  render(<App splashDuration={0} />)
  expect(screen.getByRole('status').textContent).toBe('Opening your workspace…')
  resolve(fixture())
  await screen.findByRole('heading', { name: 'What should I call you?' })
})
it('shows actionable startup failure and retries', async () => {
  const api = backend()
  vi.mocked(api.Bootstrap).mockRejectedValueOnce(new Error('Storage unavailable'))
  render(<App splashDuration={0} />)
  await screen.findByRole('alert')
  fireEvent.click(screen.getByRole('button', { name: 'Retry startup' }))
  await screen.findByLabelText('Nickname')
})
it('handles browser preview without claiming the desktop is connected', async () => {
  render(<App splashDuration={0} />)
  expect((await screen.findByRole('alert')).textContent).toBe('Open the desktop app to continue.')
})
it('saves a nickname and enters Muse setup', async () => {
  const api = backend()
  render(<App splashDuration={0} />)
  fireEvent.change(await screen.findByLabelText('Nickname'), { target: { value: 'Haki' } })
  fireEvent.click(screen.getByRole('button', { name: 'Continue' }))
  await screen.findByRole('heading', { name: 'Connect Meta Muse' })
  expect(api.SaveNickname).toHaveBeenCalledWith('Haki')
})
it('preserves the in-session key on failure and when going back', async () => {
  const api = backend(fixture('muse'))
  vi.mocked(api.ConnectMuse).mockRejectedValueOnce(new Error('Meta rejected this key.'))
  render(<App splashDuration={0} />)
  const input = await screen.findByLabelText('API key')
  fireEvent.change(input, { target: { value: 'synthetic-key' } })
  fireEvent.click(screen.getByRole('button', { name: 'Connect and continue' }))
  await screen.findByText('Meta rejected this key.')
  expect((input as HTMLInputElement).value).toBe('synthetic-key')
  fireEvent.click(screen.getByRole('button', { name: 'Show key' }))
  expect(input.getAttribute('type')).toBe('text')
  fireEvent.click(screen.getByRole('button', { name: 'Back' }))
  await screen.findByLabelText('Nickname')
  fireEvent.change(screen.getByLabelText('Nickname'), { target: { value: 'Haki' } })
  fireEvent.click(screen.getByRole('button', { name: 'Continue' }))
  expect(((await screen.findByLabelText('API key')) as HTMLInputElement).value).toBe(
    'synthetic-key',
  )
})
it('enters Home only after successful connection and avoids exposing the saved key', async () => {
  const api = backend(fixture('muse'))
  render(<App splashDuration={0} />)
  fireEvent.change(await screen.findByLabelText('API key'), { target: { value: 'synthetic-key' } })
  fireEvent.click(screen.getByRole('button', { name: 'Connect and continue' }))
  await screen.findByRole('heading', { name: 'What would you like to create?' })
  expect(api.ConnectMuse).toHaveBeenCalledWith('synthetic-key')
  expect(screen.queryByDisplayValue('synthetic-key')).toBeNull()
})
it('reconnects a returning user without replaying nickname setup', async () => {
  const state = fixture('done')
  state.connected = false
  backend(state)
  render(<App splashDuration={0} />)
  fireEvent.click(await screen.findByRole('button', { name: 'Reconnect Meta Muse' }))
  await screen.findByRole('heading', { name: 'Connect Meta Muse' })
  expect(screen.queryByLabelText('Nickname')).toBeNull()
})
it('selects a folder as the project root without a project creation form', async () => {
  const api = backend(fixture('done'))
  const next = fixture('done')
  next.projects = [{ id: 'p', name: 'Envi', description: '', root: '/work/Envi' }]
  next.selection.projectId = 'p'
  vi.mocked(api.ChooseFolder).mockResolvedValue(next)
  render(<App splashDuration={0} />)
  const workspace = await screen.findByRole('button', { name: 'This workspace' })
  vi.mocked(api.Bootstrap).mockResolvedValue(next)
  fireEvent.click(workspace)
  await screen.findByText('What should we create in Envi?')
  expect(api.ChooseFolder).toHaveBeenCalledWith('')
  expect(screen.queryByRole('button', { name: 'New project' })).toBeNull()
  expect(screen.queryByText('Create a space for your work')).toBeNull()
})
it('supports sidebar keyboard resizing and persisted theme selection', async () => {
  const api = backend(fixture('done'))
  render(<App splashDuration={0} />)
  await screen.findByRole('separator', { name: 'Sidebar width' })
  fireEvent.keyDown(screen.getByRole('separator'), { key: 'ArrowLeft' })
  await waitFor(() => expect(api.SaveAppearance).toHaveBeenCalledWith('dark', true, 252))
  const light = fixture('done')
  light.preferences.theme = 'light'
  vi.mocked(api.SaveAppearance).mockResolvedValue(light)
  fireEvent.click(screen.getByRole('button', { name: 'Settings' }))
  fireEvent.click(await screen.findByRole('radio', { name: 'Light' }))
  await waitFor(() => expect(document.documentElement.dataset.theme).toBe('light'))
})
it('searches saved items and opens a matching chat', async () => {
  const api = backend(fixture('done'))
  vi.mocked(api.Search).mockResolvedValue([
    { kind: 'chat', id: 'c', chatId: 'c', projectId: '', title: 'Budget planning' },
  ])
  render(<App splashDuration={0} />)
  fireEvent.click(await screen.findByRole('button', { name: 'Search' }))
  const dialog = await screen.findByRole('dialog')
  fireEvent.change(within(dialog).getByLabelText('Search query'), { target: { value: 'Budget' } })
  fireEvent.click(within(dialog).getByRole('button', { name: 'Search' }))
  fireEvent.click(await screen.findByRole('button', { name: /Budget planning/ }))
  await waitFor(() => expect(api.Select).toHaveBeenCalledWith('', 'c'))
})
it('requires an explicit confirmation before deleting a project', async () => {
  const state = fixture('done')
  state.projects = [{ id: 'p', name: 'Envi', description: '' }]
  const api = backend(state)
  render(<App splashDuration={0} />)
  const menuButton = await screen.findByRole('button', { name: 'Actions for Envi' })
  fireEvent.keyDown(menuButton, { key: 'ArrowDown' })
  fireEvent.click(await screen.findByRole('menuitem', { name: 'Delete' }))
  const dialog = await screen.findByRole('dialog')
  expect(api.Delete).not.toHaveBeenCalled()
  fireEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))
  await waitFor(() => expect(api.Delete).toHaveBeenCalledWith('project', 'p'))
})

it('moves keyboard focus into settings and back to the chat', async () => {
  backend(fixture('done'))
  render(<App splashDuration={0} />)
  await screen.findByRole('heading', { name: 'What would you like to create?' })
  const trigger = screen.getByRole('button', { name: 'Settings' })
  trigger.focus()
  fireEvent.click(trigger)
  const back = await screen.findByRole('button', { name: 'Back' })
  await waitFor(() => expect(back.contains(document.activeElement)).toBe(true))
  fireEvent.click(back)
  await screen.findByRole('heading', { name: 'What would you like to create?' })
  expect(screen.queryByRole('heading', { name: 'Settings' })).toBeNull()
})
it('creates a standalone chat ready for messaging', async () => {
  const api = backend(fixture('done'))
  const state = fixture('done')
  state.chats = [{ id: 'c', projectId: '', title: 'New chat' }]
  state.selection.chatId = 'c'
  vi.mocked(api.CreateChat).mockResolvedValue(state)
  render(<App splashDuration={0} />)
  await screen.findByRole('heading', { name: 'What would you like to create?' })
  fireEvent.click(screen.getByRole('button', { name: 'New chat' }))
  await screen.findByLabelText('Message')
  expect(api.CreateChat).toHaveBeenCalledWith('')
  expect((screen.getByLabelText('Message') as HTMLTextAreaElement).disabled).toBe(false)
})
it('renames a project through the keyboard-accessible action menu', async () => {
  const state = fixture('done')
  state.projects = [{ id: 'p', name: 'Envi', description: '' }]
  const api = backend(state)
  const renamed = structuredClone(state)
  renamed.projects[0].name = 'Envi v2'
  vi.mocked(api.Rename).mockResolvedValue(renamed)
  render(<App splashDuration={0} />)
  fireEvent.keyDown(await screen.findByRole('button', { name: 'Actions for Envi' }), {
    key: 'ArrowDown',
  })
  fireEvent.click(await screen.findByRole('menuitem', { name: 'Rename' }))
  const dialog = await screen.findByRole('dialog')
  fireEvent.change(within(dialog).getByLabelText('Name'), { target: { value: 'Envi v2' } })
  fireEvent.click(within(dialog).getByRole('button', { name: 'Save name' }))
  await screen.findByRole('button', { name: 'Actions for Envi v2' })
  expect(api.Rename).toHaveBeenCalledWith('project', 'p', 'Envi v2')
})

it('recovers an interrupted reset through an explicit retry', async () => {
  const state = fixture('done')
  state.preferences.resetInProgress = true
  const api = backend(state)
  vi.mocked(api.ResetData).mockResolvedValue(fixture())
  render(<App splashDuration={0} />)
  await screen.findByRole('heading', { name: 'Finish resetting Kiko' })
  fireEvent.click(screen.getByRole('button', { name: 'Retry reset' }))
  await screen.findByLabelText('Nickname')
  expect(api.ResetData).toHaveBeenCalledTimes(1)
})
it('removes a saved key without deleting the workspace', async () => {
  const state = fixture('done')
  const api = backend(state)
  vi.mocked(api.RemoveMuseKey).mockResolvedValue({ ...state, connected: false })
  render(<App splashDuration={0} />)
  fireEvent.click(await screen.findByRole('button', { name: 'Settings' }))
  fireEvent.click(screen.getByRole('button', { name: 'API key' }))
  fireEvent.click(screen.getByRole('button', { name: 'Remove saved key' }))
  await waitFor(() => expect(api.RemoveMuseKey).toHaveBeenCalledTimes(1))
  expect(screen.getByText('Not connected')).toBeTruthy()
})

it('reports web storage startup errors without sending an authenticated user back to login', async () => {
  const api = backend(fixture())
  window.kikoWeb = true
  vi.mocked(api.Bootstrap).mockRejectedValue(new Error('Could not read local data'))
  render(<App splashDuration={0} />)
  await screen.findByRole('button', { name: 'Retry startup' })
  expect(screen.getByRole('alert').textContent).toContain('Could not read local data')
  expect(screen.queryByLabelText('Server login token')).toBeNull()
})
it('shows web login only for an authentication failure', async () => {
  const api = backend(fixture())
  window.kikoWeb = true
  vi.mocked(api.Bootstrap).mockRejectedValue(new WebAuthenticationError('Sign in'))
  render(<App splashDuration={0} />)
  await screen.findByLabelText('Server login token')
})
it('applies document edits directly with no review step', async () => {
  backend(fixture('done'))
  render(<App splashDuration={0} />)
  fireEvent.click(await screen.findByRole('button', { name: 'Settings' }))
  expect(
    screen.queryByRole('checkbox', { name: 'Review AI document edits as suggestions' }),
  ).toBeNull()
})
it('organizes settings into sections and preserves an unsaved key while switching', async () => {
  backend(fixture('done'))
  render(<App splashDuration={0} />)
  fireEvent.click(await screen.findByRole('button', { name: 'Settings' }))
  expect(screen.getByRole('radio', { name: 'Dark' })).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Remove saved key' })).toBeNull()
  fireEvent.click(screen.getByRole('button', { name: 'API key' }))
  fireEvent.change(screen.getByLabelText('Replace API key'), {
    target: { value: 'unsaved-fixture' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Research' }))
  expect(screen.getByRole('heading', { name: 'Web research' })).toBeTruthy()
  expect(screen.queryByRole('radio', { name: 'Dark' })).toBeNull()
  fireEvent.click(screen.getByRole('button', { name: 'API key' }))
  expect((screen.getByLabelText('Replace API key') as HTMLInputElement).value).toBe(
    'unsaved-fixture',
  )
  fireEvent.click(screen.getByRole('button', { name: 'Data' }))
  expect(screen.getByRole('button', { name: 'Reset Kiko' })).toBeTruthy()
})
it('settings only manages the Muse API key and offers no other provider', async () => {
  const api = backend(fixture('done'))
  vi.mocked(api.ConnectMuse).mockResolvedValue(fixture('done'))
  render(<App splashDuration={0} />)
  fireEvent.click(await screen.findByRole('button', { name: 'Settings' }))
  fireEvent.click(screen.getByRole('button', { name: 'API key' }))
  expect(screen.getByRole('heading', { name: 'API key' })).toBeTruthy()
  for (const gone of [
    'Sign in a provider',
    'Vision helper',
    'Provider for this chat and new chats',
  ])
    expect(screen.queryByText(gone)).toBeNull()
  expect(screen.queryByRole('checkbox')).toBeNull()
  fireEvent.change(screen.getByLabelText('Replace API key'), { target: { value: 'new-key' } })
  fireEvent.click(screen.getByRole('button', { name: 'Verify and save key' }))
  await waitFor(() => expect(api.ConnectMuse).toHaveBeenCalledWith('new-key'))
})
it('chooses the Muse model for the chat from the composer', async () => {
  const api = backend(fixture('done'))
  vi.mocked(api.SetModel).mockResolvedValue({ ...fixture('done'), model: 'muse-ember-1.0' })
  render(<App splashDuration={0} />)
  fireEvent.pointerDown(await screen.findByRole('button', { name: 'Model' }))
  expect(await screen.findAllByRole('menuitem')).toHaveLength(models.length)
  fireEvent.click(screen.getByRole('menuitem', { name: /Muse Ember · Standard/ }))
  await waitFor(() => expect(api.SetModel).toHaveBeenCalledWith('muse-ember-1.0'))
})
it('opens search with the command shortcut', async () => {
  backend(fixture('done'))
  render(<App splashDuration={0} />)
  await screen.findByRole('button', { name: 'Settings' })
  fireEvent.keyDown(window, { key: 'k', metaKey: true })
  expect(await screen.findByRole('dialog', { name: 'Search' })).toBeTruthy()
})

it('keeps navigation available after collapsing the sidebar', async () => {
  const state = fixture('done')
  const api = backend(state)
  vi.mocked(api.SaveAppearance).mockResolvedValue({
    ...state,
    preferences: { ...state.preferences, sidebarOpen: false },
  })
  render(<App splashDuration={0} />)
  fireEvent.click(await screen.findByRole('button', { name: 'Collapse sidebar' }))
  await screen.findByRole('button', { name: 'Expand sidebar' })
  expect(screen.queryByRole('complementary', { name: 'Chats and projects' })).toBeNull()
  expect(screen.getByRole('button', { name: 'Search' })).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Settings' })).toBeTruthy()
  await waitFor(() =>
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Expand sidebar' })),
  )
})
it('lets a new workspace save and send a draft directly from Home', async () => {
  const state = fixture('done')
  const api = backend(state)
  render(<App splashDuration={0} />)
  const input = await screen.findByRole('textbox', { name: 'Message' })
  fireEvent.change(input, { target: { value: 'Plan my first document' } })
  await waitFor(() => expect(api.SaveDraft).toHaveBeenCalledWith('', '', 'Plan my first document'))
  fireEvent.click(screen.getByRole('button', { name: 'Send message' }))
  await waitFor(() =>
    expect(api.SendMessage).toHaveBeenCalledWith(
      '',
      '',
      'Plan my first document',
      expect.any(String),
    ),
  )
})
it('groups the selected project’s chats and keeps other chats in Recents', async () => {
  const state = fixture('done')
  state.projects = [{ id: 'p', name: 'Release', description: '' }]
  state.selection = { projectId: 'p', chatId: '' }
  state.chats = [
    { id: 'c', projectId: 'p', title: 'Launch plan' },
    { id: 'other', projectId: '', title: 'Another idea' },
  ]
  const api = backend(state)
  render(<App splashDuration={0} />)
  await screen.findByRole('heading', { name: 'What should we create in Release?' })
  const branch = screen.getByLabelText('Release chats')
  expect(within(branch).getByRole('button', { name: 'Launch plan' })).toBeTruthy()
  expect(within(branch).queryByRole('button', { name: 'Another idea' })).toBeNull()
  fireEvent.click(screen.getByRole('button', { name: 'Another idea' }))
  await waitFor(() => expect(api.Select).toHaveBeenCalledWith('', 'other'))
})
it('opens editing preferences from the composer context strip', async () => {
  backend(fixture('done'))
  render(<App splashDuration={0} />)
  fireEvent.click(await screen.findByRole('button', { name: 'Conversation settings' }))
  expect(screen.getByRole('heading', { name: 'General' })).toBeTruthy()
  expect(screen.getByLabelText('Nickname')).toBeTruthy()
})

it('keeps the initial splash visible for three seconds after fast startup', async () => {
  vi.useFakeTimers()
  try {
    backend()
    render(<App />)
    await act(async () => {
      await Promise.resolve()
    })
    expect(screen.getByRole('status').textContent).toBe('Opening your workspace…')
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2999)
    })
    expect(screen.queryByLabelText('Nickname')).toBeNull()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1)
    })
    expect(screen.getByLabelText('Nickname')).toBeTruthy()
  } finally {
    vi.useRealTimers()
  }
})
it('waits for actual startup when it takes longer than the splash minimum', async () => {
  vi.useFakeTimers()
  try {
    const api = backend()
    let finish!: (value: State) => void
    vi.mocked(api.Bootstrap).mockReturnValue(
      new Promise((resolve) => {
        finish = resolve
      }),
    )
    render(<App />)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000)
    })
    expect(screen.getByRole('status')).toBeTruthy()
    await act(async () => {
      finish(fixture())
    })
    expect(screen.getByLabelText('Nickname')).toBeTruthy()
  } finally {
    vi.useRealTimers()
  }
})
it('imports from Home into a new chat while preserving its draft without sending it', async () => {
  const state = fixture('done')
  const api = backend(state)
  const next = {
    ...state,
    selection: { projectId: '', chatId: 'new-chat' },
    chats: [{ id: 'new-chat', projectId: '', title: 'New chat' }],
    draft: 'Keep this draft',
  }
  vi.mocked(api.CreateChat).mockResolvedValue(next)
  vi.mocked(api.Conversation).mockImplementation(async (_project, chat) => ({
    messages: [],
    draft: chat === 'new-chat' ? 'Keep this draft' : '',
  }))
  vi.mocked(api.AttachFile).mockResolvedValue({
    id: 'file',
    chatId: 'new-chat',
    name: 'notes.txt',
    mime: 'text/plain',
    size: 5,
    pages: 1,
  })
  render(<App splashDuration={0} />)
  const input = await screen.findByRole('textbox', { name: 'Message' })
  fireEvent.change(input, { target: { value: 'Keep this draft' } })
  vi.mocked(api.Bootstrap).mockResolvedValue(next)
  fireEvent.change(screen.getByLabelText('Choose attachment'), {
    target: { files: [new File(['Hello'], 'notes.txt', { type: 'text/plain' })] },
  })
  await waitFor(() =>
    expect(api.AttachFile).toHaveBeenCalledWith('new-chat', 'notes.txt', 'SGVsbG8='),
  )
  await waitFor(() => expect(api.SaveDraft).toHaveBeenCalledWith('', 'new-chat', 'Keep this draft'))
  expect(api.SendMessage).not.toHaveBeenCalled()
  expect((screen.getByRole('textbox', { name: 'Message' }) as HTMLTextAreaElement).value).toBe(
    'Keep this draft',
  )
})
it('opens a created file from its reply instead of a toolbar strip', async () => {
  const state = fixture('done')
  state.selection.chatId = 'chat'
  state.chats = [{ id: 'chat', projectId: '', title: 'Plan' }]
  state.documents = [{ id: 'doc', chatId: 'chat', projectId: '', title: 'Release plan' }]
  state.messages = [
    {
      id: 'reply',
      chatId: 'chat',
      role: 'assistant',
      content: 'Your plan is ready.',
      status: 'complete',
      ordinal: 1,
      runId: 'run',
      error: '',
      errorKind: '',
      sequence: 1,
      attempt: 1,
      previous: [],
      steps: [
        {
          id: 'step',
          tool: 'create_document',
          label: 'Creating document',
          status: 'ok',
          outputId: 'doc',
        },
      ],
    },
  ]
  const api = backend(state)
  render(<App splashDuration={0} />)
  const file = await screen.findByRole('button', { name: /Release plan.*Document/ })
  expect(file.closest('article')?.getAttribute('aria-label')).toBe('Kiko reply')
  expect(document.querySelector('.artifact-list')).toBeNull()
  fireEvent.click(file)
  await waitFor(() => expect(api.OpenDocument).toHaveBeenCalledWith('doc'))
})

it('shows an accessible file-opening error instead of hiding it behind the active pane', async () => {
  const state = fixture('done')
  state.projects = [{ id: 'p', name: 'Kiko', description: '', root: '/work/Kiko' }]
  state.selection.projectId = 'p'
  const api = backend(state)
  vi.mocked(api.ListWorkspaceDirectory).mockResolvedValue([{ name: 'image.png', directory: false }])
  vi.mocked(api.ReadWorkspaceEntry).mockRejectedValue(new Error('Choose a text file up to 1 MB.'))
  render(<App splashDuration={0} />)
  fireEvent.click(await screen.findByRole('button', { name: 'image.png' }))
  const dialog = await screen.findByRole('dialog', { name: 'Couldn’t open this file' })
  expect(within(dialog).getByText('Choose a text file up to 1 MB.')).toBeTruthy()
  fireEvent.click(within(dialog).getByRole('button', { name: 'Close' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
})
