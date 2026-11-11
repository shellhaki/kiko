import { webBackend } from './web-api'
import type { ArtifactRecord } from './artifact-model'
import type { Block, DocumentRead, DocumentView, SavedDocument } from './document-model'
export type ChatMessage = {
  steps?: {
    id: string
    tool: string
    label: string
    status: string
    outputId?: string
    detail?: unknown
  }[]
  id: string
  chatId: string
  role: 'user' | 'assistant'
  content: string
  ordinal: number
  runId: string
  status: 'complete' | 'running' | 'stopped' | 'error' | 'interrupted' | 'filtered' | 'truncated'
  error: string
  errorKind: string
  sequence: number
  attempt: number
  previous: { content: string; status: string; error: string }[]
}
export type ConversationData = { messages: ChatMessage[]; draft: string }
export type MuseModel = {
  id: string
  name: string
  mode: 'standard' | 'contributor'
  description: string
}
export type Preferences = {
  suggestionMode?: boolean
  tavilyEnabled?: boolean
  nickname: string
  stage: 'nickname' | 'muse' | 'done'
  theme: 'dark' | 'light' | 'system'
  sidebarOpen: boolean
  sidebarWidth: number
  resetInProgress: boolean
}
export type Project = { id: string; name: string; description: string; root?: string }
export type Chat = { id: string; projectId: string; title: string }
export type Document = { id: string; chatId: string; projectId: string; title: string }
export type State = {
  model: string
  models: MuseModel[]
  artifacts?: ArtifactRecord[]
  artifactId?: string
  messages: ChatMessage[]
  draft: string
  connectionIssue?: string
  preferences: Preferences
  projects: Project[]
  chats: Chat[]
  documents: Document[]
  selection: { projectId: string; chatId: string }
  documentId: string
  connected: boolean
}
export type SearchResult = {
  kind: 'chat' | 'document'
  id: string
  title: string
  chatId: string
  projectId: string
}
export type Question = {
  id: string
  documentId: string
  blockId: string
  start: number
  end: number
  quote: string
  body: string
  suggested: string[]
  resolved: boolean
}
export type Attachment = {
  id: string
  chatId: string
  name: string
  mime: string
  size: number
  pages: number
}
export type WorkspaceFile = { chatId: string; path: string; sourceId: string; hash: string }
export type FilePermission = {
  id: string
  chatId: string
  path: string
  content: string
  kind: string
}
export type WorkspaceEntry = {
  path: string
  content: string
  hash: string
  sourceId: string
  chatId: string
  kind: string
  draft?: { content: string; baseHash: string }
}
export type WorkspaceSource = {
  id: string
  chatId: string
  kind: string
  title: string
  path: string
  rev: number
  content: string
  diskContent: string
  diskHash: string
  status: 'unlinked' | 'synced' | 'changed' | 'pending' | 'missing' | 'unavailable'
  issue: string
  draft: boolean
  running: boolean
}
export type Backend = {
  ListWorkspaceDirectory(
    project: string,
    path: string,
  ): Promise<{ name: string; directory: boolean }[]>
  ReadWorkspaceEntry(project: string, path: string): Promise<WorkspaceEntry>
  SaveWorkspaceTextDraft(
    project: string,
    path: string,
    hash: string,
    content: string,
  ): Promise<void>
  SaveWorkspaceText(
    project: string,
    chat: string,
    path: string,
    hash: string,
    content: string,
  ): Promise<WorkspaceEntry>
  DiscardWorkspaceTextDraft(project: string, path: string, hash: string): Promise<void>
  ImportWorkspaceFile(project: string, chat: string, path: string, hash: string): Promise<State>
  SaveWorkspaceSourceVersion(id: string, hash: string, rev: number): Promise<State>
  WorkspaceSource(id: string): Promise<WorkspaceSource | null>
  ReloadWorkspaceSource(id: string, hash: string, rev: number): Promise<State>
  CreateArtifactAt(chat: string, kind: string, title: string, path: string): Promise<State>
  SetArtifactPath(chat: string, id: string, path: string): Promise<State>
  WorkspaceIssues(chat: string): Promise<{ sourceId: string; message: string }[]>
  RetryWorkspaceSync(chat: string, source: string): Promise<void>
  ChooseFolder(chat: string): Promise<State>
  WorkspaceFiles(chat: string): Promise<WorkspaceFile[]>
  FilePermissions(chat: string): Promise<FilePermission[]>
  ResolveFilePermission(id: string, approved: boolean): Promise<void>
  ReadWorkspaceFile(
    chat: string,
    path: string,
  ): Promise<{ path: string; content: string; hash: string }>

  RenderAttachmentPage(chat: string, id: string, page: number): Promise<string>
  CreateDeck(chat: string, title: string): Promise<State>
  ReadArtifact(id: string): Promise<ArtifactRecord>
  SaveArtifact(id: string, rev: number, payload: unknown): Promise<ArtifactRecord>
  DiscardArtifactDraft(id: string): Promise<void>
  SaveArtifactDraft(id: string, rev: number, payload: unknown): Promise<void>
  OpenArtifact(id: string): Promise<State>
  CloseArtifact(): Promise<State>
  CreateSheet(chat: string, title: string): Promise<State>
  ExportArtifact(id: string, format: string, tab: string): Promise<boolean>
  ExportDeckPptx(id: string, encoded: string): Promise<boolean>
  AttachmentBytes(chat: string, id: string): Promise<string>
  ValidateDiagram(code: string): Promise<string>
  AttachFile(chat: string, name: string, data: string): Promise<Attachment>
  Attachments(chat: string): Promise<Attachment[]>
  ReadAttachment(chat: string, id: string, page: number): Promise<{ name: string; text: string }[]>
  RemoveAttachment(chat: string, id: string): Promise<void>
  AttachmentPreview(chat: string, id: string): Promise<string>
  ConfigureResearch(key: string): Promise<State>
  RemoveResearch(): Promise<State>
  ExportFormats(): Promise<string[]>
  ExportDocumentFormat(id: string, format: string): Promise<boolean>
  CopyDocument(id: string): Promise<void>
  RemoveMuseKey(): Promise<State>
  ResetData(): Promise<State>
  SetModel(model: string): Promise<State>
  Questions(id: string): Promise<Question[]>
  ResolveQuestion(id: string): Promise<void>
  AnswerQuestion(id: string, text: string, requestId: string): Promise<State>
  DocumentHistory(id: string): Promise<{ rev: number; author: string; createdAt: number }[]>
  RestoreDocument(id: string, expected: number, revision: number): Promise<SavedDocument>
  ExportDocument(id: string): Promise<boolean>
  CreateDocument(chatId: string, title: string, markdown: string): Promise<State>
  ReadDocument(
    id: string,
    options: { atRev?: number; sinceRev?: number; parentId?: string; search?: string },
  ): Promise<DocumentRead>
  SaveDocument(id: string, rev: number, blocks: Block[]): Promise<SavedDocument>
  SaveDocumentDraft(id: string, rev: number, blocks: Block[]): Promise<void>
  DiscardDocumentDraft(id: string): Promise<void>
  SaveDocumentView(id: string, state: DocumentView): Promise<void>
  SendMessage(projectId: string, chatId: string, text: string, requestId: string): Promise<State>
  RetryMessage(runId: string, requestId: string): Promise<State>
  StopRun(id: string, attempt: number): Promise<void>
  Conversation(projectId: string, chatId: string): Promise<ConversationData>
  SaveDraft(projectId: string, chatId: string, content: string): Promise<void>
  ReconnectMuse(): Promise<State>
  OpenLink(url: string): Promise<void>
  Bootstrap(): Promise<State>
  SaveNickname(name: string): Promise<State>
  Back(): Promise<State>
  ConnectMuse(key: string): Promise<State>
  SaveAppearance(theme: string, open: boolean, width: number): Promise<State>
  CreateProject(name: string, description: string): Promise<State>
  CreateChat(projectId: string): Promise<State>
  Select(projectId: string, chatId: string): Promise<State>
  Rename(kind: string, id: string, name: string): Promise<State>
  Delete(kind: string, id: string): Promise<State>
  Search(query: string): Promise<SearchResult[]>
  CloseDocument(): Promise<State>
  OpenDocument(id: string): Promise<State>
  OpenMuseGuide(): Promise<void>
}
declare global {
  interface Window {
    kikoWeb?: boolean
    runtime?: {
      EventsOnMultiple(
        name: string,
        callback: (message: ChatMessage) => void,
        count: number,
      ): () => void
    }
    go?: { main: { App: Backend } }
  }
}
export function desktop(): Backend {
  const api = window.go?.main.App
  if (!api && window.kikoWeb) return webBackend()
  if (!api) throw new Error('Open the desktop app to continue.')
  return api
}
export type Run = (operation: () => Promise<State>) => Promise<boolean>
