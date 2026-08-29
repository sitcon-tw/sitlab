# Material Design conversion — handover

> Historical note: this document records the original strict MD3 conversion. The current approved visual direction keeps the component and accessibility work while adapting Simfiment's warm neutral surfaces, Inter typography, stronger tokenized hierarchy, and rounded containers. SITCON green remains primary, and all elevation shadows now resolve to `none`; tonal layering, spacing, and state layers provide separation in both themes, while visible outlines are reserved for functional controls and states. `design.md` and `tokens.css` are authoritative when a historical statement below conflicts.

Status on branch `sync-engine-foundations`; the last committed conversion phase is `778649c`.

The web UI is being rebuilt against the Material Design 3 specification. **P1–P5a are committed. P5b and P6 are implemented in the working tree but remain uncommitted and still require the final browser verification loop.**

---

## Why this exists

An earlier pass claimed to convert the product to MD3 but only converted
_tokens_, not _components_. The measured evidence at that point:

- The whole of `web/` referenced **3** `.md-*` classes. Everything else was
  bespoke CSS that merely read `--md-sys-*` variables.
- **No board control had a ripple or a state layer** — about six elements in the
  entire product went through an MD3 component.
- 29 hard-coded font weights (650–850). Material's type scale has only 400 and 500.
- 84 hard-coded font sizes, including `0.8125rem` and `0.625rem`, which are not
  on the Material scale at all.
- 37 uses of a 4px radius where Material specifies a pill.
- 45 hand-rolled focus rings that `md3.css` already provided.
- 14 primitives in `packages/ui` had been built and never used.
- The palette was seeded from the brand green with neutral chroma 6, so **every
  surface was tinted green** (`#edefe5`). Material's surfaces are near-neutral.

So the work is not "finish the MD3 migration" — it is "make the product actually
use the primitives, and delete the CSS that duplicates them".

---

## The specification being implemented

`packages/ui/src/styles/tokens.css` opens with the authoritative spec block.
**Read it before writing any CSS.** It is the single source of truth and it is
deliberately not repeated in full here. Summary:

- **Shape** — Material Web ships the **7-step** scale (`none 0` / `extra-small 4`
  / `small 8` / `medium 12` / `large 16` / `extra-large 28` / `full`). The
  10-step scale with 20/32/48dp is the Android-only _M3 Expressive_ update and is
  deliberately **not** used. Per component: button/icon button/segmented/badge/
  avatar `full`; chip `small`; card `medium`; lane top edge and side sheet
  leading edge `large`; dialog `extra-large`; text field/menu/snackbar
  `extra-small`; list item and top app bar `none`.
- **Size** — button 40dp (24dp inline padding, 16dp on a leading-icon side);
  icon button 40dp visual / **48dp touch** via a non-layout `::after`; chip 32dp;
  text field 56dp with a **3px** focus outline; list item 56dp / 72dp two-line;
  menu item 48dp; top app bar 64dp; dialog 24dp padding.
- **Type** — Roboto, **only weights 400 and 500**, sizes only from the 15 type
  scale roles.
- **State** — hover .08 / focus .10 / pressed .10 / dragged .16, applied as the
  surface's own content colour via `background-color: currentColor`.
- **Elevation** — card level1, hover level2, dragged level3, drag preview level4,
  dialog level3, menu level2, snackbar level3. **Elevated cards carry no border**;
  layering is elevation alone.
- **Focus** — 3dp `secondary` outline at 2dp offset, provided once by `md3.css`.
  Never declare one in a feature stylesheet.

### Decisions already made with the user

| Question        | Decision                                                                                                                                                                                                              |
| --------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Palette         | Brand green stays the seed, but **primary chroma 48** and **neutral chroma 2** so surfaces are neutral and green appears only where it should. Regenerate with `pnpm --filter @project-template/ui generate:tokens`.  |
| Density         | Controls fully Material. **56dp fields only in dialogs and the drawer**; board chrome uses Material's own **density -2 (48dp)** via `<TextField dense>`. 56dp everywhere pushes the board past the viewport at 320px. |
| Header          | Material top app bar on `surface`, raising to `surface-container` + level2 on scroll.                                                                                                                                 |
| Typeface        | Roboto, self-hosted (`@fontsource-variable/roboto`) — this is an internal tool and must not make third-party requests.                                                                                                |
| Corners etc.    | Everything unified onto the scales above; no stylesheet hand-writes a radius, size, weight, shadow or focus ring.                                                                                                     |
| Reference image | The user supplied a dark agent-console screenshot (12px fields, bordered containers). When asked, they chose **spec over the reference** on both points: fields stay 4px, cards stay borderless with elevation.       |

---

## What is done (committed)

| Commit    | Phase | Content                                                                                                                                                                                                              |
| --------- | ----- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `0125485` | P1    | Palette rebuilt (neutral chroma 2, primary 48), Roboto, spec block written into `tokens.css`, text-field focus outline 2px → **3px** (was wrong).                                                                    |
| `36e7b71` | P2    | `Menu` primitive (Radix `DropdownMenu`) and `asChild` on `Button`/`IconButton`. **See the warning below about this commit.**                                                                                         |
| `bf770cd` | P3    | Header → `TopAppBar`; inline `SitconLogo` using `currentColor`; account popover → `Menu`; quick-create and filter rows → fields, filter chips, icon buttons; undo banner → `ToastRegion`. ~390 lines of CSS deleted. |
| `3d34a38` | P4    | Card and lane: icon buttons at 32/48dp, type scale, `Badge` lane counts, card border removed, date on the chip scale, label chips restored to 32dp. ~64 lines deleted.                                               |
| `778649c` | P5a   | Card detail drawer: all controls → 56dp outlined fields; label picker `<details>` → dialog; comment composer → text area field; comment count/system marker → `Badge`/chip. ~140 lines deleted.                      |

Verified at each step: `pnpm check:frontend-style`, `just typecheck`, `just
ui-lint`, `just web-lint`, `just ui-test` (12), `just web-test` (71), Playwright
(26 passing / 2 skipped), and no horizontal overflow at 320/608/928/1440 in both
themes.

### Two real bugs found and fixed along the way

1. An earlier edit had merged the drawer's textarea rule onto the input and
   select selectors, so `min-height: 10rem` applied to all of them and **every
   56dp field rendered at 160px**. Caused by a scripted CSS edit that removed a
   rule body but left its multi-line selector list dangling — see the warning
   below.
2. `.detailGrid` stretched its grid items, pulling every field to the tallest row.

---

## What is in progress (uncommitted)

P5b and P6 implementation is complete in the working tree:

- Assignee, member, and label pickers use `Chip`, `TextField`, `IconButton`, native `md-checkbox` controls, Material list rows, dialog footers, and shared empty states.
- The member drawer uses Material lists; native Quick Action autocomplete in description and Comment fields uses the Material menu classes; the directory warning uses `Panel` and `Button`.
- Label management uses Material fields and list rows, color targets are 40dp visual / 48dp touch, and save feedback uses the shared `Spinner`.
- Login uses `Button asChild`. Onboarding uses `TopAppBar`, outlined `Panel`, native `md-radio`, and Material lists.
- Feature focus rings were removed; `md3.css` now provides the 3dp focus indicator once for native and primitive controls.
- Startup typography uses `.md-typescale-*`; `design.md`, the UI-system reference, and root ownership guidance now point back to the authoritative `tokens.css` specification.

Current local checks: web typecheck, web lint, UI lint, web tests (80), and UI tests (12) pass. `pnpm check:frontend-style` is temporarily blocked by another session's uncommitted `web/src/features/board/syncActions.test.ts`, which contains raw color fixtures; do not fold that file into this conversion.

### Remaining product roles in `web/`

```
16  --sb-text-subtle     documented third text tier with AA contrast
 5  --sb-control         }
 4  --sb-surface-subtle  } scheme-specific board surface roles
 3  --sb-surface         }
 1  --sb-page            }
 1  --sb-lane            }
 2  --sb-accent          }
 1  --sb-coral           } SITCON product colors without Material equivalents
 1  --sb-blue            }
```

No obsolete `--sb-surface-raised` use remains. `BoardPage.module.css` is down from 2150 to about 1300 lines; remaining CSS primarily owns board layout, Markdown content, responsive composition, and documented product surfaces.

---

## What remains

1. Run the full verification loop below, including `just ui-build` before browser work.
2. Run Playwright and manually review 320 / 608 / 928 / 1440 in both themes, reduced motion, keyboard focus, and Escape focus restoration.
3. Refresh the affected desktop and mobile screenshots under `docs/assets`.
4. Resolve or exclude the concurrent `syncActions.test.ts` style-policy blocker without editing or staging that other session's work.

---

## Warnings for whoever picks this up

### 1. Do not use `git add -A` in this tree

Another session is doing backend work in `server/`, `api/` and
`server/db/migrations/` **in the same working tree**. Commit `36e7b71` swept up
nine of their in-progress files because it used `git add -A`, which left that
commit unbuildable for the integration suite. Their user has decided to leave the
history as it is.

Stage exact files, not directory globs. The concurrent sync work now also has an uncommitted `web/src/features/board/syncActions.test.ts`, so even `git add web/src` is unsafe. Nothing in this workstream touches `server/`, `api/`, `docs/public/`, or `syncActions.test.ts`; if a staged diff contains them, something is wrong.

### 2. The branch is not `main`

`HEAD` is on `sync-engine-foundations`, which carries the other session's backend
commits. All of this Material Design work landed there by accident — the earlier
session never checked the branch. The user chose to leave it. Check
`git branch --show-current` before doing anything with history.

### 3. Never remove a CSS rule by scripted text-slicing

The 160px-field bug came from a helper that removed a rule body by finding
`\n<selector> {` and cutting to the next `\n}`. When the selector was part of a
multi-line list, it left the other selectors dangling and silently merged them
onto the _next_ rule's body. If you script CSS deletions, verify afterwards:

```bash
node -e 'const s=require("fs").readFileSync(F,"utf8");const o=(s.match(/\{/g)||[]).length,c=(s.match(/\}/g)||[]).length;console.log(o===c?"BALANCED":"MISMATCH")'
```

Brace balance alone does not catch it — also grep for a selector line ending in
`,` followed by a blank line.

### 4. Test contracts that must survive markup changes

Breaking any of these turns the suites red:

- **Native `<checkbox>` / `<radio>`** — `getByRole`, `toBeChecked`, and `toBePartiallyChecked`. Material styles the native input, so this is compatible; do not swap in a div-based widget.
- Lanes stay `<section data-list="…">`; the card title stays an `<h3>` inside a
  button.
- The aria-labels `"Close dialog"` and `"Close drawer"`.
- The `dragPreview` class name (`[class*="dragPreview"]` in e2e).
- `main.sb-startup-error` and `.sb-brand`.
- `Start` / `Due` aria-labels, and the `SaveIndicator` `title` values
  (`Due儲存中`).
- **`documentWidth <= viewport` at 320×720** — this is why 56dp fields are
  confined to dialogs and the drawer.

Where a control genuinely has to change shape, update the tests rather than
contorting the markup — that was done for the team filter, which became a filter
chip plus a menu (2 e2e assertions, 3 unit assertions).

### 5. `just backend-test` fails on this machine, unrelated to any of this

The gitignored `.env` is loaded by `just`'s `dotenv-load` and leaks into the Go
config test. `cd server && go test ./...` is green. This reproduces on a clean
checkout.

---

## Verification loop

```bash
pnpm check:frontend-style && just typecheck
just ui-lint && just web-lint
just ui-test && just web-test
just ui-build                                    # web/ imports from packages/ui/dist
VITE_SITCON_DEMO=true pnpm --filter @project-template/web dev --port 5199
E2E_DEMO=true E2E_BASE_URL=http://localhost:5199 pnpm --filter @project-template/web exec playwright test
```

`just ui-build` matters: `web/` imports the built stylesheet, so a change to
`packages/ui/src/styles/*.css` is invisible in the browser until you rebuild.

Manual pass at 320 / 608 / 928 / 1440, both themes:

1. Every actionable surface shows a ripple and a state layer.
2. Buttons, icon buttons, chips and segmented controls are pills; cards 12dp,
   lane top edge 16dp, dialogs 28dp, fields and menus 4dp.
3. No font weight outside 400/500 and no size off the type scale.
4. No horizontal overflow at 320.
5. Under `prefers-reduced-motion` the ripple is gone but the pressed state layer
   remains.
6. Keyboard-only pass board → drawer → dialog; every stop shows the 3dp focus
   indicator and Escape restores focus to the trigger.
