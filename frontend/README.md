# Kiko frontend

React, TypeScript, and Tailwind CSS v4 inside the Wails system web view.
Shared primitives use Lucide outline icons and Radix accessibility behavior.

The interface now includes startup, nickname/Muse onboarding, navigation,
project/chat management, local search, and profile/appearance settings.
Messaging is disabled until Phase C; document editing follows in Phase D.
The browser entry reports a missing desktop connection instead of faking one.

See the [root README](../README.md), [design system](../docs/design-system.md),
and [quality checks](../docs/quality.md). Tests inject a fake bridge; no real
credentials are required.
