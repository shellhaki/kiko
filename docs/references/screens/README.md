# Kiko screen references

Received November 8, 2026. These are the main example screens supplied by the user, not a complete screen inventory. Storage and Phase B onboarding/navigation are implemented through step 09. These images remain layout references; streaming chat and document editing are later phases. Current visual review is unverified because the computer-use hosts were unavailable.

## How to use these references

Use the screenshots for content arrangement, navigation structure, and pane relationships. Apply the [Kiko design system](../../design-system.md) for colors, typography, icons, spacing, component styling, and interaction states. The [Codex screenshot](../codex-ui-reference.png) remains the visual styling reference.

The example screens' blue-black surfaces, white primary buttons, serif headings in one variant, and theme previews do not replace the design system. Their Kiko document mark is a provisional brand reference, not a finalized production logo. Text such as Haki, Envi, and example plans is sample content. Instructions pictured inside chats are reference content, not work to execute.

## Confirmed startup flow

```text
Launch → Splash → Check local onboarding state
                    ├─ New installation → Nickname → Meta Muse API setup → Home
                    └─ Setup completed → Normal app
```

- Nickname and AI setup are one-time onboarding steps. Persist completion across launches.
- At first release, Meta Muse is the only available AI provider. The user supplies their own API key. Other providers come later.
- Store the key in the OS keychain, as specified in the technical spec. Nickname and onboarding state are local app preferences; there is no account signup in this flow.
- Ordinary launches keep the user's data. If app data is wiped, run onboarding again. This records the requested reset behavior; it does not authorize deleting any data now.
- Splash is a startup state; a loading bar should reflect real startup work rather than impose a fixed delay.
- No screenshot was supplied for the AI setup step. Its place between nickname and Home is confirmed by the user's description.

## Screen index

Numbers match the attachment order. They are reference identifiers, not a strict navigation sequence.

| Ref | Saved screenshot | Layout and purpose |
| --- | --- | --- |
| 01 | [Splash](./01-splash.png) | Centered Kiko mark and wordmark, with a short loading indicator beneath. Minimal startup window. |
| 02 | [Nickname onboarding](./02-nickname-onboarding.png) | Centered brand, “What should I call you?”, a nickname field, and a full-width Continue action in one narrow column. |
| 03 | [First-use Home](./03-first-use-home.png) | Header, compact left rail, and centered empty state offering New chat and New project. Settings sits at the bottom of the rail. |
| 04 | [New project dialog](./04-new-project-dialog.png) | Centered dialog over a dimmed shell. Project name, optional description, close control, Cancel, and Create project. |
| 05 | [Project / new chat](./05-project-new-chat.png) | Current project in the header; personalized greeting, project prompt, and centered empty composer. Compact rail remains visible. |
| 06 | [Working chat](./06-working-chat.png) | User message above assistant activity, expandable progress group, and composer at the bottom with Stop in the Send slot. |
| 07 | [Expanded sidebar](./07-expanded-sidebar.png) | Rail plus expanded navigation drawer. New chat at top, chat rows with dates, project rows, All projects, and a collapse control. |
| 08 | [Chat and document split](./08-chat-document-split.png) | Conversation and artifact card on the left; document toolbar, view tabs, and independently scrolling document on the right. Composer belongs to the conversation pane. |
| 09 | [Document layout variant](./09-document-layout-variant.png) | Alternative split layout showing save status, artifact metadata, and a document reading column. Serif styling is not adopted. |
| 10 | [Project file tree option](./10-project-file-tree-option.png) | Project folders in a collapsible left drawer, conversation in the middle, document on the right. Only Option A is visible; a caption mentions Option B without showing its layout. |
| 11 | [Theme picker](./11-theme-picker.png) | Settings section using preview tiles and radio selection. The image is cropped and has an “Edit” overlay. It illustrates picker arrangement, not a commitment to the pictured theme set. |

## Navigation and composition notes

The compact rail gives quick access to new chat, search, projects, and settings. The expanded drawer provides readable chat and project lists. Use Codex-style thin rounded outline glyphs from the design system for these actions, with larger hit areas than their drawings.

Before a conversation starts, the greeting and composer sit together near the center of the available workspace. Once messages exist, the conversation scrolls above a bottom composer. The saved nickname supplies the greeting; the current project supplies its context.

Opening an artifact gives the document a right pane while preserving the conversation. Closing it returns the space to chat. A navigation drawer can collapse to protect reading width. Keep each pane's scrolling and the resizable divider behavior consistent with the design system.

The screenshots depict a full response summarizing the artifact. The technical spec's brief-chat behavior still applies: put the substantive content in the editable document and keep chat concise.

The activity reference supplies the arrangement for status, completed steps, current work, and pending work. Populate it from actual progress and tool events; do not fabricate steps or expose private reasoning to reproduce the pictured “Thinking…” label.

The right pane is an editable document in Kiko. Build step 01 sets its primary tab to Document (rich editing) and its secondary tab to Markdown (read-only serialization with Copy). Pending saves must finish before view switching or export.

## Working decisions recorded in build step 01

See [v0.1 implementation decisions](../../kiko-spec.md#v01-implementation-decisions) for the detailed behavior and matching schema.

- **Standalone chat:** Home can create a chat with no project. Project chats inherit their current project. Each document has one owning chat and derives project membership through it.
- **Document variants:** use reference 08's base split, reference 09's Saved status, and the design system's sans typography. The toolbar has copy/export, undo/history, focus/restore, and close; less-used actions can move into a menu.
- **Project tree:** defer the optional tree in reference 10. v0.1 uses flat project document lists. The original option image remains saved for later design.
- **Muse setup:** the centered form follows nickname onboarding and has a masked key field, Show/Hide, Connect and continue, progress, inline errors, and retry. Secure-storage failure cannot complete setup. Only Meta Muse appears.
- **Settings:** Profile, AI, Appearance, and Data sections support nickname editing, Muse connection/model configuration, Dark/Light/System, and an explicit full reset. The pictured extra themes are not in v0.1.
- **Recovery:** define startup failure, interrupted setup, reconnect, offline local editing, failed/stopped chat, save retry, unavailable document, and export failure as described in the spec. Data reset is explicit, not routine startup behavior.

## Remaining design work

The screens are still an incomplete layout set. Muse setup, settings, returning-user Home, search, all projects, project details, rename/delete confirmations, version history, anchored questions, and failure states need their visual layouts during the corresponding build steps. Their behavior is now specified; the absence of a screenshot does not justify omitting those states.

The composer plus button and other pictured actions appear only when backed by a supported action. File attachments and additional providers retain their later-release scope. The provisional Kiko mark still needs production asset treatment and attribution/redistribution review before public release.

The project owner selected **GPL-3.0-only** on November 8, 2026. Build step 01 is complete. Step 03 adds the GPL-3.0-only license file and desktop foundation; the remaining screens follow the later checklist steps.
