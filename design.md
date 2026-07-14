# Project Template Design System

## Direction

Project Template is an operational product interface. It should feel quiet, precise, and efficient under repeated use. Information hierarchy, interaction feedback, and complete states create richness; decoration does not.

The visual language uses neutral surfaces, a restrained green primary action color, a cool blue reference color, and semantic status colors. It supports dark and light modes with equal hierarchy. It does not use gradients, decorative orbs, oversized marketing typography, heavy shadows, or floating page-section cards.

`packages/ui/src/styles/tokens.css` is the implementation source of truth. Browser surfaces consume semantic `--pt-*` roles and do not invent local palettes.

## Token Roles

Tokens cover these roles in both themes:

- Background: page, subtle, surface, raised, inset, overlay.
- Text: primary, muted, subtle, disabled, on-accent.
- Border: default, strong, control, selected.
- Action: primary, primary-hover, secondary, secondary-hover, destructive.
- State: success, warning, critical, info and their muted surfaces.
- Interaction: focus outline, selection, disabled opacity, transition duration.
- Shape and space: control heights, spacing scale, radii no larger than 8px.
- Typography: sans, display, and mono families with explicit weights and line heights.

Raw colors are allowed only in the token source, browser metadata, static assets, test fixtures, or vendor/canvas bridges that cannot consume CSS variables. Every exception is path-specific and documented in `scripts/check-frontend-style.mjs`.

## Layout

Application screens use a stable product shell with predictable navigation and dense, scannable content. Page sections are unframed bands. Cards are reserved for repeated entities, modals, and genuinely framed tools; cards do not contain decorative nested cards.

Tables, boards, toolbars, counters, and fixed-format controls declare stable dimensions with grid tracks, aspect ratio, or min/max constraints. Dynamic labels, loading indicators, and hover states must not shift adjacent layout.

Text never overlaps controls or adjacent content. Long names, email addresses, and task titles wrap or truncate with an accessible full-value affordance. Font sizes do not scale continuously with viewport width; responsive breakpoints select deliberate sizes.

Reference widths are 320, 608, 928, and 1440 pixels. Mobile navigation remains task-complete. Do not hide required commands simply to make a narrow screenshot clean.

## Components

Shared primitives include Button, IconButton, Field, Dialog, Drawer, DataTable, EmptyState, Tabs, Toast, Panel, PageShell, and Spinner. Build complex behavior on established headless primitives.

Use icons for familiar tools and compact commands. Icon-only controls require an accessible name, and unfamiliar icons require a tooltip. Use segmented controls for small mutually exclusive modes, toggles or checkboxes for binary settings, and menus for larger option sets.

Feature workflows stay in the web feature that owns them. A `TaskEditor` or `WorkspaceMemberTable` is not a shared UI primitive.

## Interaction And Accessibility

- Every control works by keyboard and has visible `:focus-visible` treatment.
- Dialogs and drawers trap focus, close by documented escape behavior, and restore focus to their trigger.
- Form labels, descriptions, validation messages, and errors are programmatically related.
- Color never carries status alone.
- Motion honors `prefers-reduced-motion` and remains short and functional.
- Serious and critical axe violations block CI.
- Loading, empty, error, forbidden, disabled, destructive-confirmation, and success states are designed with the primary state.

## Reference Slice

The workspace and task screens prove the system. They include role-aware commands, list and detail views, create/edit/delete flows, filters in the URL, optimistic or explicit cache reconciliation, long-content behavior, and responsive navigation. Storybook documents every public primitive independently from those workflows.

## Review Checklist

1. The UI uses semantic tokens and both themes retain hierarchy.
2. Keyboard focus, accessible names, dialogs, and form errors work.
3. Loading, empty, error, forbidden, and destructive states are complete.
4. Layout remains stable at all reference widths and with long content.
5. Shared primitives are domain-neutral and have behavior-focused stories/tests.
6. No gradient, decorative blob, nested-card composition, or raw local palette was added.
