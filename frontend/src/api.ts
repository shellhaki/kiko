import { webBackend } from './web-api'
import type { ArtifactRecord } from './artifact-model'
import type { Block, DocumentRead, DocumentView, SavedDocument } from './document-model'
export type ChatMessage = {
  steps?: { id: string; tool: string; label: string; status: string; detail?: unknown }[]
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
export type Preferences = {
  imageModels?: Record<string, string>
  suggestionMode?: boolean
  providerId?: string
  visionProviderId?: string
  tavilyEnabled?: boolean
  nickname: string
  stage: 'nickname' | 'muse' | 'done'
  theme: 'dark' | 'light' | 'system'
  sidebarOpen: boolean
  sidebarWidth: number
  resetInProgress: boolean
}
export type Project = { id: string; name: string; description: string }
export type Chat = { id: string; projectId: string; title: string }
export type Document = { id: string; chatId: string; projectId: string; title: string }
export type State = {
  provider?: { id: string; format: string; baseURL: string; model: string }
  providers?: { id: string; format: string; baseURL: string; model: string }[]
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
export type Backend = {
  SetDirectImages(id: string, enabled: boolean): Promise<State>
  RenderAttachmentPage(chat: string, id: string, page: number): Promise<string>
  SetSuggestionMode(enabled: boolean): Promise<State>
  AcceptSuggestion(id: string): Promise<SavedDocument>
  RejectSuggestion(id: string): Promise<void>
  CreateDeck(chat: string, title: string): Promise<State>
  ConfigureProvider(id: string, model: string, key: string, endpoint: string): Promise<State>
  SelectProvider(id: string): Promise<State>
  SetVisionProvider(id: string): Promise<State>
  RemoveProviderKey(id: string): Promise<State>
  ReadArtifact(id: string): Promise<ArtifactRecord>
  SaveArtifact(id: string, rev: number, payload: unknown): Promise<ArtifactRecord>
  DiscardArtifactDraft(id: string): Promise<void>
  SaveArtifactDraft(id: string, rev: number, payload: unknown): Promise<void>
  OpenArtifact(id: string): Promise<State>
  CloseArtifact(): Promise<State>
  CreateSheet(chat: string, title: string): Promise<State>
  ExportArtifact(id: string, format: string, tab: string): Promise<boolean>
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
