# Editable documents

Implemented November 9, 2026. Product behavior follows the technical spec and styling follows the design system. Native visual review and live Muse dogfooding remain pending.

## Content and revisions

`core/document` parses supported Markdown with Goldmark 1.7.13: headings, paragraphs, nested bullet/ordered/task lists, tables, fenced/indented code, bold, italic, strike, code spans, and safe links. Raw HTML, images, embeds, and unsupported block types are refused. Pending tokens map to explicit intent-bearing blocks. Serialization preserves supported structure and inline formatting; it normalizes Markdown syntax rather than preserving the original byte layout. See [Goldmark](https://github.com/yuin/goldmark/tree/v1.7.13).

Every block has a stable random-session identifier and an eight-character SHA-256 content/attribute hash. Parent hashes incorporate child content without incorporating identity. SQLite stores ordered nested rows. Revisions store full before/after snapshots, which makes historical reconstruction independent of later moves/deletions. These snapshots trade storage space for a simple, auditable recovery path. Document size, nesting, block count, and revision size are bounded.

`ReadDocument` returns current content, outline, Markdown, saved view state, and any recovered user draft. Read options include a nested parent, search, atRev, and sinceRev with changed and gone blocks. Writes and revision increments are atomic. Tests cover nested round trips, stable IDs, formatting, order, historical reads, rollback, migration, reopen, and competing writers.

## Editing and concurrency

The Tiptap pane opens beside chat. It offers direct editing, Markdown view, copy, native Markdown export, undo, history/restore, focus, resize, and close. The editor loads separately from the initial application bundle. The implementation uses the community [React editor](https://tiptap.dev/docs/editor/getting-started/install/react) and [UniqueID extension](https://tiptap.dev/docs/editor/extensions/functionality/uniqueid).

Edits immediately queue a SQLite draft, then commit after a 600 ms typing debounce. Navigation, tab changes, copy, and export flush pending commits. A failed save keeps the editor draft and blocks navigation; the user can retry, copy text directly, or explicitly discard the draft and reload. Startup can recover a draft with its original base revision. Native force-quit can still interrupt an in-flight draft write; no application can promise that every last unsaved keystroke reached disk.

User saves compare the base revision. An incoming agent edit therefore cannot silently replace a dirty local editor. Clean panes poll saved content and preserve selection by stable block ID and local offset where possible. Pane view/width/scroll preferences persist. Editing a pending block converts it to ordinary user text, which removes its guard-free replacement privilege.

Agent operations are replace, insert, delete, and move. Whole-block changes require current revision/hash guards, except replacing an unchanged pending block. Exact find/text edits preserve surrounding inline marks, require explicit match selection when ambiguous, and refuse absent text. Ordered operations are all-or-nothing. Structured refusals include missing_guard, guard_mismatch, find_none, find_ambiguous, block_gone, and anchors_affected.

## Tools and questions

The provider assembles complete tool argument deltas before executing anything. Truncated, malformed, stopped, or incomplete calls do not execute. Document tools are constrained to the active chat. A selected folder enables confined file tools with explicit write permissions; optional configured research tools have their own bounded web access. See [folder workspaces](folder-workspaces.md). `guide` supplies editing rules on demand. Creation instructions require a skeleton followed by one section per update, and later turns receive saved tool results plus document context and instructions to read current content before writing. The loop has a 30-call-round cap.

Actual tool activity is stored in `tool_steps`; status changes publish full durable message snapshots. Only labels/statuses are sent in chat events, keeping full saved arguments/results out of every streaming update. Successful tool context is replayed on later turns. Completed writes remain durable after Stop or a later tool refusal.

The comment tool anchors a question to a unique exact passage and stores rune offsets, its original quote, and suggested replies. The editor highlights the passage and accepts suggested or typed answers in the owning chat. Safe edits move the anchor; agent edits that remove its words are refused. A user may edit/remove the passage directly: the question keeps its original quote and is marked as detached rather than discarded. Resolving or answering releases its protection.

## History, export, and reset

Restore creates a new user revision; earlier versions remain available. Markdown export flushes the editor, refuses an outstanding draft, opens the native save dialog, and writes a temporary file before replacing the selected destination. Canceling the dialog writes nothing. Filesystem export and round-trip tests pass; the native save/open interaction still requires manual verification.

Key removal cancels active work and preserves documents. Full reset requires typing RESET in a scope-specific dialog. It cancels work, persists a recovery marker, removes the key, and atomically clears local content/preferences. Failed cleanup retains the marker and exposes an explicit retry on startup. Reset tests use disposable databases and fake keychains, never the real workspace.

## Linked workspace sources

A document can have an explicit `.md` source path under its selected project root. Existing Markdown files can be imported as linked editable documents without rewriting their originals. Source status detects external changes and offers guarded comparison/reload or an explicit save of the Kiko version. Reload creates a revision and retains ordinary draft/revision/anchor checks. See [folder workspaces](folder-workspaces.md) for file handling and limits.

## Diagrams

Diagrams are drawn with [Excalidraw](https://github.com/excalidraw/excalidraw) 0.18.1, loaded as a separate lazy chunk. **Draw diagram** inserts a new figure; **Edit diagram** reopens a saved one. A diagram block stores two attributes: `scene`, the editable Excalidraw JSON (type `excalidraw`, 1–2000 elements, at most 512 KB, validated in `core/document/scene.go`), and `svg`, a static export of that scene. Documents, Markdown, PDF, and DOCX use only the SVG, so exports never depend on the editor.

The export keeps Excalidraw's hand-drawn paths on a white background. Before saving, the frontend strips `style`, `metadata`, empty `mask`, and the `style`, `direction`, `version`, and `xmlns` attributes (`frontend/src/diagram-scene.ts`), and the Go sanitizer still validates the result against its static allowlist. A fixture of a real export guards that contract. The image tool is hidden because the sanitizer allows no embedded images.

Fonts load from `/excalidraw/fonts`, copied from the package by `scripts/copy-excalidraw-assets.mjs` during `npm run assets`, so the board works offline. The CJK fallback font is omitted to save 12 MB. Exported SVG names `Excalifont`, which the PDF renderer lacks, so PDF text falls back to a system font until the font is bundled with the Typst runtime.

### Agent diagrams

The `create_diagram` tool takes Excalidraw element skeletons instead of SVG: `rectangle`, `ellipse`, and `diamond` (x, y, width, height, optional `label`), `text`, and `arrow`/`line`. Arrows connect shapes by `start`/`end` id or take an x/y plus width/height offset. Unknown element types and extra fields are refused with a specific message. `core/document/diagram.go` validates the elements (1–200, bounded coordinates, `#hex` colors, `solid`/`dashed`/`dotted`), resolves each bound arrow to edge-to-edge geometry, and renders a clean SVG itself. The block therefore has its SVG immediately, with no browser involved, so the agent's `screenshot` check, PDF/DOCX export, and folder Markdown copies work while no editor is open.

The stored `scene` is the normalized skeleton with `"skeleton": true`. When a user opens **Edit diagram**, the board converts it with Excalidraw's `convertToExcalidrawElements` using `roughness: 0` and the Helvetica font, so the board matches the Go render. **Apply diagram** saves a full scene (no `skeleton` flag) and a fresh Excalidraw SVG. Go's SVG text sizing is approximate: it centers labels but does not measure fonts, so agents should keep labels short and leave room around them.

Diagrams made before this change are plain SVG without a `scene`. They offer **Edit SVG source** or **Redraw in Excalidraw**, which starts a blank board and replaces the drawing only on **Apply diagram**.
