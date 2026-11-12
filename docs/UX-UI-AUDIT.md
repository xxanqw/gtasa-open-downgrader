# UX/UI audit

Reviewed revision: `4f2d8b2`. Findings and source line references below describe that baseline.

## Implementation follow-up

The follow-up implementation addresses UX-01 through UX-10 with regression tests:

- Cancellation keeps multiplayer cards; confirmations use explicit action labels and retain safe Cancel focus.
- The shared footer preserves full operation text during work and interruption warnings across verification. Routine successes use the existing status bar, not a persistent banner. Browser waits support reopening the page and changing watched folders without weakening archive verification.
- Automatic launcher modes now have a cancellable five-second countdown and explicit opt-in wording.
- Setup explains the sequence and supported sources. Installation context stays on Game and in relevant action confirmations/reviews, rather than consuming space in a permanent full-path header. Downgrade reviews list the affected files and backup destination.
- Mods use one stable toolbar row with selection actions beside Update installed, neutral optional states, visible update-check uncertainty, and inline expandable details.
- Error dialogs offer task-specific titles for foreground operations, recovery guidance, copying diagnostics, and navigation to setup, Recovery, or logs. They do not blindly retry mutations.
- File-table text now uses the base UI size instead of 11 units (partial UX-11 improvement).

User-feedback correction: removed the redundant full-path/Change game strip and generic success banners. These additions consumed workspace and required unnecessary dismissal; installation context belongs at the point of decision, and routine completion should stay unobtrusive.

Remaining validation: native Windows DPI and screen-reader behavior, Steam Deck controller/touch/keyboard flows, and participant usability testing. No Deck compatibility or accessibility certification is claimed. User-data deletion still requires a manual backup; it is explicitly irreversible.

Regression tests live in `internal/ui/ux_test.go`, `internal/ui/browser_wait_test.go`, and `cmd/steam-launcher/window_test.go`. Set `GTASA_UI_CAPTURE_DIR` to a temporary directory to generate local headless review images without committing platform-dependent goldens.

Follow-up validation passed:

- `go test -tags ci -race ./...`
- `go vet -tags ci ./...`
- Native Linux builds of `./cmd/downgrader` and `./cmd/steam-launcher` (outputs kept outside the repository).
- Headless render inspection of setup, narrow Mods, Recovery, browser waiting, deletion confirmation, and launcher/countdown screens.

No real game files or saves were modified by these checks.

## Summary

The main problem is not visual styling: it is **communicating what is happening, which installation is affected, and what the user should do next**. Preserve the existing file-verification and backup safeguards; improve the interface around them.

| Priority | ID | Finding | Evidence |
| --- | --- | --- | --- |
| P1 | UX-01 | Cancelling multiplayer removal leaves a false “Operation started” screen | Reproduced |
| P1 | UX-02 | Progress shortening removes download instructions and cancellation consequences | Reproduced |
| P1 | UX-03 | Permanent deletion is confirmed with “Continue” | Rendered + source |
| P2 | UX-04 | “Remember choice” actually skips the launcher next time, without an in-launcher bypass | Source |
| P2 | UX-05 | The selected game folder disappears on file-changing pages | Rendered + source |
| P2 | UX-06 | First-run hierarchy emphasizes patch/report controls rather than the setup sequence | Rendered + heuristic assessment |
| P2 | UX-07 | Narrow Mods layouts squeeze away selection and dependency information | Rendered + measured |
| P2 | UX-08 | “Review downgrade” provides a count, not an affected-file review | Source |
| P2 | UX-09 | Mod-state presentation treats absence as an error and hides update-check uncertainty | Rendered + source |
| P2 | UX-10 | Generic error dialogs provide diagnostics without contextual recovery actions | Source |
| Validate | UX-11 | Small file-table typography and unverified handheld interaction | Measured sizing + source; hardware test needed |

**P1:** address before the next release because these affect cancellation, irreversible actions, or completing an installation. **P2:** next usability iteration. Priorities are expert judgments, not measured incident rates.

## Scope and method

- Read the README, Fyne downgrader UI, shared widgets, launcher window, and server-browser implementation.
- Generated headless screenshots from the current source at 940×640 and 640×480 logical units, using temporary Fyne test fixtures. Examined empty setup, Mods, selected Mods, Recovery, browser-wait text, and deletion confirmation.
- Used synthetic file/mod states, not actual game modification. Startup detection/downloads were not exercised by these fixtures.
- Reproduced the multiplayer cancellation bug by invoking the actual removal action and cancelling its confirmation. Measured the Mods toolbar using a mixed installed/available selection.
- Researched the first-party guidelines linked below. This is an expert audit, **not user research with participants**, an accessibility certification, or a native Windows/Steam Deck acceptance test.
- Temporary audit tests were removed. `go test -tags ci ./internal/ui ./internal/uiwidget ./cmd/steam-launcher` passes, but all three packages currently report **no test files**. The temporary audit tests also passed; they demonstrated existing behavior rather than asserting the proposed fixes.

## Findings and fixes

### UX-01 — Cancellation leaves Multiplayer in a fictional running state

**Evidence:** `internal/ui/multiplayer.go:109–139, 260–266, 312–327`.

The inline `dismiss()` callback replaces the page with:

> Operation started. Completed changes and backups are preserved; refresh after it finishes.

“Remove & restore” invokes this **before** asking for confirmation. Cancelling returns without rebuilding the cards. The client Uninstall flow uses the same ordering.

Reproduced result after cancelling launcher removal:

```text
body = "Operation started. Completed changes and backups are preserved; refresh after it finishes."
busy = false
promptActive = false
```

There is no refresh button in that replacement body. Revisiting Multiplayer recovers the page, but users should not have to discover that workaround.

**Fix:** keep the cards until the user confirms and work actually begins. Restore them on cancellation and pre-operation failures. Use a truthful inline busy state only while an operation is running.

**Acceptance:** cancel launcher removal and client uninstall; the original cards remain usable, no operation is reported, and no files change. [R1]

### UX-02 — Short status messages discard essential instructions

**Evidence:** `internal/ui/layout.go:16–94`, `internal/ui/app.go:493–506, 934–939`, `internal/ui/browser_download.go:86–128`.

Status text has a hard 24-character limit. The Game readiness line receives that already-shortened text, despite having more space. A browser-assisted download produced:

```text
Waiting for important-m…
```

The expected filename, watched directory, browser action, and cancellation guidance are lost after the initial dialog closes. Other pages offer only the short footer. The 30-minute browser wait also uses the normal determinate progress bar, without a distinct waiting state.

Likewise, “Cancelled. Completed work and backups have been preserved.” becomes simply “Cancelled.” Subsequent rescans can replace even that outcome. This matters because cancellation is **not rollback**.

**Fix:** separate full operation state from the compact footer caption. Show a persistent task panel with phase, package/file, required next action, watched folder, and relevant controls. Add “Open page again” and “Change folder” during browser waits. Use a waiting/indeterminate indicator when total progress is unknown. Preserve a completion/cancellation summary with links to Recovery and Log.

**Acceptance:** a long filename never hides the instruction to finish the browser download; users can inspect/change the watched folder without cancelling the batch. A cancelled operation explicitly says completed changes remain. [R1]

### UX-03 — Irreversible deletion uses a generic affirmative button

**Evidence:** `internal/ui/dialogs.go:193–216, 314–315`, `internal/ui/tools.go:176–207`, `internal/ui/hub.go:93–104`.

The destructive dialogs explain the consequence in their body, but `confirm()` defaults the affirmative button to **Continue**. The rendered saves-deletion dialog has “Cancel / Continue.” “Clear saves & settings” also understates that the gallery is deleted.

There are good existing safeguards: a confirmation, the exact user-data path, and initial focus on Cancel. The problem is ambiguous action semantics, not missing confirmation.

**Fix:** use explicit labels such as **Delete saves, settings & gallery**, **Delete downgrade backups**, and **Restore original files**. Separate irreversible cleanup visually from restorative actions; offer backup/export before user-data deletion. Keep the safe initial focus. A reversible move or backup is preferable where feasible.

**Acceptance:** every destructive confirmation names the actual action and affected scope. Escape/Cancel changes nothing; opening the dialog and pressing Enter must not accidentally select deletion. [R2][R3]

### UX-04 — “Remember choice” has a surprising consequence

**Evidence:** `cmd/steam-launcher/window.go:61–72, 192–223`, `cmd/steam-launcher/main.go:74`, `internal/ui/multiplayer.go:165–182`.

The checkbox sounds like it remembers a selection. In fact, `startRemembered()` immediately activates that mode on future launches. Recovery exists through **Downgrader → Multiplayer → Reset choice**, but the chooser supplies no visible “show chooser this time” mechanism.

**Fix:** label it “Launch this mode automatically next time,” explain where to reset it, and provide an accessible one-launch bypass. Options include a short cancellable launch countdown or a documented bypass action that works with the intended input devices.

**Acceptance:** users can switch from an automatically launched mode without editing files or rediscovering the setting in another application. [R1]

### UX-05 — File-changing pages lose installation identity

**Evidence:** `internal/ui/hub.go:142–161`, `internal/ui/app.go:303–307`.

The game-folder header is hidden outside Game. Mods and Recovery retain a version/access footer, not the selected path. With multiple Steam libraries, test copies, or multiplayer installations, users must recall which directory they selected.

**Fix:** retain a compact, read-only installation header on every game-specific page: folder/library identity, detected edition, verification state, and a deliberate “Change game” action. Include the target folder in destructive and installation reviews too.

**Acceptance:** before installing mods or deleting backups, users can identify the affected installation without navigating away. [R1, recognition rather than recall and error prevention]

### UX-06 — First-run hierarchy does not clearly express the sequence

**Evidence:** `internal/ui/hub.go:63–87, 225–241, 286–327`, `internal/ui/app.go:195–203`.

The empty-state render shows checksum/report controls, a full-width Download patches button, and an empty file table. Mods and Multiplayer are disabled. A helpful choose-folder sentence exists, but there is no persistent step sequence or explanation attached to the unavailable destinations. Supported source editions are documented in the README rather than clearly introduced here.

This is a discoverability hypothesis, not a demonstrated task failure. Automatic detection and the existing readiness text already reduce the problem.

**Fix:** use an empty-state checklist: **1. Choose game → 2. Scan → 3. Prepare patches if needed → 4. Review downgrade**. Show supported editions and the backup/cancellation model nearby. De-emphasize checksum/report controls until there are results. Keep game-changing actions gated, but explain the requirements next to locked destinations or in a non-mutating preview.

**Acceptance:** a first-time user can identify the next step and explain why Mods is unavailable without opening the README. [R1][R4]

### UX-07 — Compact Mods layout hides decision-making information

**Evidence:** `internal/ui/mod_web_table.go:233–264, 380–407`, `internal/ui/mod_table.go:97–204`.

Rows reserve fixed status/action widths. The single-line toolbar reserves its filter, update button, and applicable batch actions before allocating the selection summary. In a 612-unit toolbar, representing the inner width of a 640-unit window, a mixed selection left the summary **29.14 units** wide. It rendered as an ellipsis. Row purposes and “Required (locked) | …” explanations also truncate at this size.

The update caption counts installed selections rather than known available updates; the all-available selection displays disabled “Update 0.”

**Fix:** introduce a width-dependent two-line toolbar or overflow menu, with selection/dependency counts in a guaranteed slot. Preserve update scope in the label, e.g. “Check selected for updates.” Let rows expand or expose full dependency information through a keyboard-accessible detail affordance.

**Acceptance:** test no selection, available-only, installed-only, mixed, and filtered selections at narrow widths and enlarged text. Counts and action scope remain readable; actions do not overlap. [R1]

### UX-08 — “Review downgrade” is only an aggregate confirmation

**Evidence:** `internal/ui/hub.go:364–379`, `internal/ui/app.go:815–822`.

The review contains the number of files, target version, backup location, and cancellation warning, but no list of affected files. The preceding table is useful, yet the review itself does not distinguish changed files from unchanged ones or identify the target folder.

**Fix:** include the selected installation, source → target edition, grouped affected files, modified-file warnings, and backup destination. Keep checksums under an advanced disclosure rather than making them the main review interface.

**Acceptance:** users can verify what will change from the review itself before committing. Do not weaken hash verification or permit unsupported editions. [R1]

### UX-09 — Mod states overstate absence and understate uncertainty

**Evidence:** `internal/ui/mod_batch.go:31–43`, `internal/ui/mod_status.go:14–27`, `internal/ui/mod_web_table.go:324–341`.

Every optional uninstalled mod receives danger styling, producing a wall of red on a healthy fresh installation. Conversely, the row renderer reduces installed-state details to green “Installed” or “Installed (update),” hiding “Check unavailable” and untracked-version information in Details. The aggregate check summary exists, but occupies the narrow header action lane.

**Fix:** distinguish independent dimensions: **Available / Installed**, **Update available / Check unavailable / Version unknown**, and **Required / Optional**. Use neutral styling for optional absence and warnings for uncertainty. Reserve danger styling for actual failure or unsafe conditions. Keep text/icon cues, not color alone.

**Acceptance:** no optional packages installed does not look broken. A network check failure remains visibly different from “up to date,” while still showing the package as installed. [R1][R4]

### UX-10 — Errors lack contextual recovery actions

**Evidence:** `internal/ui/app.go:314–320`, `internal/ui/dialogs.go:213–214`.

The generic error route presents a title of “Error,” raw diagnostic text, and Close. Individual messages may be useful, and the Log/export tools are valuable, but the dialog cannot consistently answer which task failed, whether changes remain, or which recovery action applies.

**Fix:** classify common failures into task-specific messages: unsupported edition, read-only folder, insufficient space, network/download failure, archive mismatch, and game still running. Pair a plain-language summary with relevant actions such as Retry, Choose folder, Import archive, Open Recovery, or Copy diagnostics. Retain raw details in a disclosure/log. Never retry file-changing operations blindly.

**Acceptance:** each common failure offers an applicable next step and accurately states whether completed changes remain. [R1, error recovery]

### UX-11 — Validate readability and handheld interaction before claiming readiness

**Evidence:** `internal/ui/layout.go:100–108`, `internal/uiwidget/theme.go:73–82`, `internal/ui/mod_web_table.go:244–245`, `cmd/steam-launcher/window.go:72`, `cmd/steam-launcher/servers.go:147–205`.

The file table forces an 11-unit font against a 14-unit base; mod checkboxes occupy 28×28 logical units. The launcher and server connection form require navigation and text entry. The inspected application code provides keyboard interactions, but no dedicated gamepad navigation or on-screen keyboard integration was found.

These facts **do not prove a WCAG failure or Steam Deck incompatibility**: Fyne logical units are not CSS pixels or measured glyph heights, and Steam Input configuration may provide navigation. Existing focus outlines, accessible labels, scrolling focus helpers, and text-plus-icon states are positive.

**Validation/fix:** offer a comfortable typography/density option; test real glyph legibility and focus visibility. Exercise the launcher and server form at 1280×800 on Deck, with its actual Steam Input mapping, trackpad, touch, keyboard invocation, and enlarged UI settings. Validate Windows scaling and screen-reader behavior separately.

**Acceptance:** the intended handheld flow works without an unexpected external keyboard, no critical control is clipped, and displayed text is readable at normal viewing distance. Valve recommends aiming for 12-pixel character height, with a 9-pixel minimum in its game compatibility review; use that as a relevant benchmark, not a certification claim for this utility. W3C target sizing is likewise a reference, not a native-app pixel conversion. [R5][R6]

## Recommended delivery order

1. **Correctness/safety:** UX-01 cancellation state; UX-02 full operation state; UX-03 explicit destructive actions. Add persistent regression tests.
2. **Task clarity:** UX-04 automatic launch escape, UX-05 installation identity, UX-06 setup guidance, UX-08 review contents.
3. **Presentation/accessibility:** UX-07 responsive Mods layout, UX-09 state taxonomy, UX-10 recovery-oriented errors, UX-11 real-device validation.

Avoid a broad cosmetic rewrite before these changes. Keep verified-downgrade gating, automatic dependency selection, backup preservation, safe modal focus, and log export.

## Suggested usability research

Recruit 5–6 representative participants spanning a first-time Windows user, an experienced modder with multiple installations, and a Linux/Steam Deck user. This is a proposed formative study, not evidence collected in this audit.

Ask them to:

1. Select the correct installation, identify whether its edition is supported, and review a downgrade without committing it.
2. Select a mod requiring dependencies and explain which packages will be installed.
3. Complete a browser-assisted download when the browser saves to a different folder.
4. Cancel a removal and describe whether anything is still running or has changed.
5. Locate recovery backups and distinguish restoring game files from deleting saves.
6. Enable automatic launcher choice, then change mode on the next launch.

Record unassisted completion, wrong-installation choices, hesitation/backtracking, requests for help, and comprehension of cancellation and deletion. Use disposable copies or simulated destructive actions; never risk participants’ saves. Measure before/after rather than inventing a usability score from this inspection.

## Research references

- **R1 — Nielsen Norman Group: [10 Usability Heuristics](https://www.nngroup.com/articles/ten-usability-heuristics/).** System status, user control, error prevention, recognition rather than recall, and recovery-oriented errors.
- **R2 — Microsoft: [Dialog controls](https://learn.microsoft.com/en-us/windows/apps/develop/ui/controls/dialogs-and-flyouts/dialogs).** Specific action labels and a safe, nondestructive exit; includes a Delete/Cancel example.
- **R3 — GNOME HIG: [Dialogs](https://developer.gnome.org/hig/patterns/feedback/dialogs.html).** Specific imperative labels; prefer undo where possible; use confirmation for irreversible actions and avoid a destructive default.
- **R4 — Nielsen Norman Group: [Button States: Communicate Interaction](https://www.nngroup.com/articles/button-states-communicate-interaction/).** States should communicate meaning and unavailable controls should remain understandable.
- **R5 — Valve: [Steam Deck and Steam Machine Compatibility Review](https://partner.steamgames.com/doc/steamhardware/compat).** Launcher controller navigation, text input, 1280×800 display support, and text-legibility guidance. These are Steam game review criteria, not a verified result for this project.
- **R6 — W3C: [Target Size (Minimum)](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html), [Focus Visible](https://www.w3.org/WAI/WCAG22/Understanding/focus-visible.html), [Contrast (Minimum)](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html).** Accessibility benchmarks; a native Fyne accessibility evaluation still requires platform testing. No blanket contrast failure is asserted here.
