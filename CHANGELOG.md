# Changes

## 0.1.0-dev — unreleased

- Desktop shell, nickname/Muse onboarding, secure key storage, projects, chats, search, and themes.
- Streaming conversations with local history/drafts, Stop, retry, and interrupted-run recovery.
- Rich editable documents, guarded AI document tools, activity history, anchored questions, revision restore, and Markdown export.
- Key replacement/removal and recoverable data reset.

This is a development build. Live Muse document quality, native export interaction, visual/accessibility review, hosted CI, Windows/Linux runtime checks, and release signing remain pending. No public release is claimed.

## Later-release features — implemented, unreleased

- v0.2 scope: opt-in Tavily research, static SVG diagrams, DOCX/Typst PDF exports, local image/PDF attachments, extraction/search, and offline PDF preview.
- v0.3 scope: explicitly configured provider adapters with isolated credentials, spreadsheet cells/formulas/styles and XLSX/CSV, bounded Office extraction, and an optional Responses vision helper.
- v0.4 scope: editable slide decks, image/chart layouts, notes, PPTX and 16:9 PDF.
- Later personal-web scope: authenticated HTTP RPC/SSE/downloads, encrypted server-side keys, separate data directory, narrow layouts, and revision-guarded document suggestions.
- Recoverable sheet/deck drafts and explicit conflict recovery; sanitized diagrams remain in Markdown serialization and render in supported document exports.

These entries describe available development functionality, not published release versions. Original Office slide rendering and Files API reuse remain deferred; detailed limits are in docs/later-releases.md. No real inference/research requests were made during implementation. macOS packaging and local automated checks are verified; native/Office visual certification, real-service testing, Windows/Linux runtime checks, hosted CI, and release signing remain open.

### Rendering follow-up

- Sandboxed PDFium WASM page rasterization with real local PNGs, bounded child workers, no mounted host filesystem, and preserved component notices/provenance.
- Explicit exact-model image opt-in for OpenAI/Anthropic; direct view_page/diagram images, correct multi-tool ordering, transient bounded image context, and no separate helper charge for direct inspections.
- Local image fallback/retry for browser PDF preview failures. Muse Contributor's text chat and helper behavior remain unchanged.
