![Kiko — every idea deserves a plan](docs/assets/readme-banner.svg)

Turn an idea into a document you can keep shaping. Kiko is a free, open-source desktop assistant designed around conversation and live, editable documents. You bring your own Meta Muse API key. The demo uses **Muse Spark 1.3 Contributor** (`muse-spark-1.3-contributor`) for discounted usage; Meta may use submitted content and responses to train its models.

**Current state: development build with the core workflow and later-release features implemented.** Chat and rich documents now sit beside research, diagrams, attachments, spreadsheets, slide decks, optional providers, and guarded suggestions. Exports include Markdown, DOCX, PDF, XLSX, CSV, and PPTX. A personal self-hosted web adapter shares the core. Local automated checks pass; live provider quality, complete accessibility/native review, Office export appearance, and cross-platform runtime certification remain pending. Follow [the checklist](docs/steps.txt) and [feature scope and limits](docs/later-releases.md).

| Product | Engineering | Community |
| --- | --- | --- |
| [The pitch](docs/kiko-pitch.md) | [Technical spec](docs/kiko-spec.md) | [Contributing](CONTRIBUTING.md) |
| [Design system](docs/design-system.md) | [Muse API contract](docs/muse-api-contract.md) | [Security](SECURITY.md) |
| [Screen reference index](docs/references/screens/README.md) | [Development checks](docs/development.md) | [GPL-3.0-only](LICENSE) |

## Run locally

You need **Go 1.27+**, **Node 24.15+ or 26+** (Node 22.22.2+ also meets the dependency requirements), npm, and **Wails v2.17.0**. Use Wails v2; this repository does not target v3.

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0
wails doctor
npm --prefix frontend ci
npm --prefix frontend run build
wails dev
```

Make sure the Go binary directory is on your PATH. Install the native prerequisites for your platform before running the doctor:

| Platform | Native prerequisites | Validation here |
| --- | --- | --- |
| macOS | Xcode command line tools | Apple Silicon build checked; current visual/launch review pending |
| Windows | WebView2 runtime and the Go C compiler toolchain required by Wails | Not yet checked |
| Linux | GCC, GTK3, WebKit2GTK development libraries; a Secret Service keyring for runtime key storage | Not yet checked |

Follow [Wails' platform setup instructions](https://v2.wails.io/docs/gettingstarted/installation/) for exact packages. Linux distributions using WebKit2GTK 4.1 require `-tags webkit2_41` on Wails commands. The architecture and reference layouts target macOS, Windows, and Linux; release packages are a later checklist step.

Frontend dependencies have exact direct versions and a committed npm lockfile. Go dependencies are recorded in `go.mod` and `go.sum`. A fresh checkout must build the frontend before a plain `go test ./...` because Go embeds `frontend/dist`.

## Check and build

```sh
npm --prefix frontend ci
node scripts/check.mjs
wails build
node scripts/setup-runtime.mjs
```

The local runner checks formatting, lint, frontend tests, strict TypeScript, the production frontend build, Go formatting, `go vet`, and Go tests with the race detector. Fix frontend formatting with `npm --prefix frontend run format` and Go formatting with `gofmt` on the reported files. See [quality checks and desktop smoke testing](docs/quality.md) for focused commands and native prerequisites.

CI is configured to run those checks and build Wails on macOS, Windows, and Linux without provider keys. Hosted results are pending connection to a remote repository. Generated Wails bindings and build outputs are excluded from Git. Run `node scripts/setup-runtime.mjs` after packaging to bundle checksum-verified Typst 0.15.1, its notices, and project license files; on macOS this re-signs the updated bundle locally. PDF/diagram rendering needs this runtime or `KIKO_TYPST_BINARY`.

To inspect only the frontend, run `npm --prefix frontend run dev -- --host 127.0.0.1`. This frontend-only URL has no Go bridge. For a working browser preview, keep `wails dev` running and use its application-server URL (normally http://localhost:34115); changes reload during development.

## Where things live

```text
main.go, app.go    Wails lifecycle and thin desktop bindings
keys.go           OS keychain adapter
core/             Storage, streaming, agent tools, document revisions, and settings
frontend/src/     React interface and interaction tests
docs/             Product, design, protocol fixtures, and ordered build steps
build/            Native packaging resources; build/bin is generated
.github/          Issue and pull-request guidance
```

## Your data and your key

Desktop Kiko stores local content, attachment bytes, and drafts in SQLite and saves provider/Tavily keys in separate OS-keychain accounts. [Personal web mode](docs/web.md) uses a separate database and an encrypted server-side vault. Pressing Connect explicitly performs a short, potentially billable readiness request to Meta; it sends no local chats/documents. Stored keys are never returned to the frontend. Routine tests use synthetic credentials and a local HTTP server. Live account verification has not been performed in this development session.

See [storage.md](docs/storage.md) for the data location and the `KIKO_DATA_DIR` development override. A missing key prompts reconnection without deleting content; a normal relaunch preserves setup and navigation. No cloud accounts, sync, or telemetry are implemented. Sending a message shares conversation history and tool results with its selected provider. Attached files are available to the agent after Send; vision can send an image or PDF to the explicitly selected helper. Optional Tavily calls share queries/URLs with Tavily. Saved conversations remain readable without a key; remote inference requires its configured key. Ollama can run locally. Connecting an additional provider also performs an explicit, potentially billable readiness request. No provider switch occurs automatically. Stop cancels the local request, without guaranteeing immediate provider-side cancellation or zero further billing.

Reference screenshots are local design inputs and are intentionally excluded from Git pending redistribution/privacy review. The index remains useful, but image links require those local files. The README banner is original project SVG artwork and is included in the source.

## Write, shape, keep

![Kiko development preview with a sample project plan](docs/assets/workspace-preview.png)

Development preview using disposable sample data, with the provider disconnected.

| Conversation | Documents | Control |
| --- | --- | --- |
| Stream Muse replies | Edit rich text beside chat | Keep your key in the OS keychain |
| Save history and drafts locally | Watch sections fill as tools finish | Stop a run and retain completed work |
| Retry interrupted replies | Answer questions on a passage | Restore earlier revisions |
| Organize chats in projects | Export Markdown, DOCX, or PDF | Review AI suggestions before applying |
| Attach images, PDFs, Office, or text | Edit sheets and slides; export XLSX/CSV/PPTX | Keep drafts through conflicts and failed saves |

PDF attachments can use **Use local page images** if the browser preview fails. Direct page/diagram images are opt-in for the exact configured OpenAI or Anthropic model under Settings; they use that chat provider's credits. The default Muse Contributor chat keeps its selected vision-helper path. Rendering a local page itself makes no provider request.

Use **New artifact** in a chat to start writing directly, or ask Muse for a plan/proposal/spec to exercise the agent workflow. [Document behavior and limits](docs/documents.md) and [release preparation](docs/release.md) describe the current implementation.

## Build with us

There is room for Go, React, accessibility, design, testing, and documentation contributions. Start with [CONTRIBUTING.md](CONTRIBUTING.md), read [agents.md](agents.md), and take a focused item from [steps.txt](docs/steps.txt). Keep unfinished work unchecked and include evidence when completing a step.

Kiko's source is licensed under **GNU GPL version 3 only**, SPDX `GPL-3.0-only`. See [LICENSE](LICENSE) and [third-party notices](THIRD_PARTY_NOTICES.md). Dependencies keep their respective licenses.
