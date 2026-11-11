# Kiko — Technical Spec

Implementation status: later-release functionality and its explicit adaptations/remaining limits are recorded in [later-releases.md](later-releases.md); [steps.txt](steps.txt) tracks verified work. The future-state contracts below are not a claim that every provider, rendering path, or public release has been certified.

Nov 8, 2026 · @Excel

## Current folder-workspace direction

The owner revised the product flow on November 9, 2026. [Folder workspaces](folder-workspaces.md) is the current contract for project roots, file tools, permissions, generated files, and motion. It supersedes conflicting virtual-project and splash decisions below. The selected folder is the project root; chats remain normal conversations and retain the side-by-side editor. The original release table below records the initial milestone scope rather than current feature availability.

## Scope

This spec covers **v0.1**: one user, one desktop machine, Muse as the only provider, and documents that the agent writes live and the user can edit. It's designed so that more providers, PDF/DOCX export and the mobile view slot in later without a rewrite.

| In v0.1 | Not in v0.1 |
| --- | --- |
| Desktop app (Wails) for macOS, Windows and Linux | Web version, mobile |
| Projects, each with chats and documents | Accounts, sync, collaboration |
| Chat with streaming replies | Voice, images, file uploads |
| Agent tools: create, update and read documents | Tavily web search plugin (v0.2), diagrams, sheets, slides |
| Side-by-side editable document pane | Review-gated suggestions (retired: edits apply directly) |
| Muse through the Meta Model API | Other providers (v0.3) |
| Markdown export | PDF and DOCX (v0.2) |
| Local storage in SQLite | Cloud backup |

## Stack

| Layer | Choice | Why |
| --- | --- | --- |
| App shell | Wails v2 (Go) | Go everywhere it matters, small native binaries, and the system web view for the UI |
| Core | A Go package, `kiko/core` | Agent loop, tools, block store, providers and export, written once and shared by desktop and web |
| Desktop adapter | Wails bindings + Wails events | Front end calls Go methods directly, and streaming arrives as events |
| Web adapter (later) | `net/http` + server-sent events | The same core served over HTTP for the web version |
| UI | React + TypeScript + Tailwind CSS v4 | One front end for desktop and web |
| Document editor | Tiptap (ProseMirror) | Block-based, editable, supports stable block ids and programmatic edits |
| Storage | SQLite through `modernc.org/sqlite` | Pure Go with no cgo, so cross-compiling stays simple |
| Secrets | OS keychain through `github.com/zalando/go-keyring` | Keys are never written to disk in plain text |
| Export | Markdown in Go; DOCX and PDF in v0.2 | Generated from the block store in the core |

**Desktop first, web next, mobile when wanted.** All logic lives in the Go core, and the front end only renders and sends user actions. A web version is then just a second adapter around the same core. A future mobile app can reuse the React UI and talk to a hosted core.

## Architecture

```text
React UI (system web view)
  ↕ Wails bindings and events
Go core: conversations, agent loop, document/file tools, revisions, export
  ├─ SQLite: local content, preferences, folder roots, permissions
  ├─ os.Root: confined project files and source copies
  └─ Muse provider → OS keychain + provider HTTPS endpoint
```

The desktop adapter owns native dialogs and window integration. The Go core
has no dependency on Wails; a later HTTP adapter can call the same core.

The front end only renders and sends user actions. Everything else lives in the Go core: the agent loop runs the tools against the block store, which persists to SQLite, and model calls go through the providers, which read keys from the OS keychain and stream from the Meta Model API. On desktop, the front end reaches the core through Wails bindings and events. The web version later puts an HTTP + SSE adapter around the same core.

## v0.1 implementation decisions

Recorded November 8, 2026 during build step 01. These are working implementation decisions filling gaps in the supplied layouts; the user's explicit requirements and the design system take precedence.

### Chats, projects, and document ownership

- **Standalone chats are supported.** `chats.project_id` is nullable. New chat from Home creates a standalone chat; New chat inside a project inherits that project. Standalone chats appear in the general chat list, with no invented project name.
- **Each document belongs to exactly one chat.** `documents.chat_id` is required. Its project is derived from the owning chat, so there is no second project assignment to fall out of sync. A chat may own multiple documents. Moving/sharing documents between chats is outside v0.1.
- **Restore context explicitly.** Opening a chat restores its messages, activity, artifact list, and last open document. Clicking an artifact opens that document; selecting a document from a project list opens its owning chat and document together. If the saved document is gone, show the chat with its document pane closed.
- **Projects use selected disk folders.** The desktop folder picker creates or reuses a project with a canonical root. Existing virtual projects are preserved for compatibility. See [folder workspaces](folder-workspaces.md) for file storage, permissions, and migration behavior. The optional visual file tree remains deferred.
- **Deletion has an explicit scope.** Deleting a document removes its blocks, revisions, comments, and nodes. Deleting a chat also removes its documents, messages, activity, and view state. Deleting a project also removes its chats and their content. Show the affected scope before a confirmed deletion and apply it transactionally. Version history restores edits to existing documents; it is not a trash bin for deleted projects/chats/documents.

### Document pane and toolbar

Use reference 08 for the base split and reference 09 for the quiet Saved status. Apply the design system's sans typography to both. The primary tab is **Document**, a rich, directly editable Tiptap surface. **Markdown** shows a read-only serialization of the same revision with Copy; it is not a second independently stored document or a source editor in v0.1. Flush pending edits before switching views or exporting; a save failure leaves the draft intact and blocks exporting stale content.

The toolbar contains the title, save status, Copy Markdown, Export, Undo, Version history, focus/restore pane, and Close. Less-used actions may live in a menu at narrow widths. Export offers Markdown only in v0.1. Focus expands the document in the current window and offers a clear return to chat; detached windows are deferred. Keep the selected tab, open document, and relevant scroll positions when switching context.

### Startup and onboarding

Startup is Splash → local state check → the next required screen. Show actual initialization progress or an indeterminate indicator; use no fake percentage. The owner requested a three-second minimum splash; continue showing it if initialization takes longer.

1. New users enter a nickname in the supplied centered layout. Trim whitespace; require 1–40 Unicode characters and allow ordinary international names. Save the nickname locally and allow Back from the next screen.
2. AI setup uses that same narrow centered form: “Connect Meta Muse”, a short explanation that the user supplies their own key, a labeled masked API-key field with Show/Hide, and **Connect and continue**. Meta Muse is the only provider choice. Add a verified official key-setup link when step 02 establishes the provider contract. Disclose that connection verification sends a provider request and may use credits.
3. During verification show **Connecting…** and prevent duplicate submissions. Keep the key in memory until it can be stored in the keychain. On success persist provider configuration and mark setup complete. Keep the nickname if setup is interrupted; never persist a key in frontend storage or SQLite to resume a draft.
4. Show invalid-key errors by the key field. For network/rate-limit errors preserve the in-session value and offer Retry. For a keychain error explain that secure storage failed; do not mark setup complete or silently fall back to plaintext. Going Back preserves the key only for the current in-memory session.
5. Returning users with completed setup enter the normal app without a network check on every launch. A missing key shows a reconnect state, preserving chats and documents. Offline users can still read and edit local documents; sending requires a usable connection.

Persist separate onboarding progress and completion so interruption resumes at the appropriate screen. A full data reset clears nickname and setup completion and returns to onboarding. Failure recovery must not wipe ordinary user content.

### Settings and recovery

Settings contains **Profile** (nickname), **AI** (Meta Muse connection, replace/remove key, and model setting), **Appearance** (Dark, Light, System), and **Data** (explicit full reset). The model setting follows the verified Muse contract from step 02; it does not expose unsupported providers. Dark is the initial preference. Theme tiles may follow reference 11's arrangement using Kiko's own palettes.

Replacing a key verifies the new key before replacing the old saved one. Removing a key disconnects chat sending without deleting local content. Full reset explicitly covers the local database, preferences, and Kiko keychain entries, requires confirmation, cancels active runs, and coordinates deletion through a recoverable reset state. Report partial cleanup and offer retry rather than claiming everything was erased. Tests exercise reset with disposable data only.

Required recovery screens/states include startup storage failure with Retry (no automatic reset), interrupted onboarding, reconnect Muse, empty and failed search, failed message/retry, stopped generation, unsaved document/retry, unavailable document, and canceled/failed export. Keep errors near their action and preserve drafts. Persist actual tool activity; a progress label must not fabricate private reasoning or a completed action.

### Open-source preparation

Kiko will be released as open source under **GPL-3.0-only**, selected by the project owner on November 8, 2026. Use the exact SPDX identifier `GPL-3.0-only`; this choice does not include an “or later” option. Add the corresponding LICENSE file in build step 03. Prepare contribution/security guidance, reproducible setup, dependency/asset notices, and the visual README specified in root `agents.md`. Check reference screenshots for redistribution rights and personal data before publishing them. Publication remains a separate action.

## Document model

Kiko's model copies the document-tool contract Claude uses in claude.ai, as seen from the agent's side. Anthropic's internal implementation isn't public, but the contract is what shapes the experience, and it's simple:

- **A document is a flat list of blocks.** Headings, paragraphs, lists, tables, code blocks, embeds. A "section" is just a heading block and the blocks under it. No separate section table.
- **Every block has a stable id and a hash.** The id never changes while the block lives. The hash `h` is 8 hex characters of the block's content, so the agent can prove it saw the latest version before replacing it.
- **The document has a revision number, `rev`.** It goes up on every change by anyone. Reads and write results all report the current `rev`.
- **Pending blocks are placeholders with an intent.** "Risks: what could slip and the fallback." They show the plan in place, and the agent later replaces each one with real content.
- **Embeds point at separate nodes.** A diagram is its own object (a small SVG module), and an embed block in the text shows it. Editing the diagram never touches the prose around it.
- **The agent writes Markdown, and the app returns an outline.** Every write's result (the "ack") lists the blocks it created, with their ids and hashes, in a compact form. The agent uses those ids for its next edit without reading the document again.

```json
{
  "id": "doc_8f2k", "title": "Heartme — Proposal", "rev": 14,
  "blocks": [
    { "id": "k3f9.0",  "h": "98b34f03", "type": "heading", "level": 1, "text": "Heartme — Proposal" },
    { "id": "k3f9.48", "h": "d2a23ca2", "type": "heading", "level": 2, "text": "The problem" },
    { "id": "k3f9.70", "h": "309bda2a", "type": "paragraph", "text": "The internet has two broken ways..." },
    { "id": "k3f9.39", "h": "541acb44", "type": "pending", "intent": "The product: pieces, hearts, subscriptions" },
    { "id": "k3f9.81", "h": "dfc26c9f", "type": "embed", "ref": "node/w_flywheel", "caption": "The flywheel" }
  ]
}
```

Block ids are `<session>.<counter>`: each editing session gets a random prefix and counts up. Ids from two writers never collide, which matters once edits come from both the user and the agent.

## Agent loop and tools

The loop is standard tool calling: send the conversation and tool definitions, run the tool calls the model returns, add the results, and repeat until the model answers in plain text. Cap it at 30 steps per turn.

The tools copy the shape of Claude's document tools: a few general verbs with typed operations inside, instead of many narrow tools.

| Tool | Arguments | What it does | Release |
| --- | --- | --- | --- |
| `create_document` | `name`, `markdown`, `blocks`, optional `path` | Creates the whole skeleton in one call: title, byline, and one pending block per section. The markdown carries `<?kiko block s1?>` tokens, and `blocks` maps each token to `{type: "pending", intent}`. Returns the doc id and an ack with each pending block's id. Optional `path` selects its root-relative `.md` source location | v0.1 |
| `update` | `doc_id`, `ops[]` | Every change goes through here. Ops are `replace`, `insert`, `delete` and `move`. A target is block ids, or `find` with exact quoted text (plus `nth` when it repeats). Content is `markdown` (re-creates blocks) or `text` (re-words in place and keeps bold, links and chips). Several ops run in order, all or nothing | v0.1 |
| `read` | `doc_id`, `payload` | Four projections: `outline` (each block's id, hash and first words), `view` + `parentId` (one table or list in full), `search` (text hits), and `sinceRev` (only what changed since a rev, with removed blocks marked gone) | v0.1 |
| `guide` | `topics[]` | Loads instruction pages on demand (editing rules, diagram rules), so the system prompt stays short | v0.1 |
| `create_diagram` | `doc_id`, `code`, `after_block` | Creates an SVG diagram node and its embed block in one call | v0.2 |
| `screenshot` | `node_id` | Renders a diagram to an image so the model can check its own picture before moving on | v0.2 |
| `comment` | `doc_id`, `anchor_text`, `body`, `suggested_replies` | Leaves a question anchored on a passage, with tap-to-answer replies | v0.1 |

**Refusals, not exceptions.** A tool that can't apply a change refuses with a code and the fix, and changes nothing: `missing_guard`, `guard_mismatch` (with the block as it stands now), `find_none`, `block_gone`. The model reads the refusal, adjusts and resends. Smaller models recover far better from a precise refusal than from a vague error.

**The ack.** Every write returns the new `rev` and a compact outline of what it wrote:

```xml
<doc op='0'>
  <paragraph id='k3f9.48' h='d2a23ca2' heading='2'>The problem</paragraph>
  <paragraph id='.70' h='309bda2a'>The intern…</paragraph>
  <list id='.198' h='165c83f8' kind='bullet'><gap blocks='3'/></list>
</doc>
```

Ids after the first are printed short (`.70` means `k3f9.70`), and text is cut to its first words. That keeps tool results small, which matters with paid tokens and smaller context windows.

**The rhythm** (system prompt rules):

1. **Skeleton first.** When asked for a document, the first tool call creates it with every section pending, before any research.
2. **One section per call.** Each `update` replaces one pending block, using the id from the creation ack. Never the whole document in one call.
3. **No reads between your own writes.** The ack already has the ids.
4. **Catch up at the start of a later turn.** If the doc was touched before, the first call is `read` with `sinceRev` set to the last rev seen, to pick up the user's edits.
5. **Chat stays short.** One line between calls about what you're working on, and one line at the end. The content goes in the document.
6. **Quote exactly, guard whole blocks.** `find` text must match the document word for word. A whole-block replace or delete carries `ifHash` and `ifRev`.

**On the live feel.** In claude.ai, a tool call shows nothing until it finishes, so the feeling of watching a document fill comes from the one-section-per-call rhythm. Kiko keeps that rhythm. If a provider streams tool-call arguments, Kiko can also render a section word by word, as an extra on top.

The current folder-workspace extension adds `list_files`, `read_file`, `write_file`, and `delete_file` under the selected root. Only deletion pauses for user approval. `delete_document` and `delete_artifact` remove documents, sheets, and decks with the same approval pause; linked workspace file records are unlinked while disk files are left in place. Document, sheet, and deck creation accepts a custom source path. The explorer, text drafts, source imports, and guarded external-version comparisons are described in [folder workspaces](folder-workspaces.md).

## Safe dual editing

The user and the agent edit the same document, and nothing is locked. Optimistic checks keep them from overwriting each other, and the rule is always that **the user's edit wins**.

1. **Guards on whole-block changes.** Replacing or deleting a whole block must carry `ifHash` (the block's hash as the agent last saw it) and `ifRev` (the rev of the read that showed it). If the user changed the block since, the call is refused with `guard_mismatch` and the block as it stands. Without guards, it's refused with `missing_guard`. Filling a pending block needs no guard.
2. **Find targets for small edits.** To change a phrase, the agent quotes the exact words. Everything around them stays untouched, including the user's edits in the same paragraph. If the quote no longer exists, it's refused with `find_none`, and the agent reads that block and tries again.
3. **The user's intent is respected.** If a pending block the agent was about to fill now holds the user's own text, the agent drops its write. If the user deleted a pending block (`block_gone`), that section isn't wanted, and the agent never re-creates it.
4. **Catch-up reads.** Each later turn starts with `read` + `sinceRev`, which returns only changed blocks and marks removed ones gone. The agent works from that, not from its memory of the document.
5. **Comments are protected.** An edit that would remove words a comment is anchored to is refused with `anchors_affected`, unless the user asked for that change.
6. **Direct edits (suggestion mode retired).** The agent's edits apply immediately; there is no proposal queue and no accept/reject step. The only ask path is the anchored comment question in chat.
7. **Nothing is ever lost.** Every rev is kept. The agent can `read` a past version with `atRev` and re-insert deleted text from it, never retyping from memory. The user can restore any earlier version from history.

## Provider layer

The agent loop talks to one interface. Each provider is just a config entry plus a format adapter.

```go
type Provider interface {
    ID() string                 // "muse", "openai", "anthropic", "openrouter"
    Stream(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error)
}

type ChatEvent struct {
    Type       string // "text" | "tool_call" | "usage" | "done" | "error"
    Delta      string // text delta, or tool-call argument delta
    ToolCallID string
    ToolName   string
    StopReason string // "end" | "tool_use" | "max_tokens" | "filtered" | "cancelled" | "error"
    RawFinishReason string // original provider finish reason
}

// Two implementations cover every provider:
//   openaiChat{baseURL, model}       -> Muse, OpenAI, OpenRouter, Groq, Gemini, Ollama
//   anthropicMessages{baseURL, model} -> Claude
```

**Muse in v0.1.** Use `POST https://api.meta.ai/v1/chat/completions` with Bearer authentication and the `openai-chat` adapter. The demo uses `muse-spark-1.3-contributor`, explicitly selected by the owner on November 9, 2026 to reduce cost. This build offers only that model. Contributor usage permits Meta to use submitted content/responses for training. See the [verified API contract](./muse-api-contract.md) for sources, request fields, error handling, and fixtures.

```json
{ "id": "muse", "format": "openai-chat", "baseUrl": "https://api.meta.ai/v1", "model": "muse-spark-1.3-contributor" }
```

Meta documents streamed function arguments. Assemble them by call index and validate only after a completed tool-call generation; do not write partially received JSON. Default to one section committed per completed tool call. Use `tool_choice: "auto"`, `parallel_tool_calls: false`, and `max_completion_tokens`. Keep usage separate from text and distinguish truncation, filtering, interruption, and successful completion.

The documentation check is complete; no live API call has been made. Current official pages disagree on some optional reasoning levels and key syntax, so the implementation treats keys as opaque and leaves regular reasoning effort unset. Use the owner-selected Contributor variant; do not fall back to Standard mode automatically. Stop cancels the Go request and local execution without promising immediate provider-side cancellation or billing termination.

**Adding providers later (v0.3).** OpenAI, OpenRouter, Groq, Gemini's OpenAI-compatible endpoint and Ollama all reuse `openai-chat` with a different base URL and model. Claude needs one new adapter for `anthropic-messages`. The agent loop, tools and UI don't change.

**Keys in Go.** The provider reads its key from the OS keychain at call time, makes the HTTPS request, and parses the server-sent events into `ChatEvent`s. The front end never sees a key. It only gets the agent's text and document updates through Wails events.

## UI

**Desktop (v0.1).**

- **Left sidebar:** projects, and the chats and documents in each one.
- **Chat pane:** messages stream in. When the agent creates or writes a document, a compact card appears in the chat ("Writing: The problem"). Clicking it focuses that document.
- **Document pane (right):** opens automatically when the agent creates a document. Shows the title, then each section: pending sections as a soft placeholder with their intent, writing sections with a live cursor, done sections fully editable. A toolbar holds Export, Undo, and a version history button.
- **Resizable split:** drag the divider, or collapse the document pane to a card.
- **Stop button:** cancels local generation and pending tool execution. Completed sections and partial chat text remain; uncommitted tool arguments are discarded.

**Web and mobile (later).** The web version reuses the same React UI, served by the Go core over HTTP. A mobile app can come whenever it's wanted: chat full screen, documents as cards that open full screen on tap, reusing the same components and talking to a hosted core.

**Settings (v0.1).** Edit the nickname, manage the Meta Muse key and supported model setting, choose Dark/Light/System, and explicitly reset local data. Multi-provider selection arrives later. See the implementation decisions above for connection and recovery behavior.

## Visual design

The [Kiko design system](./design-system.md) is the source of truth for interface styling, based on the supplied Codex desktop screenshot. It defines the neutral charcoal surfaces, restrained purple accents, system sans typography, rounded outline icons, spacing, component states, and pane behavior.

Future example screens guide layout; their colors, fonts, and icons are replaced with the Kiko design system. The system supersedes the earlier clay accent and serif document direction. Theme values live as semantic CSS variables mapped into Tailwind CSS v4.

## Questions in the document

When the agent isn't sure about something, it doesn't stop to ask in chat or quietly guess. It writes its best version, then leaves a short question **anchored on the exact passage** it's unsure about, with two or three answers the user can tap. This moves into v0.1, because it's a big part of what makes planning with the agent feel collaborative.

**How it works.**

1. After the last section is filled, the agent may call `comment` up to three times. Each call has `anchor_text` (exact words in the document), a one-line `body`, and `suggested_replies`.
2. The passage gets a soft highlight, and the question shows as a card in the margin on desktop (or under the paragraph on narrow screens), with each reply as a button.
3. Tapping a reply posts it in the thread and sends it to the agent as a new turn, along with the thread id and the anchored passage.
4. The agent makes the edit through `update` with `answering: <thread id>`, then posts a one-line reply in the same thread saying what changed. If the reply was "keep it", it changes nothing and says so.
5. The user can always type their own answer instead of tapping.

**Rules for the agent** (in the system prompt):

- **Only ask where the answer changes the draft.** No questions just to reach three, and none on short or throwaway documents.
- **One thing per question.** Never two questions joined together.
- **Ask in this order:** first a call that could fairly have gone another way (audience, angle, what leads), then a guess or a gap (an assumption, a placeholder number).
- **Replies are real answers.** Two to five words each, leading to really different results. When the question is whether to change something, one reply keeps it as is ("Keep developers first").
- **Never ask what the request already answered.**
- **Hand off honestly.** The final chat line says questions were left in the document, without repeating them.

**Data.** Questions use the `comments` table: `anchor_block`, `anchor_start` and `anchor_end` pin the passage, `suggested_replies` holds the buttons, and `parent_id` links replies into a thread. An edit that would delete the anchored words is refused with `anchors_affected` unless the user asked for it.

## The agent's role

Kiko is a normal conversational assistant whose speciality is documents. Its system prompt tells it both things:

- **It talks like any good assistant.** A quick question gets a quick answer in chat, with no tool calls. Brainstorming, opinions and explanations happen in conversation.
- **It knows its job is documents.** When the user asks for something they'll keep, edit or share (a proposal, spec, plan, report, deck or spreadsheet), it makes an artifact without asking whether to. Asking for "a proposal" means a document, not a long chat message.
- **Tools only when they earn it.** It searches the web for facts that change over time (prices, versions, who leads what, competitors), not for things it already knows well. It reads a document before editing only when the user may have changed it.
- **Artifacts are how it delivers documents.** The chat carries one line about what it's doing. The content lives in the artifact.
- **It decides the output type.** A doc for writing, a sheet for tables to sort and calculate, slides for presenting. If the request could fairly be two types, it asks once.

The system prompt is short. The detailed rules for each artifact type and plugin live in skills that load only when needed (see below), so small models aren't overloaded with instructions they don't need yet.

## Plugins

A plugin adds tools the agent can use, backed by a service the user connects with their own key. **The agent only sees a plugin's tools when it's connected.** If the user hasn't added a key, the tools aren't in the request at all, and the agent plans from what it knows and says so when a fact may be out of date.

**First plugin: Tavily web search.** Tavily is a search API built for AI agents. Kiko calls it directly from the Go core with the user's key, from the keychain like model keys:

| Tool | Tavily endpoint | What the agent uses it for |
| --- | --- | --- |
| `web_search` | `POST https://api.tavily.com/search` | Finding current facts, competitors, prices and docs. Returns ranked results with titles, URLs and content snippets |
| `web_fetch` | `POST https://api.tavily.com/extract` | Reading a page in full as clean Markdown before taking numbers or quotes from it |

Basic search uses 1 Tavily credit per request, and basic extract uses 1 credit per 5 URLs, according to [Tavily's integrations documentation](https://docs.nexla.com/user-guides/connectors/tavily_api). The plugin settings show the user roughly how many calls each session made.

**Research rules** (in the web-search skill, loaded when the plugin is on):

- Search before stating anything about the world as it is now. Don't search for stable knowledge.
- Read a page with `web_fetch` before taking numbers from it, because a snippet isn't a source.
- Cite sources in the document as links, with a Sources list at the end.
- Keep searches short and specific, and run independent searches in parallel.

**The plugin shape.** Each plugin is a Go package implementing one interface, plus a skill file:

```go
type Plugin interface {
    ID() string                       // "tavily"
    Name() string                     // "Web search (Tavily)"
    Configured() bool                 // has a key in the keychain
    Tools() []ToolDef                 // added to the request only when Configured()
    Call(ctx context.Context, tool string, args json.RawMessage) (json.RawMessage, error)
    Skill() string                    // SKILL.md text, loaded through guide()
}
```

Later plugins (Google Drive, Notion, GitHub, or any MCP server) follow the same shape. Supporting MCP servers as plugins would open up a whole ecosystem without writing each integration by hand.

## Artifact types and document skills

Artifacts are how Kiko delivers documents, and there are three types. Each has its own editor, its own tools and its own skill. Each one exports to the file formats people need, so a real file is always one click away.

| Artifact | What it holds | Agent tools | Exports to | Release |
| --- | --- | --- | --- | --- |
| **Doc** | Blocks: headings, text, lists, tables, code, diagrams | `create_document`, `update`, `read` | Markdown, DOCX, PDF | v0.1 (Markdown), v0.2 (DOCX, PDF) |
| **Sheet** | Tabs of cells with values, formulas and formatting | `create_sheet`, `set_cells` (a range of values or formulas), `read_range` | XLSX, CSV | v0.3 |
| **Slides** | Slides, each with a layout (title, bullets, two columns, image, chart) and blocks | `create_deck`, `update_slide`, `read_deck` | PPTX, PDF | v0.4 |

The agent can also make a plain file when the user names a format ("give me an Excel file"). It uses `export_file` on an artifact, and the file appears as a download card in the chat.

**How each format is built, all in the Go core:**

- **XLSX:** [excelize](https://github.com/xuri/excelize), a mature pure-Go library. Its built-in formula calculation also powers live formula results in the Sheet editor, so the same engine computes in the app and in the exported file.
- **PDF:** [Typst](https://github.com/typst/typst) bundled as a small binary. The core turns a doc or deck into Typst markup and compiles it. That gives proper typography, tables, page numbers, headers and a cover page, well beyond printing HTML.
- **DOCX:** a small hand-written OOXML generator (Go templates inside a zip). The free Go libraries for this format are unmaintained or commercially licensed, and the structures Kiko needs are limited.
- **PPTX:** [PptxGenJS](https://github.com/gitbrent/PptxGenJS) running in the editor: the slide preview and the exported file share one theme and geometry, text stays editable text and charts stay native charts. The desktop app writes the bytes through the native save dialog; the web version downloads them from the browser. PDF still compiles from the Go core through Typst.
- **Markdown and CSV:** plain Go.

**Skills.** Each artifact type, export format and plugin ships a `SKILL.md`: short instructions on doing that job well (how to structure a spec, how to build a clean financial sheet, how to lay out a slide). They follow the open Agent Skills format: a name and description the agent always sees, and a body loaded only through `guide()` when the task needs it. Users can drop community skills into a skills folder, and the agent finds them the same way.

| Skill | Loaded when |
| --- | --- |
| `docs` | Making or editing any doc |
| `sheets` | Making or editing a sheet |
| `slides` | Making or editing a deck |
| `pdf-export` | Exporting a polished PDF (cover, contents, page styles) |
| `research` | The Tavily plugin is on and the task needs current facts |
| `questions` | Deciding whether to leave questions in an artifact |

## Showing tool calls in the thought process

Every tool call is visible, in the flow of the reply, the way Claude shows its work. The user can always see what the agent did and why, without it taking over the screen.

**What the user sees in each reply:**

1. **Working status.** Show a neutral waiting state before output and actual tool activity as it arrives. Private reasoning is not a visible progress feed; v0.1 does not depend on a reasoning-text field.
2. **Steps.** Each tool call is one line with an icon and a plain-language label, shown live as it happens, with a spinner on the current step:
   - Searched the web: *creator platforms Nigeria*, with the result count and source links
   - Read a page: *ko-fi.com/help*
   - Loaded skill: *docs*
   - Created doc: *Heartme — Proposal*
   - Wrote section: *The problem*
   - Edited section: *Business model*
   - Left a question on: *"We start with developers."*
3. **Text.** The agent's short chat lines sit between the steps they relate to, in order.
4. **Summary.** When the reply finishes, the steps fold into one line, such as "Used 9 tools · 41 s". One click expands it again.

**Details on demand.** Clicking a step shows its arguments and result, formatted for reading. A developer toggle in settings shows the raw JSON. A refused call shows quietly with its reason ("Section changed, re-read it"), so the user can see the agent recovering instead of failing silently.

**How it's wired.** The Go core emits one event stream per turn over Wails events (and later SSE on the web):

```go
type TurnEvent struct {
    Type     string // "status" | "text_delta" | "step_started" | "step_finished" | "artifact_changed" | "done"
    StepID   string
    Tool     string // "web_search", "update", ...
    Label    string // "Wrote section: The problem", built from the args
    Detail   json.RawMessage // args and result summary, for the expanded view
    Status   string // "ok" | "refused" | "error"
    Delta    string
    Elapsed  int64  // ms
}
```

Each tool has a small `Label(args)` function that writes its human sentence, so labels stay consistent. Steps are saved with the message, so reopening an old chat shows exactly what happened.

## Understanding images and documents

Users can attach files to a chat (screenshots, PDFs, Word documents, spreadsheets, decks), and the agent uses them as source material. Kiko uses the model's own vision where it exists and fills every gap with tools, so any model can work with any file.

**What Muse handles natively.** According to [Meta's model docs](https://dev.meta.ai/docs/models), Muse Spark takes text, images, video, audio and PDFs as input, with a 1,048,576-token context. The Model API also has a Files API: upload a file once and reference it by id in later requests. File/PDF request compatibility must be rechecked during v0.2; the older assumption that native PDF input requires switching formats is not a verified constraint. v0.1 sends text through Chat Completions; a Responses adapter is a separate later decision.

**What no model reads natively:** Word (.docx), Excel (.xlsx), PowerPoint (.pptx), CSV and Markdown files. Kiko converts these itself.

**The attachment pipeline** (in the Go core, on attach):

| File type | What Kiko does | Sent to a vision model as |
| --- | --- | --- |
| Image (PNG, JPG, WebP) | Stores it, makes a thumbnail | The image itself |
| PDF | Extracts text per page with a pure-Go PDF reader; renders page images on demand with pdf.js in the web view | Native PDF (Muse), or page images |
| DOCX | Parses the document XML into Markdown: headings, lists, tables | Text |
| XLSX, CSV | Reads every sheet with excelize into Markdown tables, keeping formulas next to values | Text |
| PPTX | Parses slide XML into text per slide; renders slide images on demand | Text, plus slide images |
| MD, TXT, code | Read as is | Text |

Extracted text is indexed in SQLite full-text search (FTS5), so large files can be searched instead of loaded whole.

**Tools for the agent** (always available when a chat has attachments):

| Tool | What it does |
| --- | --- |
| `list_attachments` | Names, types, sizes and page or sheet counts for this chat's files |
| `read_attachment` | Extracted text of a file, or of chosen pages, sheets or slides, as Markdown with page markers |
| `search_attachments` | Finds passages matching a query across all attachments, with file and page for each hit |
| `view_page` | Returns one PDF page or slide as an image, for charts, diagrams and layouts that text misses. Only offered to vision models |
| `describe_image` | Sends an image to a vision-capable helper model with a question and returns its answer as text. This lets text-only models "see" |

**Rules for the agent** (in an `attachments` skill):

- Small files (under about 20 pages or 30,000 tokens) go straight into context. Larger ones are read through `search_attachments` and `read_attachment`, page by page.
- Use `view_page` or `describe_image` when the meaning is visual: charts, diagrams, screenshots, scanned pages with no text layer.
- When a document uses facts from an attachment, cite it inline, like "(pitch.pdf, p. 3)".

**Helper model.** In settings, the user can choose a vision helper for `describe_image`. Muse is the default, since it's multimodal. Meta also offers a cheaper **Contributor** tier of Muse Spark, but requests on that tier may be used to train Meta's models. The settings screen says so plainly, so users choose knowingly.

**Storage.** One more table:

```sql
CREATE TABLE attachments (
  id TEXT PRIMARY KEY, chat_id TEXT NOT NULL REFERENCES chats(id),
  name TEXT NOT NULL, mime TEXT NOT NULL, size INTEGER NOT NULL,
  path TEXT NOT NULL,              -- copy kept in the app's data folder
  pages INTEGER,                   -- pages, slides or sheets
  extracted TEXT,                  -- Markdown, with page markers
  provider_file_id TEXT,           -- Muse Files API id, when uploaded
  created_at INTEGER NOT NULL
);
CREATE VIRTUAL TABLE attachment_fts USING fts5(attachment_id, page, text);
```

**Release:** images and PDFs in v0.2, and Office files and the helper model in v0.3.

## Storage and data schema

Local content and preferences live in one SQLite file; API keys live separately in the OS keychain. The document side is built around two ideas from the block model: blocks carry the revs they were born and changed in, and every write is logged as one revision. Together these make `sinceRev` reads, `atRev` history and restore cheap.

```sql
CREATE TABLE projects (
  id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT,
  created_at INTEGER NOT NULL
);

CREATE TABLE chats (
  id TEXT PRIMARY KEY, project_id TEXT REFERENCES projects(id) ON DELETE CASCADE,
  title TEXT, provider_id TEXT NOT NULL REFERENCES providers(id),
  created_at INTEGER NOT NULL
);

CREATE TABLE messages (
  id TEXT PRIMARY KEY, chat_id TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
  role TEXT NOT NULL,            -- user | assistant | tool
  content TEXT, tool_calls TEXT, -- JSON
  created_at INTEGER NOT NULL
);

CREATE TABLE documents (
  id TEXT PRIMARY KEY, chat_id TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
  title TEXT NOT NULL, rev INTEGER NOT NULL DEFAULT 0, updated_at INTEGER NOT NULL
);

CREATE TABLE blocks (
  id TEXT NOT NULL,               -- '<session>.<counter>'
  document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  parent_id TEXT,                 -- list items, table rows and cells nest
  position REAL NOT NULL,         -- fractional ordering among siblings
  type TEXT NOT NULL,             -- heading | paragraph | list | listItem | table | row | cell | code | embed | pending
  attrs TEXT,                     -- JSON: level, kind, language, intent, ref, caption
  content TEXT,                   -- JSON inline runs: text, bold, link, code, chips
  h TEXT NOT NULL,                -- 8-hex hash of content + attrs
  born_rev INTEGER NOT NULL, changed_rev INTEGER NOT NULL,
  gone_rev INTEGER,               -- set when deleted; row kept for history
  PRIMARY KEY (document_id, id)
);

CREATE TABLE revisions (
  document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  rev INTEGER NOT NULL,
  author TEXT NOT NULL,           -- user | agent
  ops TEXT NOT NULL,              -- JSON: the ops applied, with before-images
  created_at INTEGER NOT NULL,
  PRIMARY KEY (document_id, rev)
);

CREATE TABLE nodes (              -- diagrams (v0.2)
  id TEXT PRIMARY KEY, document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  kind TEXT NOT NULL,             -- widget
  code TEXT NOT NULL, draft TEXT, pub INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE comments (           -- questions in the document, v0.1
  id TEXT PRIMARY KEY, document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  parent_id TEXT,                 -- thread root
  anchor_block TEXT, anchor_start INTEGER, anchor_end INTEGER,
  body TEXT NOT NULL, suggested_replies TEXT, author TEXT NOT NULL,
  resolved INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL
);

CREATE TABLE app_settings (
  key TEXT PRIMARY KEY, value TEXT NOT NULL -- JSON preferences; never API keys
);

CREATE TABLE chat_view_state (
  chat_id TEXT PRIMARY KEY REFERENCES chats(id) ON DELETE CASCADE,
  document_id TEXT REFERENCES documents(id) ON DELETE SET NULL,
  state TEXT NOT NULL -- JSON: selected tab, pane widths, scroll positions
);

CREATE TABLE tool_steps (
  id TEXT PRIMARY KEY,
  message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  tool TEXT NOT NULL, label TEXT NOT NULL,
  status TEXT NOT NULL, detail TEXT, elapsed_ms INTEGER,
  created_at INTEGER NOT NULL
);

CREATE TABLE providers (
  id TEXT PRIMARY KEY, format TEXT NOT NULL, base_url TEXT NOT NULL,
  model TEXT NOT NULL, is_default INTEGER NOT NULL DEFAULT 0
  -- the API key is NOT here: it lives in the OS keychain under this id
);
```

The executable pre-release schema is [001_initial.sql](../core/storage/migrations/001_initial.sql), with connection/migration and typed preference behavior documented in [storage.md](./storage.md). Its constraints and indexes strengthen the illustrative schema above. It also adds a single-row `workspace_state` table for the selected project/chat, seeds Muse configuration once, stores global preferences under the `preferences` key, and uses the onboarding `stage` (`nickname`, `muse`, `done`) as both progress and completion. Comments use integer passage offsets. The nodes table reserves future storage and does not implement diagrams in v0.1.

The app enables SQLite foreign keys on every connection. Core operations enforce that a restored `chat_view_state.document_id` belongs to that same chat. `app_settings` stores nickname, onboarding progress/completion, theme, global pane state, and any reset-in-progress marker. Tool step details are sanitized before persistence. Removing the saved API key retains the Muse provider configuration row so existing chats keep their provider reference.

Project documents are queried through `documents JOIN chats ON documents.chat_id = chats.id`, filtered by `chats.project_id`. Blocks, revisions, comments, and nodes remain owned by their document. All delete paths apply the explicit scope described above; no file-tree schema is introduced in v0.1.

- **`sinceRev` read:** blocks where `changed_rev > :rev`, plus ids where `gone_rev > :rev`, with unchanged runs folded into `<gap blocks='n'/>`.
- **`atRev` read:** blocks where `born_rev <= :rev` and (`gone_rev` is null or `> :rev`), with content rebuilt from the before-images in `revisions`.
- **User typing** is saved after 800 ms of no input, as one revision with `author = 'user'`.

## Export

All exports start from the same source: the document's sections joined as Markdown, with the title as a top-level heading.

| Format | How | Release |
| --- | --- | --- |
| Markdown | Join the doc's blocks as Markdown and save with the native save dialog | v0.1 |
| PDF | Convert the doc to Typst markup and compile it with the bundled Typst binary | v0.2 |
| DOCX | Hand-written OOXML generator: Go templates inside a zip | v0.2 |
| XLSX, CSV | excelize, from the Sheet artifact | v0.3 |
| PPTX | PptxGenJS, from the Slides artifact preview (desktop: native save dialog, web: browser download) | v0.4 |

All export code lives in the Go core except slides, which generate in the editor so the preview and the file match. See Artifact types and document skills for why each library was chosen.

## Security and keys

- **Keys stay in the OS keychain.** They're stored under the provider id with `go-keyring`, read only by the Go provider, and sent only to that provider's base URL.
- **Saved keys never return to the front end.** The setup form holds only the key currently being typed and clears it after success. Go verifies and stores it; Wails binds no saved-key read method. It is never stored in browser storage or SQLite.
- **No telemetry by default.** The app makes no network calls except to the providers the user configured.
- **Model output is treated as data.** Agent-written Markdown goes through the block parser and the editor's schema, so raw HTML and scripts from a model can't run in the app.
- **Web version keys (later).** A hosted web version can't use the OS keychain. Self-hosters keep keys in server config, encrypted at rest. That design comes with the web adapter.
- **Open source means auditable.** The key-handling path is one short Go file that anyone can check.

## Build milestones for v0.1

Built in this order, so each step is testable on its own:

- [x] **1. Shell and storage.** A Wails v2 app with the React template, `kiko/core` as its own package, the SQLite schema, and projects and chats you can create and list.
- [ ] **2. Muse chat.** Keychain storage with `go-keyring`, the `openaiChat` provider, and a streaming plain chat with Muse over Wails events. No tools yet.
- [ ] **3. Block store.** Blocks with ids, hashes and revs, the revision log, and the Markdown-to-blocks parser in Go. Unit-test `sinceRev` and `atRev` reads here, before any UI.
- [ ] **4. Document pane.** A Tiptap editor bound to the block store: pending blocks show their intent, and user typing saves as revisions.
- [ ] **5. Agent tools.** `create_document`, `update` (replace, insert, delete, move, with find targets and guards), `read` (outline, view, search, sinceRev), and `guide`. Every refusal code returns its fix. Every call shows as a step in the reply.
- [ ] **6. The rhythm.** The system prompt rules: skeleton first, one section per call, acks instead of reads, catch-up on later turns. Test it with Muse on three real documents.
- [ ] **7. Export and polish.** Questions in the document with tap-to-answer replies, Markdown export, the stop button, chat cards, the resizable split, and the dark theme.
- [ ] **8. Dogfood.** Write the next real project proposal and spec in Kiko, and fix what breaks.

## Open questions

- **Live Muse validation.** The documented model ID, rate limits, and streamed arguments are recorded in the [API contract](./muse-api-contract.md). Account-specific access, actual streamed calls, output budgets, cancellation behavior, and model quality still need live checks in the implementation steps.
- **Typst size and licence.** Typst is Apache-2.0 licensed. How much does bundling it add to the installer on each platform, and is that acceptable?
- **Web version auth and keys.** Single-user self-hosted first, or multi-user from the start?
- **Diagrams.** Should v0.2 diagrams be SVG modules like Claude's, or Mermaid blocks, which are simpler to generate?

---

**Sources**

- [Meta API reference](https://dev.meta.ai/docs/api-reference)
- [Meta models](https://dev.meta.ai/docs/models)
- [Meta tool calling](https://dev.meta.ai/docs/tool-calling)
- [Meta pricing and rate limits](https://dev.meta.ai/docs/pricing-rate-limits)
- [Kiko API contract and source notes](./muse-api-contract.md)
