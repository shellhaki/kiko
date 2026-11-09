# Kiko design system

Version 1 · November 8, 2026

## Design authority

Kiko takes its visual direction from the Codex desktop interface in the supplied screenshot: neutral charcoal surfaces, quiet navigation, generous conversation space, a rounded composer, restrained purple accents, and small rounded outline icons.

This document is the source of truth for Kiko's interface styling. It replaces the earlier warm grey, clay accent, and serif document direction in the technical spec. Product scope and behavior still come from [the technical spec](./kiko-spec.md).

Future example screens define the arrangement of their content and controls. Apply this system's colors, typography, icons, spacing, materials, and interaction states to those layouts. A reference screen's theme, branding, fonts, icon library, or decorative treatment does not override this system. Where a reference lacks a state or component, use the defaults below.

## Visual reference

![Codex desktop visual reference](./references/codex-ui-reference.png)

The saved screenshot is the visual anchor. The main sampled surface colors are `#181818`, `#1D1D1D`, `#262626`, `#363636`, and the message purple `#4A3083`. Other tokens below are implementation choices informed by the screenshot. Font identity, icon source, and exact logical dimensions cannot be established from the image; the choices here reproduce its character across platforms.

Use logical CSS pixels for all dimensions. The screenshot is a scaled desktop capture; do not copy its bitmap measurements directly into CSS. The operating system menu bar, dock, traffic lights, personal avatar, app integrations, and screenshot thumbnail are surrounding context, not Kiko components. Use native window controls where the platform supplies them.

## Principles

- **Content carries the screen.** Conversation and documents receive the space and strongest text contrast. Navigation remains quiet.
- **Neutral structure, selective purple.** Panels use neutral greys. Purple identifies sending, user messages, links, and focused interactions.
- **Few containers.** Assistant prose sits directly on the workspace. Use cards for objects and grouped actions, rather than wrapping every paragraph.
- **Soft controls, precise alignment.** Rounded rows and inputs sit on a consistent spacing grid. Icons, labels, and hit areas align throughout the app.
- **Immediate feedback.** Hover, press, editing, saving, and tool activity have clear states. Animation supports the action.
- **Familiar desktop behavior.** Resize panels, navigate by keyboard, select text, open contextual menus, and undo edits without leaving the task.

## Color system

Dark mode is the reference appearance and the initial Kiko theme. Light mode uses the same hierarchy and geometry; its palette below is a proposed companion theme, since no light reference was supplied. Offer System, Dark, and Light in settings; an unset preference starts in Dark.

| Semantic CSS token | Dark | Light | Role |
| --- | --- | --- | --- |
| `--bg-workspace` | `#181818` | `#FFFFFF` | Conversation and document canvas |
| `--bg-sidebar` | `#1D1D1D` | `#F5F5F5` | Project and chat navigation |
| `--bg-chrome` | `#262626` | `#EBEBEB` | Optional rail and window toolbar |
| `--bg-hover` | `#292929` | `#E8E8E8` | Hovered row or quiet button |
| `--bg-selected` | `#2F2F2F` | `#E2E2E2` | Selected navigation row |
| `--bg-raised` | `#262626` | `#FFFFFF` | Menus, popovers, dialogs |
| `--bg-input` | `#262626` | `#F5F5F5` | Standard text fields |
| `--bg-composer` | `#363636` | `#F0F0F0` | Large conversation composer |
| `--bg-pending` | `#222222` | `#F5F5F5` | Unwritten document sections |
| `--border-subtle` | `#303030` | `#E3E3E3` | Panel boundaries and separators |
| `--border-control` | `#808080` | `#767676` | Boundary when needed to identify a control |
| `--text-primary` | `#EEEEEE` | `#202020` | Main text and headings |
| `--text-secondary` | `#B5B5B5` | `#555555` | Supporting copy and inactive icons |
| `--text-muted` | `#A1A1A1` | `#666666` | Metadata and placeholders |
| `--accent-solid` | `#7950CC` | `#7042BF` | Primary action, enabled Send |
| `--accent-hover` | `#865CDD` | `#6335AF` | Hovered primary action |
| `--accent-pressed` | `#6840BA` | `#55299C` | Pressed primary action |
| `--accent-text` | `#B48AF7` | `#7042BF` | Links and accent icons |
| `--accent-subtle` | `#30243F` | `#EFE8FA` | Anchored question and subtle accent fill |
| `--message-user` | `#4A3083` | `#EEE6FA` | User message bubble |
| `--message-user-text` | `#F5F0FF` | `#332044` | Text on user messages |
| `--text-on-accent` | `#FFFFFF` | `#FFFFFF` | Text or icons on solid accent |
| `--focus-ring` | `#C5A0FF` | `#7042BF` | Keyboard focus |
| `--selection` | `#543B7A` | `#DDC9F7` | Text selection fill; keep primary text |
| `--status-success` | `#8ACDA6` | `#247044` | Saved or completed |
| `--status-warning` | `#E4BF7B` | `#805B16` | Actionable warning |
| `--status-danger` | `#F29999` | `#B42332` | Error and destructive action |

Do not place muted text on a solid purple button. Keep links underlined in prose so color is not their only cue. Selected navigation uses a neutral fill and stronger text; reserve purple for the small number of roles above. Status colors occupy icons and short labels, rather than large colored panels.

Use flat, opaque surfaces as the baseline. Separate the sidebar from the workspace with a subtle tonal step and a single thin boundary. Standard cards have no shadow. Floating menus and dialogs may use `0 12px 36px rgb(0 0 0 / 24%)` in dark mode and `0 8px 28px rgb(0 0 0 / 12%)` in light mode. Use blur only on a floating layer that actually overlaps scrolling content, with an opaque fallback.

## Typography

Use a system sans family for the interface, chat, and editable documents:

```css
--font-sans: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
--font-mono: ui-monospace, "SFMono-Regular", Consolas, "Liberation Mono", monospace;
```

Use the platform's installed system font; do not bundle Apple's fonts for other systems. Keep `font-optical-sizing: auto`. Body tracking is normal; headings may use `-0.02em`. Weight 400 is normal, 500 marks controls and selection, and 600 marks headings. Avoid weight 300 for small text.

| Role | Size / line height | Weight |
| --- | --- | --- |
| Metadata, shortcut, tooltip | 12 / 16 px | 400–500 |
| Section label, compact control | 13 / 18 px | 400–500 |
| Sidebar row, toolbar, menu | 14 / 20 px | 400; selected 500 |
| Conversation and composer | 15 / 24 px | 400 |
| Document body | 16 / 26 px | 400 |
| Document heading 3 | 18 / 26 px | 600 |
| Pane title, document heading 2 | 20 / 28 px | 600 |
| Document title | 28 / 36 px | 600 |
| Empty conversation prompt | 28 / 36 px | 500 |
| Code | 13 / 20 px | 400 |

Use sentence case. Left align prose and labels. Keep body measures around 65–80 characters and preserve comfortable paragraph spacing. Chat and documents share the sans family; a document gains hierarchy from size, leading, and spacing. Exported document templates may have their own typography without changing the editor chrome.

Store text sizes and related spacing in rem units, with the values above based on a 16 px root. Support text scaling and zoom without clipping controls or hiding content.

## Iconography

The screenshot's icons are small, restrained line drawings: rounded caps, rounded joins, open interiors, and simple familiar silhouettes. Examples include a folder, magnifying glass, bell, square with a pencil, sidebar rectangle, history clock, ellipsis, plus, microphone, and send arrow. Their optical weight is lighter than adjacent text. Most icons are unfilled; the selected Home icon is a deliberate filled exception.

**Implementation choice:** use `lucide-react` as Kiko's starting library, with its rounded caps and joins. This is a practical visual match, not a claim that Codex uses Lucide. Use a single shared icon wrapper to control size, stroke, and color. Custom SVGs must follow the same geometry and stroke treatment. Do not mix emoji, multicolor illustrations, solid dashboard icon packs, and line icons for routine actions.

| Property | Rule |
| --- | --- |
| Drawing grid | 24 × 24 SVG viewBox |
| Default stroke | 1.75 units; adjust within 1.5–2 only for optical balance |
| Caps and joins | Round; no square line endings |
| Toolbar and sidebar glyph | 18 px |
| Main rail glyph, if a rail is present | 20 px |
| Inline file or status glyph | 16 px |
| Default color | `--text-secondary`; inherit `currentColor` |
| Hover and selected color | `--text-primary` |
| Accent color | `--accent-text` for linked artifacts and accent actions |
| Filled exception | Selected Home or similarly clear active destination; use a matching custom variant if needed |
| Button background | Transparent at rest; neutral hover or selected surface |
| Target | At least 32 × 32 px on desktop, 44 × 44 px for touch |

Do not make every active icon purple or place each glyph in a colored tile. In the screenshot, the active navigation container provides most of the selection cue. An icon's visual size and its clickable area are separate dimensions.

| Kiko action | Starting Lucide glyph | Treatment |
| --- | --- | --- |
| New chat | `SquarePen` | Outline, next to “New chat” |
| Project | `Folder` / `FolderOpen` | Outline; open state when expanded |
| Search | `Search` | Outline |
| Toggle sidebar | `PanelLeft` | Outline |
| Toggle document pane | `PanelRight` | Outline |
| Recent chats | `History` | Outline |
| Document | `FileText` | Outline; accent when linked in prose |
| More actions | `Ellipsis` | Horizontal dots |
| Add | `Plus` | Simple cross |
| Send | `ArrowUp` | White glyph on a purple circular button |
| Stop | `Square` | Compact filled square inside the action button |
| Export | `Download` | Outline with a text label in document toolbar |
| Undo / redo | `Undo2` / `Redo2` | Matching outline pair |
| Version history | `History` | Outline with accessible context |
| Comment | `MessageSquare` | Outline |
| Settings | `Settings2` | Outline |
| Provider key | `KeyRound` | Outline |

Render only actions Kiko supports in the current release. The reference microphone and integration icons do not add voice or integrations to v0.1. Do not copy product-specific logos as generic navigation symbols.

Give icon buttons action labels such as “Open document” and “Collapse sidebar,” with tooltips on hover and keyboard focus. Hide decorative SVGs from assistive technology. A tooltip supplements the accessible name; it does not replace it.

## Spacing and shape

Use a 4 px base scale: **4, 8, 12, 16, 20, 24, 32, 40, 48, 64**. A 2 px adjustment is allowed for optical alignment. Standard icon-to-label gap is 8 px. Group related controls 4–8 px apart; separate groups by 16–24 px.

| Element | Radius | Typical size or padding |
| --- | --- | --- |
| Compact control | 6 px | 28 px tall; 8 px horizontal padding |
| Standard button / field | 8 px | 36 px tall; 12 px horizontal padding |
| Navigation row | 10 px | 36 px minimum; 8–12 px horizontal padding |
| Artifact card / popover | 12 px | 12–16 px padding |
| Dialog | 16 px | 24 px padding |
| User message | 20 px | 12 px vertical, 16 px horizontal |
| Composer | 24 px | 16 px padding; 12 px gap to footer controls |
| Circular action | 999 px | 32 px desktop diameter; larger touch target |

Nested elements use proportionally smaller radii. Keep equal padding on equal edges. Do not give every surface the same pill shape.

## Desktop arrangement

The user's [screen reference index](./references/screens/README.md) records the supplied layouts and startup flow. Those examples establish a compact rail, an expandable navigation drawer, a centered composer before chatting, and a right document pane. Their styling is replaced with this system. The defaults below fill in dimensions and behavior where the examples leave them open; layout variants in the index remain alternatives until chosen.

```text
┌──────────────────────────────────────────────────────────────────────┐
│ Native window toolbar · current chat / document · pane controls     │
├──────┬─────────────────┬─────────────────────────────────────────────┤
│ Rail │ Project sidebar │ Conversation                               │
│ *    │                 │                                             │
│      │ New chat        │                                  User text  │
│      │ Projects        │ Assistant text and compact tool activity    │
│      │ Recent chats    │                                             │
│      │                 │                  Composer                   │
└──────┴─────────────────┴─────────────────────────────────────────────┘
* Optional: include only when Kiko has enough distinct destinations.

Document open:
┌───────────────────────┬──────────────────┬───────────────────────────┐
│ Project sidebar       │ Conversation     │ Document toolbar          │
│                       │                  │                           │
│                       │ Tool activity    │ Editable document         │
│                       │ and messages     │ Pending sections          │
│                       │                  │ Anchored questions        │
│                       │ Composer         │                           │
└───────────────────────┴──────────────────┴───────────────────────────┘
```

| Region | Default guidance |
| --- | --- |
| Native toolbar | About 48 px logical height; respect platform window controls |
| Optional app rail | 52 px wide; keep utility actions near the bottom |
| Project sidebar | 260 px initial width; resizable from 220–320 px |
| Sidebar inset | 12 px at edges, 16–24 px between named groups |
| Chat without document | Center a column up to 880 px wide, with at least 24 px side padding |
| Chat with document | Start at roughly 40% of remaining width; document gets 60% |
| Conversation minimum | 360 px when both panes are visible |
| Document minimum | 480 px when both panes are visible |
| Document reading column | Around 680 px, centered with at least 32 px inner padding |
| Composer | Align to conversation text column; 16–24 px above pane bottom |
| Split divider | 1 px visual boundary with an 8 px hit area |

The sidebar, conversation, and document each scroll independently. Keep the composer inside its conversation pane and reserve space beneath messages for it. Opening a document must not leave the composer floating across both panes. Prefer flexible widths over fixed screenshot proportions.

When minimum pane widths cannot fit, collapse the sidebar first. If the conversation and document still cannot fit, switch to one primary pane with a clear Chat / Document control and preserved scroll positions. At narrow widths use 16 px gutters, full-width editors, and questions below their passages. Future mobile layouts use 44 px touch targets and respect the on-screen keyboard.

## Component behavior and appearance

### Navigation

Place the Kiko name, New chat, project groups, and recent chats in a stable vertical order. Use muted group labels with ordinary sentence case. Rows have transparent backgrounds at rest, `--bg-hover` on hover, and `--bg-selected` when selected. Give the current row a persistent visible cue and expose `aria-current`. Keep long titles on one line with ellipsis and reveal the full title on focus or hover. Row actions appear on hover, keyboard focus, or selection and remain reachable by keyboard.

### Conversation

User messages sit on the right in the purple message surface, up to 75% of the column width; long messages wrap naturally. Assistant replies sit on the workspace without a bubble. Leave 24–32 px between message groups. Use quiet metadata and action icons beneath the relevant message. Message tools remain available on keyboard focus and touch. Code and tables may scroll horizontally inside their own containers; prose must wrap.

### Composer

The composer is the strongest neutral container: `--bg-composer`, a 24 px radius, and a subtle edge. Text occupies the first row; supported add actions and the model selector share a footer, with Send aligned to the lower right. Use the provider and model's user-facing names. A one-line draft starts around 96 px total height and grows to roughly 240 px before its text area scrolls.

Use a purple circular Send button when sending is available. Empty input has an explicitly disabled Send state. During generation, the same action slot becomes Stop. Enter sends, Shift+Enter adds a line, and input-method composition must finish before Enter can send. Provide focus feedback for the whole composer without drawing an oversized glowing outline around it.

### Buttons, fields, menus, and dialogs

Primary buttons use the solid accent with white text; use one primary action per local task. Secondary buttons use neutral surfaces, and tertiary actions are plain text or icon buttons. Every button has distinct default, hover, pressed, disabled, and focus-visible states. Disabled controls are inert and visually subdued; explain the reason nearby when it is not obvious.

Fields have visible labels and supporting or error text where needed. Standard fields use the input surface and an identifiable boundary. Menus and dialogs use raised surfaces, small icons, comfortable rows, and an anchored origin. Focus moves into a modal and returns to its trigger on close. Escape dismisses the top dismissible layer. Destructive actions use danger text and a precise verb such as “Delete document.”

### Tool activity and artifacts

Show tools as compact lines with an outline icon, plain-language label, and state. The current step may have a small spinner. Completed work folds into a neutral summary such as “Used 4 tools · 18 s.” Expanding reveals activity details. Label steps with user-facing actions: “Writing: The problem,” “Reading document,” or “Saved changes.” Tool refusals explain what changed and what the agent is doing next.

Artifact cards are compact horizontal objects with a file icon, title, and optional short status. Clicking one opens its document. Use neutral surfaces and accent file icons; avoid large decorative thumbnails for text documents. Expanded technical arguments belong in a details view.

### Document editor

Use the workspace surface and system sans typography. Keep the title and body on a stable reading column, with a compact toolbar for Export, Undo, and Version history. Pending sections use the pending surface, a subtle radius, and their intended content in readable secondary text. During writing, a caret and short status identify the current section; completed sections become ordinary editable content.

Keep user text selections intact while the agent writes elsewhere. Avoid inserting layout animation between paragraphs. Show “Saving…”, “Saved”, or a useful failure message near the document toolbar without shifting its layout. Export opens a small format menu containing the formats supported by that release.

### Anchored questions

Give the passage a subtle purple highlight and pair it with an explicit comment marker. The question card uses a neutral raised surface with its short question and two or three outlined reply buttons. On wide document panes it may occupy a margin; when space is tight it sits beneath the relevant paragraph. Clicking a reply shows progress, then the resolved state. Preserve a keyboard path from the passage to the question and back.

### Empty, loading, and failure states

An empty conversation has one short prompt, such as “What would you like to plan?”, and a ready composer. An empty project has a clear New chat action. Loading preserves the existing layout; a tool label or small spinner explains ongoing work. Keep errors close to the affected action and preserve the user's draft. “Couldn't reach Muse. Check your connection and try again.” is useful; a raw stack trace is not interface copy.

## Motion and input

| Interaction | Starting behavior |
| --- | --- |
| Hover / pressed color | Immediate press response; 100–140 ms hover transition |
| Menu / tooltip | 120–160 ms opacity; at most 4 px movement from its trigger |
| Pane open / close | About 200–240 ms; no bounce; reverse from current position |
| Divider drag | Track the pointer directly; no animation lag |
| Expand activity | About 160–200 ms; preserve the reading position |
| Streaming text | Append naturally; no per-character entrance effects |

Use reversible transitions and restore spatial relationships on close. While generation runs, auto-scroll only if the user was already at the bottom. If they scroll away, show a quiet jump-to-latest control. Stop responds immediately and preserves completed work.

Respect reduced motion by removing sliding, spring travel, and animated expansion; retain state changes and short fades. Any translucent treatment has a solid version for reduced transparency. Do not lock input during an animation.

## Accessibility and implementation contract

- Keep normal text at a contrast ratio of at least 4.5:1 and essential large text or control graphics at least 3:1 against their actual adjacent surface. Subtle decorative separators are not the sole boundary of an interactive control. Validate new token pairings as components are implemented.
- Use a 2 px `--focus-ring` outline with 2 px offset for keyboard focus. Never rely on hover alone. Text, status labels, and shapes accompany color cues.
- Keep tool updates from repeatedly interrupting screen readers. Announce meaningful turn, save, and error states through a polite status region. Expose expanded, selected, busy, and disabled states semantically.
- The pane divider supports pointer dragging and keyboard resizing. Navigation, menus, dialogs, composer controls, and anchored questions work without a mouse.
- Use semantic CSS tokens from the color table. Components must not embed ad hoc hex values. Put theme values on the app root and map them into Tailwind CSS v4 with `@theme inline`.
- Implement shared Button, IconButton, Icon, NavRow, Composer, ToolStep, ArtifactCard, PaneHeader, Menu, Dialog, and QuestionCard primitives before styling individual screens independently.

Example token use:

```css
@theme inline {
  --color-workspace: var(--bg-workspace);
  --color-sidebar: var(--bg-sidebar);
  --color-composer: var(--bg-composer);
  --color-foreground: var(--text-primary);
  --color-muted: var(--text-muted);
  --color-accent: var(--accent-solid);
}

.icon {
  width: 1.125rem;
  height: 1.125rem;
  flex: none;
  stroke-width: 1.75;
  stroke-linecap: round;
  stroke-linejoin: round;
}

:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 2px;
}
```

The color table supplies the variables for each root theme; this snippet shows their consumption rather than defining a second palette.

## Applying the system to supplied screens

1. Identify the screen's purpose, content hierarchy, pane arrangement, and required actions.
2. Preserve that approved arrangement, adapting dimensions to Kiko's desktop density and minimum pane widths.
3. Replace the screen's palette with these semantic tokens, its typography with this type scale, and its icons with the shared rounded outline family.
4. Build its controls from the shared primitives. Define hover, selected, focus, disabled, loading, empty, and failure states wherever applicable.
5. Compare the result with the saved Codex reference for surface hierarchy, icon weight, spacing, and restraint. Check the screen at its intended size, with long content, enlarged text, and narrower panes.

The target is the same calm, sleek desktop character across every Kiko screen, with each example contributing its intended layout.
