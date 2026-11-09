# Release preparation

The current version is 0.1.0-dev. It is not certified as a finished v0.1 release. Do not publish it as production-ready before the remaining items in steps.txt have evidence.

## Reproducible build inputs

Use the declared Go toolchain, Node version in .nvmrc, npm lockfile, and Wails 2.17.0. Run `npm --prefix frontend ci` followed by `node scripts/check.mjs` from the repository root. No provider key is required for those checks.

| Platform | Package command | Current evidence |
| --- | --- | --- |
| macOS | `wails build -m -nosyncgomod` | Apple Silicon build; native interaction review pending |
| Windows | `wails build -m -nosyncgomod` | CI matrix prepared; hosted build/runtime pending |
| Linux | `wails build -m -nosyncgomod -tags webkit2_41` | CI matrix prepared with native packages; hosted build/runtime pending |

Native prerequisites and commands are in development.md and quality.md. The existing workflow performs checks and platform builds without keys. It does not publish. No remote repository is configured in this workspace.

## Distribution requirements

Include LICENSE, third-party notices, release notes, and the corresponding GPL-3.0-only source for any distributed binary. Check all dependency and bundled asset notices against the actual package. Reference PNGs remain excluded from Git because they contain personal/contextual information and have not passed redistribution review. The original project SVG banner is included.

macOS currently uses local self-signing from Wails; Developer ID signing/notarization has not been performed. Windows signing and Linux distribution testing remain pending. The local macOS linker reports that its Go object targets macOS 13 while the link target is 11; set and verify the actual supported minimum before release certification.

Use CHANGELOG.md as the draft release notes. Replace the development version only after validating the intended release, obtaining real workflow evidence, and preparing matching packages/source. Uploading or publishing requires the owner’s instruction.

## Runtime and export packaging

After the native Wails build, run `node scripts/setup-runtime.mjs` on the target host. It downloads checksum-pinned Typst 0.15.1 for that host, bundles its LICENSE/NOTICE and manifest, includes project license/third-party notices, and re-signs the modified macOS bundle locally. Never add runtime resources after final production signing without signing again. Frontend build copies offline pdf.js fonts/CMaps/WASM and their notices into embedded assets. Review the final binary, assets, corresponding source, and all bundled notices together before distribution.

Later-version functionality is present in the development build; version promotion still requires native export interaction and visual checks in Office applications, live provider/account checks, platform coverage, and the release audit. An XML-structure or PDF-page-count test does not certify Office appearance. The personal web adapter is scoped to one owner and still needs remote deployment/security review before public exposure. No packages were uploaded or published.

The embedded PDFium/WASM renderer adds `build/pdfium-manifest.json` and `build/pdfium-notices.md` to the redistribution materials. Preserve them with THIRD_PARTY_NOTICES.md; the runtime setup copies them into native resources. The quality runner checks the embedded WASM digest against the pinned module manifest. Production macOS hardened-runtime signing still needs a check of the WASM compiler's executable-memory requirements; current rendering evidence uses a local ad-hoc signature.
