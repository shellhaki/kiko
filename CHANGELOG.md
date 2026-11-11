# Changes

## 0.1.0-dev — unreleased

- Desktop shell, nickname/Muse onboarding, secure key storage, projects, chats, search, and themes.
- Streaming conversations with local history/drafts, Stop, retry, and interrupted-run recovery.
- Rich editable documents, guarded AI document tools, activity history, anchored questions, revision restore, and Markdown export.
- Key replacement/removal and recoverable data reset.

This is a development build. Live Muse document quality, native export interaction, visual/accessibility review, hosted CI, Windows/Linux runtime checks, and release signing remain pending. No public release is claimed.

## Later-release features — implemented, unreleased

- v0.2 scope: opt-in Tavily research, static SVG diagrams, DOCX/Typst PDF exports, local image/PDF attachments, extraction/search, and offline PDF preview.
- v0.3 scope: provider adapters with isolated credentials (since reduced to Meta Muse only), spreadsheet cells/formulas/styles and XLSX/CSV, bounded Office extraction, and an optional Responses vision helper.
- v0.4 scope: editable slide decks, image/chart layouts, notes, PPTX and 16:9 PDF.
- Later personal-web scope: authenticated HTTP RPC/SSE/downloads, encrypted server-side keys, separate data directory, narrow layouts, and revision-guarded document suggestions.
- Diagrams are drawn and edited with Excalidraw: the editable scene is stored with each diagram and its static SVG feeds documents and exports. The agent's `create_diagram` tool now takes Excalidraw element skeletons that Go validates and renders to SVG, so agent diagrams are editable on the board. Older SVG-only diagrams stay editable as source or can be redrawn.
- Meta Muse is the only supported provider. Setup asks for one API key; Settings changes that key only. Each chat chooses its Muse model (Spark 1.3 and 1.2 in Contributor and Standard modes, Ember, Wonder) from the composer. OpenAI, Anthropic, OpenRouter, Groq, Gemini, Ollama, the vision-helper choice, and direct image input were removed; migration 012 moves existing chats to Muse.
- Recoverable sheet/deck drafts and explicit conflict recovery; sanitized diagrams remain in Markdown serialization and render in supported document exports.

These entries describe available development functionality, not published release versions. Original Office slide rendering and Files API reuse remain deferred; detailed limits are in docs/later-releases.md. No real inference/research requests were made during implementation. macOS packaging and local automated checks are verified; native/Office visual certification, real-service testing, Windows/Linux runtime checks, hosted CI, and release signing remain open.

### Rendering follow-up

- Sandboxed PDFium WASM page rasterization with real local PNGs, bounded child workers, no mounted host filesystem, and preserved component notices/provenance.
- (Removed November 11, 2026 with the other providers.) Explicit exact-model image opt-in for OpenAI/Anthropic; direct view_page/diagram images, correct multi-tool ordering, transient bounded image context, and no separate helper charge for direct inspections.
- Local image fallback/retry for browser PDF preview failures. Muse Contributor's text chat and helper behavior remain unchanged.
