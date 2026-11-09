# Personal web workspace

Kiko can serve its built React interface using the same Go services and SQLite storage. This is a personal, single-owner workspace. It has no account registration, team permissions, sync, or tenant isolation. It defaults to loopback and uses a separate `Kiko-Web` data directory. Desktop OS-keychain entries are not imported.

Build the frontend and application first, then start the binary with `--web`. Use your shell's secure environment configuration to supply:

| Variable | Meaning |
| --- | --- |
| `KIKO_WEB_TOKEN` | A random login token of at least 32 characters; use a generated secret, not a memorable password |
| `KIKO_WEB_MASTER_KEY` | Independent random 32-byte key, encoded as base64 or hex |
| `KIKO_WEB_ADDRESS` | Default `127.0.0.1:34116` |
| `KIKO_WEB_DATA_DIR` | Optional dedicated directory for the database and encrypted credentials |
| `KIKO_TYPST_BINARY` | Optional explicit compiler path for PDF and diagram rendering |

Do not put secrets in source files, command-line arguments, screenshots, or public logs. Keep the master key in durable secret storage. Losing it loses access to saved provider keys; replacing it does not decrypt existing entries. Back up the database, encrypted credential file, and master key through appropriate separate channels. Attachment bytes are in the database, so include its SQLite WAL in a live backup or stop Kiko before copying files.

For a local macOS package:

```sh
npm --prefix frontend ci
wails build -m -nosyncgomod
node scripts/setup-runtime.mjs
build/bin/Kiko.app/Contents/MacOS/Kiko --web
```

Visit http://127.0.0.1:34116 after setting the required secrets. Enter the server login token in the browser. Provider keys entered in Settings go to the server's encrypted vault; existing keys are never returned to the browser. A new browser gets the same owner's workspace.

Remote binding additionally requires `KIKO_WEB_ALLOW_REMOTE=1`, `KIKO_WEB_SECURE=1`, and an exact HTTPS `KIKO_WEB_ORIGIN` without a path (for example `https://kiko.example.org`). Terminate TLS at a reverse proxy, prevent direct public HTTP access to the Go port, preserve the host/origin, and disable proxy buffering on `/api/events`. Do not expose this personal adapter as a public multi-user service. Remote deployment has not been independently audited or exercised here.

Authentication uses constant-time token comparison, random 12-hour in-memory sessions, HttpOnly/SameSite=Strict cookies, exact-Origin checks for writes, bounded login attempts and session counts, and an explicit RPC allowlist. Restarting invalidates sessions. Keys are encrypted with AES-256-GCM and an account-bound authenticated context, written atomically with restrictive permissions. The vault verifies the master key before starting. This protects credential files at rest, not a compromised server process or browser. SQLite documents and attachments remain plaintext local content.

The browser receives saved message updates over authenticated SSE. Downloads use authenticated HTTP exports; clipboard actions use browser permissions. Narrow layouts show navigation as a dismissible drawer and give editors the full content width. Authentication, CSRF, RPC, vault integrity, login, and failure behavior have local automated tests. Real phones, reverse proxies, and native accessibility remain manual validation items.
