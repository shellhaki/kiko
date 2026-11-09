# Security

Kiko is a development-stage desktop application and has no supported production release yet. Muse onboarding sends a short provider request only after Connect is pressed. Keys are stored through the OS keychain adapter and never returned through frontend bindings. Routine tests use synthetic credentials and fake secure storage; live account behavior remains unverified. Storage and provider behavior are documented in the spec and API contract.

## Report a vulnerability privately

If the hosting repository has GitHub private vulnerability reporting enabled, use **Security → Report a vulnerability**. This local scaffold has no configured remote or published security contact yet. If private reporting is unavailable, ask the maintainer for a private contact without including vulnerability details in a public issue. A maintainer must configure a working private reporting channel before release.

Provide the affected version/platform, a minimal reproduction, and the impact you observed. Remove API keys, personal documents, and unrelated private information. Do not upload a real key to demonstrate a leak. If a credential has been exposed, revoke it through the provider dashboard.

Public issues are appropriate for ordinary bugs; they are not the place to publish an unpatched vulnerability or sensitive data. There is no promised response time or bug bounty at this stage.
