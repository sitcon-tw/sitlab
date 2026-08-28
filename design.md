# SITCON Board Design System

## Product direction

SITCON Board is a daily operational workspace for the organizing team. The interface must stay quiet, compact, and easy to scan; information and action feedback take priority over decoration. The first viewport identifies `SITCON / 2027` and exposes an actionable board without a marketing hero, gradients, decorative artwork, or a permanent sidebar.

The visual character is adapted from Simfiment: warm neutral canvases, crisp layered surfaces, rounded containers, compact Inter typography, and restrained use of color. SITCON green remains the product primary and the official product bar remains the strongest brand surface. The interface is deliberately shadow-free.

The product uses **Material Design 3 as a foundation, not as its visual target**. Material supplies semantic color roles, accessibility behavior, interaction states, and a coherent sizing vocabulary. Simfiment is the visual reference for surface hierarchy, compact density, rounded controls, typography, and restraint. When the two disagree visually, use the documented SITCON product role or deliberate deviation instead of reproducing a stock Material component. `packages/ui/src/styles/tokens.css` remains the browser design authority.

## Ownership and conformance

Browser styling has three layers:

1. `--md-ref-palette-*` tonal ramps generated from the brand seed. Regenerate them with `pnpm --filter @project-template/ui generate:tokens`.
2. `--md-sys-*` system roles consumed by primitives and product CSS.
3. A small set of `--sb-*` product roles for surfaces or brand meanings that Material does not model directly.

`tokens.css` is the only stylesheet allowed to contain concrete colors. `packages/ui/src/styles/md3.css` owns reusable primitive styling, the global 3dp focus indicator, state layers, and motion behavior. Feature stylesheets may arrange primitives and map product roles, but must not redraw a primitive, introduce a new type size or weight, or declare a competing focus ring.

Use the 7-step Material Web shape scale documented in `tokens.css`; the Android-only 20/32/48dp M3 Expressive additions are not part of this product. Typography uses self-hosted Inter Variable with system CJK fallbacks and keeps all sizes on the 15 Material type roles; product headings and labels use the documented 600/700 strong roles. Actionable surfaces use their own content color for hover, focus, pressed, and dragged state layers. Surface tone, spacing, and state layers communicate hierarchy; browser UI must not use decorative borders or shadows. Visible outlines are reserved for controls that need an edge, keyboard focus, drag targets, semantic errors, color swatches, and tabular content.

## Product roles

- Board page, lane, card, and compact-control surfaces remain product roles. Light mode uses a warm near-white canvas and bright cards; dark mode uses a near-black canvas and progressively lighter charcoal surfaces. Tonal contrast keeps adjacent layers legible without decorative borders or shadows.
- Green is the Material primary seed. Secondary supplies focus and selected-container roles; tertiary supplies informational and Inbox roles.
- Warning is a product color group because Material 3 has no warning role. Error represents failures and overdue work.
- `--sb-text-subtle` remains a deliberate third text tier. Material has no equivalent, and `outline` alone is below AA contrast on the product surface. Use it only for supporting metadata, never primary content.
- SITCON accent, coral, and blue remain product colors with no Material semantic equivalent.
- The dark product bar uses the official white SITCON logo from the `sitcon-tw/2027` source. Product code does not redraw or recolor the mark.

## Deliberate Material deviations

Every deviation requires a product reason and must be recorded here.

1. **The product top app bar remains on the SITCON ink surface in both themes.** This preserves event identity instead of using the default Material surface role.
2. **The snackbar region may stack messages.** Material normally presents one snackbar; the board must expose multiple independent mutation failures without dropping any of them.
3. **The ripple uses a single 500ms expansion and fade.** Material's two-stage release animation is simplified while retaining the required state layer and reduced-motion behavior.
4. **Form controls follow the Simfiment product treatment rather than stock Material fields.** Text, date, textarea, and single-choice fields use a compact charcoal/neutral control surface with a quiet, uninterrupted outline instead of Material's filled underline or notched outline. Floating labels stay inside the control so different parent and field tones never create a visible background patch above the edge. SITCON keeps its softer 12px rounding on all four corners rather than copying Simfiment's top-only field corners. Every single-choice popup uses the same accessible, shadow-free menu surface as board sorting so browser-native menus cannot introduce an inconsistent appearance.
5. **Dense board chrome uses the supported -2 field density.** Full 56dp fields remain in dialogs and drawers; dense sorting and creation controls use 48dp fields so the 320px board remains operable.
6. **Board page, lane, card, and compact-control surfaces are scheme-specific product roles.** Their light and dark mappings intentionally differ as described above.
7. **`--sb-text-subtle` is retained for low-emphasis metadata.** Replacing it with `outline` would fail contrast; replacing every use with the same Material text role would remove necessary hierarchy.
8. **Product typography uses Inter and strong 600/700 roles.** This carries the compact hierarchy of the approved Simfiment reference while retaining Material's named size and line-height roles.
9. **The browser interface is shadow-free and border-light.** Cards, menus, dialogs, drawers, snackbars, and sticky chrome use tonal layering and spacing instead of Material elevation shadows or decorative outlines. Fields, selection controls, focus, drag targets, semantic errors, color swatches, and tables retain the edges needed to remain operable and legible.

## Product layout

The fixed product bar contains identity, the member drawer trigger, offline state when needed, and the account menu. Quick Create is one row on desktop and a title-plus-controls layout on mobile. Its segmented mode switches between one team and all team leads; additional options cover status, description, and searchable project labels. New cards default to Inbox and appear at the top of the target lane.

The desktop filter row keeps compact searchable team, grouped multi-member, and all-labels selectors beside card search; label administration sits at the far edge. Due date near-to-far is the default Board sort. At 928px and below, card search stays visible while sorting and the three advanced selectors move into one immediate-apply dialog. A segmented control switches between the default Board and a Gantt view while preserving those filters. Lanes stay ordered as Waiting, Inbox, To do, Doing, Review, and Done. The Gantt view groups open Issues by team, offers day and Monday-to-Sunday week scales, and keeps a sticky Issue column and timeline header on fully opaque surfaces so scrolled grid lines never show through. Day is the default scale; week is stored in the query string when selected. Narrow screens scroll the active visualization horizontally instead of compressing lanes, rows, or dates until they collide.

The complete directory appears only in the right-side drawer. It is a side sheet on desktop and may occupy the full width on mobile. Do not add a permanent sidebar or duplicate the full directory on the main board.

## Interaction contract

- Production starts from injected bootstrap data. It does not replace the board with a loading page, skeleton, spinner, or empty columns.
- Healthy background refresh, pending work, processing, and success stay visually quiet. A field the user just edited may show an in-place saving indicator that resolves to a brief saved marker. One drawer-level live region announces failures without stealing focus.
- Card creation and card mutations update optimistically. Durable failures preserve user intent and expose Retry.
- Gantt is a read-only scheduling view over bootstrap data and excludes cards in lists marked closed. Complete Start/Due pairs render as inclusive ranges, Due-only and Start-only cards render as distinct markers, invalid date order remains visibly invalid, and cards without either date remain in a grouped Unscheduled section. Activating a row opens the existing detail drawer for edits; optimistic date and status changes immediately reflow or remove the row.
- Dragging supports pointer, touch, and keyboard movement. Manual sorting permits reordering within and across lanes, with visible filtered cards acting as insertion anchors while hidden cards preserve relative order. Due, Start, and Updated sorting reject same-lane reordering; a cross-lane drop changes only the GitLab lifecycle status and keeps the selected presentation sort. The detail drawer provides a keyboard alternative for changing lanes.
- The detail drawer owns lifecycle state, team, title, GFM Markdown description, multiple assignees, GitLab Start and Due dates, labels, GitLab-native Child items and Linked items, typed Quick Actions, comments, and the issue link. Markdown preview never enables raw HTML.
- Child and Linked item sections use fixed status, assignee, date, and label metadata with no persisted display-options control. They collapse independently for the current drawer session. Creating and attaching Child Tasks and adding Issue/Task links use dialogs with debounced title or IID search; Linked candidates use native checkboxes, retain selections across searches, and submit one batch. Relation failures remain inline and preserve the entered value. A related Issue already present in bootstrap opens in the local drawer even when filters hide it, while Tasks and unboarded Issues open GitLab in a new tab. Detaching a child never presents itself as deleting the Task. After confirmation, detach and unlink remove the row optimistically while the request runs in the background, then restore it with an inline error if GitLab rejects the mutation.
- Assignee and member pickers group members by directory team. Group-heading native checkboxes affect the currently visible members, preserve indeterminate state, and do not alter hidden search results.
- Team selection uses native radios. Label selection uses native checkboxes. Styling native controls is required; div-based replacements are not acceptable.
- Card search covers title, description, issue IID, team, assignee identity, and labels. Whitespace-separated terms use case-insensitive AND matching across those fields, and filtering waits briefly for typing to settle before updating the board.
- View, search, sort, and filter state is stored in the query string with history replacement so reloads and shared URLs restore the same view. Board and Due date near-to-far are omitted as defaults; manual ordering and Gantt are stored explicitly. Board sorting is preserved but hidden while Gantt is active.
- Dialogs and drawers trap focus, close with Escape, and restore focus to their trigger. Icon-only controls have an accessible name and a tooltip.
- Avatar initials render immediately; a successfully loaded image fades in without layout shift, and a failed image never exposes a broken-image indicator.

## Responsive and accessibility review

Review 320, 608, 928, and 1440 pixel widths in both themes. At every width verify:

1. The product bar and Quick Create controls do not overlap.
2. Titles, member names, metadata, and errors remain inside their containers.
3. The board and Gantt timeline scroll internally without compressing lanes, rows, cards, or dates into unreadable layouts.
4. Member, assignee, label, comment, and account surfaces remain fully operable.
5. Keyboard focus shows the 3dp secondary indicator and color is never the only state signal.
6. Every actionable Material surface shows its state layer and ripple.
7. Under `prefers-reduced-motion`, ripple and transitions disappear while the pressed state layer remains.
8. `documentWidth` does not exceed the 320px viewport outside the intentional board scroller.
