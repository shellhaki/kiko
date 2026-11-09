# Contributing to Kiko

Kiko is early in development. Read [the checklist](docs/steps.txt) before picking work so contributions follow the current dependencies and scope. The implementation rules are in [agents.md](agents.md).

## Start a change

1. Fork or obtain a checkout, then follow the [README setup](README.md#run-locally).
2. Create a branch for one focused change. Discuss large product or architecture changes before implementing them.
3. Follow the technical spec for behavior, the design system for styling, and the supplied examples for arrangement. Screenshots may be available only in the maintainer's local reference folder.
4. Write meaningful tests for each new functionality, including relevant failure paths. Keep code straightforward, identifiers clear, and comments limited to necessary explanations.
5. Run the checks below and describe the result in your pull request. Update docs when behavior changes.

```sh
npm --prefix frontend ci
node scripts/check.mjs
wails build
```

Follow [the quality and desktop smoke-check guide](docs/quality.md). Run `npm --prefix frontend run format` and `gofmt` on changed Go files when formatting checks fail. Report which operating system you checked; a successful frontend build is not evidence that every native platform works. UI changes should include screenshots with personal content and credentials removed.

## Track completion honestly

Mark a step `[x]` in `docs/steps.txt` only when its deliverables and checks are complete. Add its date and a short evidence note. Partial work stays unchecked. Keep step numbers stable and record remaining work explicitly.

## Keys, fixtures, and dependencies

Never put API keys, real conversations, local databases, or credentials in code, issues, screenshots, fixtures, or test output. Routine tests run without a real key. Synthetic protocol fixtures belong in `docs/fixtures/muse`; real provider checks must be explicit and never run by default in CI.

Keep direct frontend versions exact and update the lockfile with changes. Review dependencies and record attribution in `THIRD_PARTY_NOTICES.md`; do not add libraries for functionality a few clear lines can handle. Do not redistribute reference images until their rights and personal information have been reviewed.

## Pull requests

Describe the concrete problem, resulting behavior, validation, and any remaining limitation. Small, reviewable changes are easier to integrate. Do not claim an unfinished feature is available in the README.

Contributions to Kiko are provided under GPL-3.0-only, the project's chosen license. Retain third-party notices when adapting external material. Security reports follow [SECURITY.md](SECURITY.md).
