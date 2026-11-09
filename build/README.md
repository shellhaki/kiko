# Native build resources

`appicon.png` and `darwin/Info.plist` are Wails v2 defaults generated during the first native build. They are placeholders for the later branding and release-packaging work, not final Kiko artwork or minimum-OS certification. Their upstream attribution is recorded in THIRD_PARTY_NOTICES.md.

`bin/` contains local build outputs and is excluded from Git. macOS bundles produced here are for local development; developer signing, notarization, platform identifiers, final icons, and release distribution are covered by step 24.
