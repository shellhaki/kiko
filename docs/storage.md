# Local storage

The framework-independent package `core/storage` owns the SQLite connection, versioned schema, typed preferences, workspace metadata, selection, and local search. The desktop adapter opens it lazily during Bootstrap, closes it on shutdown, and never binds raw SQL access to the frontend.

## Location and lifecycle

The default file is `Kiko/kiko.db` under Go's `os.UserConfigDir()` directory: Application Support on macOS, AppData on Windows, and the user configuration directory on Linux. `KIKO_DATA_DIR` overrides the parent directory for isolated development/testing. It changes the database path, not the operating-system keychain namespace.

`Open` creates missing parent directories, opens existing data without truncation, enables foreign keys and a five-second busy timeout on every physical connection, and runs pending migrations. It retains one connection by default and serializes transactions with SQLite's immediate locking mode. Call `Close` when finished. Within a transaction callback, use the supplied transaction; calling Store methods that acquire another connection can deadlock the single-connection pool.

The driver is pinned to `modernc.org/sqlite` 1.62.0, a Go SQLite implementation. Connection options are defined by the [driver documentation](https://pkg.go.dev/modernc.org/sqlite), and schema version/foreign-key behavior follows [SQLite's PRAGMA documentation](https://www.sqlite.org/pragma.html). Kiko builds the file URI from an escaped absolute path rather than accepting an arbitrary DSN.

## Schema and migrations

`migrations/001_initial.sql` and `002_conversations.sql` are the executable schema sources. `PRAGMA user_version` records the applied version. Pending migrations and their version changes run together in a transaction. A failed migration rolls back its DDL and data changes; a database with a future version returns `ErrSchemaTooNew`. Corrupt files and failed opens are reported without resetting or deleting the file.

Append subsequent embedded migrations to the ordered migration list in `store.go`; never edit an already-released migration to change existing installations. Initial schema changes here are pre-release. Migration tests cover real version-1 message backfill into version 2, failed DDL, and an unsupported future version.

Relationships include nullable project ownership for standalone chats, required chat ownership for documents, nested block parents within one document, same-document comment parents/anchors, message-owned tool steps, and cascaded deletion of project/chat/document contents. Triggers prevent selecting a document in another chat or reassigning an open document to a conflicting owner. Deleting a document clears its saved view selection.

`workspace_state` holds the selected project/chat with foreign keys. `chat_view_state` holds an open document and future view preferences. Typed global preferences contain nickname, onboarding stage, Dark/Light/System choice, sidebar expansion/width, and a reserved reset marker. Onboarding stages are `nickname`, `muse`, and `done`; stage `done` is the completion flag. Invalid preferences produce an error rather than silently resetting setup.

The schema seeds the Muse provider configuration once. Reopening does not overwrite a selected model or user data. Provider rows and typed preferences have no API-key field. Keys belong in the OS keychain; only the currently typed input exists temporarily in the frontend.

## Workspace operations

Selecting a canonical folder creates or reuses its project and updates selected context atomically. Legacy virtual-project creation remains an internal compatibility operation; new desktop UI uses the folder picker. Chat creation updates context atomically. Names are trimmed and limited to 120 Unicode characters. A project description is optional. Rename/delete accept only a fixed project/chat kind; IDs and names are bound SQL parameters. Deletion is permanent in this release, so the interface confirms its scope first.

Search finds chat titles, document titles, and active document-block text, treating `%` and `_` as literal search input and returning at most 50 results. It is local SQLite search, not web search. Opening a document also restores its owning chat/project. Documents open in the rich editor alongside chat.

## Validation

Temporary-file tests cover all initial tables, fresh defaults, close/reopen persistence, Unicode-safe file paths containing URI punctuation, rollback on error and panic, migrations, future-schema refusal, concurrent initialization, foreign keys on two connections, ownership guards, deletion cascades, document selection, local search, preferences, and startup resumption. They do not create or reset the user's normal data directory.

## Conversations and recovery

Migration 002 orders messages within each chat and adds `chat_runs`, deduplicated `chat_requests`, and scoped `drafts`. A send atomically stores one user message and one assistant placeholder, selects its chat, and removes only the submitted scope’s draft. Repeated request IDs do not submit another provider call. At most one run per chat can be active.

Replies persist full content snapshots before publishing UI events, at most every 50 ms plus a final write. Events include chat/message/run identity, attempt, and monotonically increasing sequence. Old attempts cannot update newer ones. Retry retains the same user/assistant rows and archives the previous partial reply. A failed final save is reported in the current window; unsaved output is not claimed durable.

On startup, running records become interrupted while retaining their saved text. Normal shutdown cancels and waits for active replies before closing SQLite. A native single-instance lock prevents another normal app launch from recovering a live run. Do not run separate development processes against the same database. Deleting a chat/project cancels its active requests and cascades its messages, attempts, and drafts.

Drafts are scoped to Home, project greeting, or chat. The frontend serializes draft saves and waits before submitting. Missing keys block sending, while history remains readable and drafts remain editable. Local HTTP and service tests cover streaming, cancellation, late callbacks, retry, persistence, recovery, and save failures without real credentials.

## Documents, drafts, and questions

The document API and concurrency contract are documented in [documents.md](documents.md). Migration 003 adds recoverable document drafts with their base revision; migration 004 stores the original question quote so a user edit cannot erase its context. Revisions contain before/after snapshots, and ordered nested blocks remain queryable independently. Normal launches migrate existing databases without clearing them. Reset is a separate explicit action with a recoverable preference marker and keychain cleanup.

## Later-release migrations

Migration 005 selects the owner-requested Contributor model. Migration 006 adds chat-owned attachment BLOBs, extracted page JSON, thumbnails, and FTS5 page indexing. Source bytes are bounded at 20 MB/file and 512 MB aggregate; deletes/reset cascade into the content and index. This transactional BLOB layout supersedes the proposed attachment filesystem paths; no provider Files upload IDs are currently persisted.

Migration 007 adds typed sheet/deck artifact payloads, recoverable drafts, revision snapshots, and selected artifact IDs. Saves compare revisions and atomically append history. A conflicting draft stays recoverable until explicitly discarded. Document and artifact panes are mutually exclusive per chat. Artifact history is stored but does not yet have a restore UI. Migration 008 adds document proposals, their source revision and previews, and accepted/rejected status. Accept revalidates the source revision before one atomic user revision.

Migration 012 makes Muse the only provider: it moves every chat to `muse`, deletes other provider rows and the old image-consent setting, and adds `chat_models`, one row per chat holding its model id. A trigger gives each new chat the current default model, and choosing a model updates both the selected chat and that default. Provider configuration is stored without keys. Keychain entries saved by older builds for other providers are no longer read and can be deleted in your keychain app. Desktop credentials remain isolated OS-keychain accounts; web credentials use the AES-GCM vault described in web.md. No normal migration clears content. Back up existing databases before deploying a new development build; tests use temporary stores.

## Folder workspace migration

Migration 009 adds `project_roots` (one canonical disk root per project), `chat_permissions` (retained inert schema) and `file_permissions` (now delete-only approvals; writes apply directly), `workspace_files` (source identity, relative path, and last written hash), and `workspace_sync_errors` (retryable source-copy failures). These tables leave existing project/chat/document IDs intact. Foreign keys cascade metadata, never disk files. Reopening preserves roots. See [folder workspaces](folder-workspaces.md) for the current behavioral contract and separate SQLite/disk save semantics.

Migration 010 adds `workspace_text_drafts`, keyed by project and normalized relative path with its base content hash. Source path reservations reuse `workspace_files` with an empty hash until a disk write succeeds; this preserves custom paths through failed writes and retries. External-version reloads use ordinary document/artifact revision writes. Imported source files retain their original disk hash until a later explicit edit/save. Migration 011 adds a `kind` column to `file_permissions` so delete approvals are distinguishable from legacy write rows.
