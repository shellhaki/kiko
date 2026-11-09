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

The provider assembles complete tool argument deltas before executing anything. Truncated, malformed, stopped, or incomplete calls do not execute. Tools are constrained to the active chat, with no filesystem or web access. `guide` supplies editing rules on demand. Creation instructions require a skeleton followed by one section per update, and later turns receive saved tool results plus document context and instructions to read current content before writing. The loop has a 30-call-round cap.

Actual tool activity is stored in `tool_steps`; status changes publish full durable message snapshots. Only labels/statuses are sent in chat events, keeping full saved arguments/results out of every streaming update. Successful tool context is replayed on later turns. Completed writes remain durable after Stop or a later tool refusal.

The comment tool anchors a question to a unique exact passage and stores rune offsets, its original quote, and suggested replies. The editor highlights the passage and accepts suggested or typed answers in the owning chat. Safe edits move the anchor; agent edits that remove its words are refused. A user may edit/remove the passage directly: the question keeps its original quote and is marked as detached rather than discarded. Resolving or answering releases its protection.

## History, export, and reset

Restore creates a new user revision; earlier versions remain available. Markdown export flushes the editor, refuses an outstanding draft, opens the native save dialog, and writes a temporary file before replacing the selected destination. Canceling the dialog writes nothing. Filesystem export and round-trip tests pass; the native save/open interaction still requires manual verification.

Key removal cancels active work and preserves documents. Full reset requires typing RESET in a scope-specific dialog. It cancels work, persists a recovery marker, removes the key, and atomically clears local content/preferences. Failed cleanup retains the marker and exposes an explicit retry on startup. Reset tests use disposable databases and fake keychains, never the real workspace.
