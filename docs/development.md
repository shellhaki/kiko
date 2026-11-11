# Development validation

## Step 03 — November 8, 2026

Environment: macOS 26.6 on Apple Silicon, Go 1.27.3, Node 26.8.2, npm 11.19.1, Xcode 27.2, and Wails CLI 2.17.0. Dependencies are pinned through go.mod/go.sum and frontend/package-lock.json. The Wails CLI used for this session was installed in a temporary tools directory; contributors can follow the normal install command in the README.

Completed checks:

- A clean `npm ci` completed from the frontend lockfile.
- `npm run build` passed TypeScript compilation and the Vite production build with Tailwind v4.
- `npm test` passed three frontend tests: pending/successful desktop connection, browser-only state, and failed connection recovery.
- `npm run lint` passed with warnings treated as failures.
- `go test ./...` passed the JSON contract test for the frontend binding.
- `go vet ./...` passed.
- `wails doctor` confirmed the native macOS prerequisites.
- `wails build -m -nosyncgomod` produced `build/bin/Kiko.app` (frontend build, native compilation, packaging, and local self-signing). The flags keep the already-resolved Go module files unchanged.
- Opened that app bundle and observed the native window at `wails://wails/`, displaying “Kiko is ready. 0.1.0-dev”. This verifies the actual React → Wails → Go → React call, beyond mocked tests.
- Checked Git exclusions for node_modules, generated assets/bindings, binaries, local databases, environment files, and the private reference PNGs.

The native compiler emitted a deployment-target linker warning; the resulting app launched on the recorded macOS version. This does not certify compatibility with older macOS versions. Windows/Linux builds, production signing/notarization, and minimum-OS verification remain later work. Wails' default application icon and bundle metadata remain packaging placeholders.

No remote was configured and no files were committed, pushed, or published. No API key was requested, stored, or used. The app is a foundation screen, not onboarding or a working AI chat.

Step 04 now supplies CI configuration and the shared local quality runner; see the validation record below. Repeat native launch checks when the bridge, window setup, or packaged assets change. Avoid sharing unredacted verbose build output: Wails can include environment values in its diagnostic logs.

## Step 04 — November 8, 2026

Added `node scripts/check.mjs` as the shared local/CI command, Prettier 3.9.10 with an explicit formatting configuration, strict TypeScript checks, editor and LF line-ending conventions, and the Go 1.27.3 toolchain declaration. The runner builds the frontend before inspecting/testing Go packages, fails on formatting differences, disallows Go module edits, runs vet, and runs tests with the race detector.

Validation on the step-03 macOS environment:

- A temporary unformatted TypeScript file caused the runner to return nonzero and identify the file. The probe was removed immediately afterward.
- A clean offline install from the updated lockfile succeeded using the local npm cache.
- The complete local runner passed: formatting, lint without warnings, three frontend tests, strict TypeScript and production build, Go formatting, vet, and the race-enabled Go contract test.
- `node --check scripts/check.mjs` passed.
- Actionlint 1.7.12 accepted `.github/workflows/ci.yml`.
- The native Wails macOS build passed again. The previously recorded deployment-target linker warning remains; minimum-OS certification is still release work.
- Updated Prettier license attribution and the frontend transitive dependency inventory.

The workflow defines macOS 15, Windows 2025, and Ubuntu 24.04 jobs with exact official action commit pins, locked frontend installation, explicit native prerequisites, quality checks, and native Wails builds. It does not upload or publish outputs. No hosted CI jobs were executed because this checkout has no configured remote; Windows/Linux results remain unverified. The desktop smoke-check procedure is in [quality.md](./quality.md), with step 03's observed native bridge response as the current manual baseline. Application behavior did not change in this step.

## Steps 05–09 — November 8, 2026

Implemented the storage foundation and Phase B shell/onboarding/navigation. `core/storage` owns an embedded version-1 schema, connection configuration, migrations, typed preferences, and workspace CRUD/search. The desktop adapter opens the database in the platform configuration directory (or KIKO_DATA_DIR) during Bootstrap and closes it at shutdown. `core.Service` coordinates onboarding with a Go-only OS keychain adapter and the bounded Muse connection verifier.

The interface now has a real startup state, nickname and Muse forms, keyboard-accessible dialogs/action menus/tooltips, a rail and resizable/collapsible sidebar, project/chat navigation and management, local search, and profile/theme settings. Saved context survives reopen. The composer is intentionally disabled before Phase C. Document selection restores context, but editing/export and full settings/reset/key management remain later checklist work.

Checks passed:

- The shared local runner: formatting, lint without warnings, strict TypeScript, production frontend build, 14 frontend tests, Go vet, and all Go tests with the race detector.
- Frontend scenarios cover startup pending/error/retry, nickname setup, failed/successful connection, in-session key retention, missing-key reconnect, project creation, standalone chat creation, rename, deletion confirmation, search, sidebar keyboard resizing, theme changes, and dialog focus restoration.
- Storage tests use temporary files to exercise every initial table, persistence, migrations, failed DDL rollback, future schema refusal without data loss, concurrent initialization, per-connection foreign keys, deletion cascades, owner protection, selected context, and local search.
- Core/verifier tests cover readiness request shape, HTTP errors, malformed/truncated responses, redirects without credential forwarding, canceled verification, failed key storage, restoring the previous key if database completion fails, and snapshots/preferences containing no saved key. Keyring adapter tests use the library's mock implementation, not the real OS keychain.
- The final macOS Wails production build passed. The existing deployment-target linker warning remains; minimum-OS/platform certification is release work.
- Updated direct dependency licenses and transitive inventories for SQLite, keyring, Lucide, and Radix.

No live Meta request was made and no real API key was read or saved. HTTP tests use local test servers and synthetic keys. The browser host reported unavailable, and native inspection reported a pipe-startup failure, so the new screens were not visually inspected and a current native launch/relaunch has not been observed. A disposable app copy was prepared under the temporary directory, but inspection could not proceed. The temporary frontend visual harness was removed and its local server stopped; it is not an application entry or a bypass of onboarding.

The earlier step-03 native launch remains historical evidence only. Repeat the current manual procedure when computer-use access is available, and perform live connection verification only through an explicitly initiated Connect action with a user-provided key. Stopped at Phase C; step 10 has not been implemented.

## Steps 10–11 — November 8, 2026

Implemented Phase C: Go-only Muse Chat Completions streaming, durable message snapshots, request deduplication, scoped drafts, Markdown rendering, Stop, retry with archived partial attempts, and interrupted-run recovery. The frontend handles IME/Shift+Enter, stream ordering, chat changes, missing keys, and manual scrolling. A two-second snapshot check while running recovers dropped bridge events. Native launches use a single-instance lock.

The live development application server is http://localhost:34115 while `wails dev` runs; Vite alone does not supply Go bindings. No preview bypass or fake provider is included in the app. Tests use a local HTTP server and fake provider/keychain.

Validation: 21 frontend tests, strict TypeScript/production build, formatting/lint, Go vet and all Go race tests pass. Protocol tests cover split Unicode/SSE frames, terminal states, malformed responses, redirects, timeouts, cancelable retry, and no automatic replay after text. Service/storage tests cover idempotency, history, Stop/retry, stale writes, draft persistence, migration/recovery, deletion during streaming, and visible save failures. Native build evidence is recorded in the checklist. The frontend build reports a 515 KB main chunk; optimization remains possible.

Live account generation and visual/native interaction review remain unverified. The earlier computer-use hosts were unavailable; no real API key was supplied or used. Phase D document editing remains unchecked.

## Document workflow — November 9, 2026

Implemented revisioned nested documents and guarded operations, a Tiptap split pane with draft recovery/debounced saves, document tools with saved activity/replayed context, anchored questions/answers, history restore, native Markdown export, and settings/key/reset recovery. See documents.md for the implementation contract.

The automated suite now covers actual Tiptap transactions and debounce saves, draft recovery and failed-save navigation, Markdown switching, anchored question highlights/answers, reset recovery, scoped tool execution, incomplete arguments, step limits, safe concurrent edits, historical reconstruction, anchor movement, and atomic filesystem export. Routine tests never contact Meta or manipulate real keychain data.

Visual inspection was retried: native pipe startup failed and the browser access host was unavailable. The preview server remains reachable at http://localhost:34115. A fresh native build and final quality results are recorded in steps.txt. Live model behavior, native save-dialog interaction, real OS-keychain replacement/reset, and Windows/Linux runtime review remain unverified. Current builds retain a ~521 KB main JavaScript chunk warning; the ~471 KB editor chunk is lazy-loaded.

## Contributor model switch — November 9, 2026

The default model is `muse-spark-1.3-contributor`; each chat can choose another Muse model from the catalog in `core/muse/models.go`, and explicit retries use the chat's current model. Migrations 005 and 012 change existing configuration without clearing chats/documents. Historical run models remain recorded until a user explicitly retries. There is no automatic fallback to another model.

All local quality checks pass, including 31 frontend tests and Go race tests. A regression test upgrades a simulated version-4 database, preserves its saved messages, verifies Contributor retry selection, and checks reset defaults. Tests use synthetic credentials/local servers; no live inference was performed. Setup/settings disclose Contributor training usage and do not call it free.

## Editor resize crash — November 9, 2026

The native dev log identified `TypeError: null is not an object (evaluating event.currentTarget.parentElement)` in the document resize handler. It retained the React pointer event inside a state updater, which React could replay after currentTarget was cleared. A StrictMode regression test with captured pointer drags reproduced the same failure before the fix.

The handler now measures the workspace and calculates a bounded width synchronously, then passes only the numeric width to the updater. A document error boundary contains both lazy-loading and editor render failures and offers Retry editor / Return to chat. It does not clear stored content or drafts. Recovery tests keep a sibling conversation visible, retry the editor, and close a failed pane. All document interaction tests now run in StrictMode, matching the app entry point.

The frontend suite passes 34 tests. Automated results establish the reproduced failure and recovery behavior; visual/native interaction still requires review when inspection is available.

## Later-release checks

See [later-releases.md](later-releases.md) for implemented scope and explicit limits, and [web.md](web.md) for the personal HTTP adapter. Routine checks do not contact providers or Tavily. Additional Go suites exercise OOXML structure, spreadsheet calculations/exports, attachment ownership/import limits, research requests, compatible providers/Claude streaming, suggestions, sessions/origins, and encrypted vault integrity. Frontend tests cover artifact draft/save/conflict recovery, local attachments, and browser login/RPC alongside the existing document and conversation tests.

Bundle the pinned runtime after a native build:

```sh
node scripts/setup-runtime.mjs
KIKO_TYPST_BINARY="$PWD/build/bin/Kiko.app/Contents/Resources/typst" go test -count=1 ./core/export ./core/deck
```

Use `build/bin/typst` on Linux and `build/bin/typst.exe` on Windows. The setup script verifies the official release checksum and preserves runtime/font notices. Changing a signed macOS bundle requires re-signing; setup performs local ad-hoc re-signing after copying resources. This does not provide Developer ID or notarization. CI includes this packaging and compiler integration step but has not run on a hosted repository yet.
