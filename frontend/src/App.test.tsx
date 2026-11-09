import { WebAuthenticationError } from './web-api'
import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import App from './App'
import type { Backend, State } from './api'

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
  }
}
function backend(state = fixture()): Backend {
  const api = {
    SetSuggestionMode: vi.fn(),
    AcceptSuggestion: vi.fn(),
    RejectSuggestion: vi.fn(),
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
  render(<App />)
  expect(screen.getByRole('status').textContent).toBe('Opening your workspace…')
  resolve(fixture())
  await screen.findByRole('heading', { name: 'What should I call you?' })
})
it('shows actionable startup failure and retries', async () => {
  const api = backend()
  vi.mocked(api.Bootstrap).mockRejectedValueOnce(new Error('Storage unavailable'))
  render(<App />)
  await screen.findByRole('alert')
  fireEvent.click(screen.getByRole('button', { name: 'Retry startup' }))
  await screen.findByLabelText('Nickname')
})
it('handles browser preview without claiming the desktop is connected', async () => {
  render(<App />)
  expect((await screen.findByRole('alert')).textContent).toBe('Open the desktop app to continue.')
})
it('saves a nickname and enters Muse setup', async () => {
  const api = backend()
  render(<App />)
  fireEvent.change(await screen.findByLabelText('Nickname'), { target: { value: 'Haki' } })
  fireEvent.click(screen.getByRole('button', { name: 'Continue' }))
  await screen.findByRole('heading', { name: 'Connect Meta Muse' })
  expect(api.SaveNickname).toHaveBeenCalledWith('Haki')
})
it('preserves the in-session key on failure and when going back', async () => {
  const api = backend(fixture('muse'))
  vi.mocked(api.ConnectMuse).mockRejectedValueOnce(new Error('Meta rejected this key.'))
  render(<App />)
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
  render(<App />)
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
  render(<App />)
  fireEvent.click(await screen.findByRole('button', { name: 'Reconnect Meta Muse' }))
  await screen.findByRole('heading', { name: 'Connect Meta Muse' })
  expect(screen.queryByLabelText('Nickname')).toBeNull()
})
it('creates a project with an optional description and focuses its workspace', async () => {
  const api = backend(fixture('done'))
  const next = fixture('done')
  next.projects = [{ id: 'p', name: 'Envi', description: 'Secrets' }]
  next.selection.projectId = 'p'
  vi.mocked(api.CreateProject).mockResolvedValue(next)
  render(<App />)
  await screen.findByRole('heading', { name: 'What would you like to create?' })
  fireEvent.click(screen.getAllByRole('button', { name: 'New project' })[0])
  const dialog = await screen.findByRole('dialog')
  fireEvent.change(within(dialog).getByLabelText('Project name'), { target: { value: 'Envi' } })
  fireEvent.change(within(dialog).getByLabelText('Description (optional)'), {
    target: { value: 'Secrets' },
  })
  fireEvent.click(within(dialog).getByRole('button', { name: 'Create project' }))
  await screen.findByText('What should we create in Envi?')
  expect(api.CreateProject).toHaveBeenCalledWith('Envi', 'Secrets')
  expect(screen.queryByRole('dialog')).toBeNull()
})
it('supports sidebar keyboard resizing and persisted theme selection', async () => {
  const api = backend(fixture('done'))
  render(<App />)
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
  render(<App />)
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
  render(<App />)
  const menuButton = await screen.findByRole('button', { name: 'Actions for Envi' })
  fireEvent.keyDown(menuButton, { key: 'ArrowDown' })
  fireEvent.click(await screen.findByRole('menuitem', { name: 'Delete' }))
  const dialog = await screen.findByRole('dialog')
  expect(api.Delete).not.toHaveBeenCalled()
  fireEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))
  await waitFor(() => expect(api.Delete).toHaveBeenCalledWith('project', 'p'))
})

it('returns keyboard focus to the trigger after dismissing a dialog', async () => {
  backend(fixture('done'))
  render(<App />)
  await screen.findByRole('heading', { name: 'What would you like to create?' })
  const trigger = screen.getAllByRole('button', { name: 'New project' })[0]
  trigger.focus()
  fireEvent.click(trigger)
  const dialog = await screen.findByRole('dialog')
  await waitFor(() => expect(dialog.contains(document.activeElement)).toBe(true))
  fireEvent.keyDown(document.activeElement!, { key: 'Escape' })
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  await waitFor(() => expect(document.activeElement).toBe(trigger))
})
it('creates a standalone chat ready for messaging', async () => {
  const api = backend(fixture('done'))
  const state = fixture('done')
  state.chats = [{ id: 'c', projectId: '', title: 'New chat' }]
  state.selection.chatId = 'c'
  vi.mocked(api.CreateChat).mockResolvedValue(state)
  render(<App />)
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
  render(<App />)
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
  render(<App />)
  await screen.findByRole('heading', { name: 'Finish resetting Kiko' })
  fireEvent.click(screen.getByRole('button', { name: 'Retry reset' }))
  await screen.findByLabelText('Nickname')
  expect(api.ResetData).toHaveBeenCalledTimes(1)
})
it('removes a saved key without deleting the workspace', async () => {
  const state = fixture('done')
  const api = backend(state)
  vi.mocked(api.RemoveMuseKey).mockResolvedValue({ ...state, connected: false })
  render(<App />)
  fireEvent.click(await screen.findByRole('button', { name: 'Settings' }))
  fireEvent.click(screen.getByRole('button', { name: 'AI & models' }))
  fireEvent.click(screen.getByRole('button', { name: 'Remove saved key' }))
  await waitFor(() => expect(api.RemoveMuseKey).toHaveBeenCalledTimes(1))
  expect(screen.getByText('Disconnected · Muse Spark 1.3 Contributor')).toBeTruthy()
})

it('reports web storage startup errors without sending an authenticated user back to login', async () => {
  const api = backend(fixture())
  window.kikoWeb = true
  vi.mocked(api.Bootstrap).mockRejectedValue(new Error('Could not read local data'))
  render(<App />)
  await screen.findByRole('button', { name: 'Retry startup' })
  expect(screen.getByRole('alert').textContent).toContain('Could not read local data')
  expect(screen.queryByLabelText('Server login token')).toBeNull()
})
it('shows web login only for an authentication failure', async () => {
  const api = backend(fixture())
  window.kikoWeb = true
  vi.mocked(api.Bootstrap).mockRejectedValue(new WebAuthenticationError('Sign in'))
  render(<App />)
  await screen.findByLabelText('Server login token')
})
it('requires a separate explicit action to enable direct images for a configured model', async () => {
  const state = fixture('done')
  state.provider = {
    id: 'openai',
    format: 'openai-chat',
    baseURL: 'https://api.openai.com/v1',
    model: 'fixture-vision',
  }
  state.providers = [state.provider]
  const api = backend(state)
  vi.mocked(api.SetDirectImages).mockResolvedValue({
    ...state,
    preferences: { ...state.preferences, imageModels: { openai: 'fixture-vision' } },
  })
  render(<App />)
  fireEvent.click(await screen.findByRole('button', { name: 'Settings' }))
  fireEvent.click(screen.getByRole('button', { name: 'AI & models' }))
  const checkbox = screen.getByRole('checkbox', {
    name: 'Send page and diagram images directly to openai · fixture-vision',
  })
  expect((checkbox as HTMLInputElement).checked).toBe(false)
  fireEvent.click(checkbox)
  await waitFor(() => expect(api.SetDirectImages).toHaveBeenCalledWith('openai', true))
  expect(
    (
      screen.getByRole('checkbox', {
        name: 'Send page and diagram images directly to openai · fixture-vision',
      }) as HTMLInputElement
    ).checked,
  ).toBe(true)
})
it('captures suggestion mode before the save guard rerenders the controlled checkbox', async () => {
  const state = fixture('done')
  const api = backend(state)
  vi.mocked(api.SetSuggestionMode).mockResolvedValue({
    ...state,
    preferences: { ...state.preferences, suggestionMode: true },
  })
  render(<App />)
  fireEvent.click(await screen.findByRole('button', { name: 'Settings' }))
  fireEvent.click(screen.getByRole('checkbox', { name: 'Review AI document edits as suggestions' }))
  await waitFor(() => expect(api.SetSuggestionMode).toHaveBeenCalledWith(true))
})
it('captures provider and helper choices before awaiting guarded navigation', async () => {
  const state = fixture('done')
  const muse = {
    id: 'muse',
    format: 'openai-chat',
    baseURL: 'https://api.meta.ai/v1',
    model: 'muse-spark-1.3-contributor',
  }
  const openai = {
    id: 'openai',
    format: 'openai-chat',
    baseURL: 'https://api.openai.com/v1',
    model: 'fixture-vision',
  }
  state.provider = muse
  state.providers = [muse, openai]
  const api = backend(state)
  vi.mocked(api.SelectProvider).mockResolvedValue({ ...state, provider: openai })
  vi.mocked(api.SetVisionProvider).mockResolvedValue({
    ...state,
    provider: openai,
    preferences: { ...state.preferences, visionProviderId: 'openai' },
  })
  render(<App />)
  fireEvent.click(await screen.findByRole('button', { name: 'Settings' }))
  fireEvent.click(screen.getByRole('button', { name: 'AI & models' }))
  fireEvent.change(screen.getByLabelText('Provider for this chat and new chats'), {
    target: { value: 'openai' },
  })
  await waitFor(() => expect(api.SelectProvider).toHaveBeenCalledWith('openai'))
  fireEvent.change(screen.getByLabelText('Vision helper'), { target: { value: 'openai' } })
  await waitFor(() => expect(api.SetVisionProvider).toHaveBeenCalledWith('openai'))
})

it('organizes settings into sections and preserves an unsaved key while switching', async () => {
  backend(fixture('done'))
  render(<App />)
  fireEvent.click(await screen.findByRole('button', { name: 'Settings' }))
  expect(screen.getByRole('radio', { name: 'Dark' })).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Remove saved key' })).toBeNull()
  fireEvent.click(screen.getByRole('button', { name: 'AI & models' }))
  fireEvent.change(screen.getByLabelText('Replace Muse API key'), {
    target: { value: 'unsaved-fixture' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Research' }))
  expect(screen.getByRole('heading', { name: 'Web research' })).toBeTruthy()
  expect(screen.queryByRole('radio', { name: 'Dark' })).toBeNull()
  fireEvent.click(screen.getByRole('button', { name: 'AI & models' }))
  expect((screen.getByLabelText('Replace Muse API key') as HTMLInputElement).value).toBe(
    'unsaved-fixture',
  )
  fireEvent.click(screen.getByRole('button', { name: 'Data' }))
  expect(screen.getByRole('button', { name: 'Reset Kiko' })).toBeTruthy()
})
it('opens search with the command shortcut', async () => {
  backend(fixture('done'))
  render(<App />)
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
  render(<App />)
  fireEvent.click(await screen.findByRole('button', { name: 'Collapse sidebar' }))
  await screen.findByRole('button', { name: 'Expand sidebar' })
  expect(screen.queryByRole('complementary', { name: 'Chats and projects' })).toBeNull()
  expect(screen.getByRole('button', { name: 'Search' })).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Settings' })).toBeTruthy()
  await waitFor(() =>
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Expand sidebar' })),
  )
})
it('opens AI settings from the composer without sending a message', async () => {
  const state = fixture('done')
  state.chats = [{ id: 'c', projectId: '', title: 'New chat' }]
  state.selection.chatId = 'c'
  const api = backend(state)
  render(<App />)
  fireEvent.click(await screen.findByRole('button', { name: 'Model settings' }))
  expect(screen.getByRole('heading', { name: 'AI & models' })).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Remove saved key' })).toBeTruthy()
  expect(api.SendMessage).not.toHaveBeenCalled()
})

it('lets a new workspace save and send a draft directly from Home', async () => {
  const state = fixture('done')
  const api = backend(state)
  render(<App />)
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
  render(<App />)
  await screen.findByRole('heading', { name: 'What should we create in Release?' })
  const branch = screen.getByLabelText('Release chats')
  expect(within(branch).getByRole('button', { name: 'Launch plan' })).toBeTruthy()
  expect(within(branch).queryByRole('button', { name: 'Another idea' })).toBeNull()
  fireEvent.click(screen.getByRole('button', { name: 'Another idea' }))
  await waitFor(() => expect(api.Select).toHaveBeenCalledWith('', 'other'))
})
it('opens editing preferences from the composer context strip', async () => {
  backend(fixture('done'))
  render(<App />)
  fireEvent.click(await screen.findByRole('button', { name: 'Conversation settings' }))
  expect(
    screen.getByRole('checkbox', { name: 'Review AI document edits as suggestions' }),
  ).toBeTruthy()
})
