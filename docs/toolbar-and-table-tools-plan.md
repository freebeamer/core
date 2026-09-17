# Toolbar and table tools implementation plan

Date: 2026-09-02
Status: Completed

## Goal

Add a compact, conventional calibration-editor toolbar to FreeBeamer without
putting application behavior inside the toolbar component. The same command
definitions must drive toolbar buttons, keyboard shortcuts, and future native
or web menus.

The toolbar will also expose a contextual table tool that applies a selected
operation and operand to the active map's selected cells. A multi-cell
operation must be validated and recorded as one atomic edit so one Undo
restores the complete selection.

This work does not add ECU flashing, live emulation, data acquisition, or
comparison-bin support.

## TunerPro behavior used as reference

TunerPro separates its main application toolbar from its table editor toolbox.
The main toolbar groups common file and application commands. The table toolbox
acts on the current calculated-value selection: the user selects one or more
cells, chooses an operation, enters an operand, and executes it.

The documented table operations include offset, multiply, divide, fill with a
value, smoothing, and copying corresponding values from a comparison binary.
They operate on calculated engineering values and are unavailable in raw-hex
view. Recent TunerPro versions also allow typing a value directly into a
multi-cell selection as a shortcut for filling it.

FreeBeamer will reproduce the useful interaction model, not TunerPro artwork,
branding, or an exact visual copy.

References:

- [TunerPro interface and toolbar](https://www.tunerpro.net/WebHelp/source/tunerprointerface.htm)
- [TunerPro table editing and table tools](https://tunerpro.net/WebHelp/source/tables.htm)
- [TunerPro keyboard shortcuts](https://www.tunerpro.net/WebHelp/source/shortcuts.htm)
- [TunerPro version history](https://tunerpro.net/downloadApp.htm)

## Toolbar layout

Use two compact rows at the top of the application workspace:

```text
File  Edit  View  Tools  Help                 calibration.bin*   Ready
[Open] [Recent] [Save As] | [Undo] [Redo] | [New Grid] [Changes] | [Help]
--------------------------------------------------------------------------------
Active map: [Operation v] [Operand] [Apply]                 Selection: 3 x 4
```

The first implementation may render the menu labels as application-owned web
menus. The command model must remain suitable for Wails native menus later.
Toolbar groups may collapse into an overflow menu when the window is narrow;
they must not force the map workspace below its supported minimum width.

### Global command groups

1. **File**: Open workspace, recent workspaces, Save As, close workspace.
2. **Edit**: Undo and Redo.
3. **Workspace**: new grid and open Changes.
4. **Integrity**: checksum/compatibility status and verification when useful.
5. **Help**: shortcuts/help dialog and, later, About.

Do not show disabled flashing, emulation, comparison, or logging buttons for
features FreeBeamer does not implement.

### Document state

Show the loaded BIN filename in the top row. Append `*` when unsaved edits
exist. The adjacent status must distinguish ready, busy, read-only/refused,
warning, and error states with text or an icon as well as color. The full BIN
path, definition filename, and refusal reason belong in tooltips or the existing
workspace details rather than permanently consuming toolbar width.

## Command architecture

The toolbar is a view over commands, not the owner of application workflows.
Introduce one command contract with at least:

```text
id
label
description
shortcut
icon
enabled
execute
```

Suggested frontend ownership:

```text
frontend/src/app/shell/
  commands/
    app-command.ts
    workspace-commands.ts
    keyboard-command-dispatcher.ts
  toolbar/
    top-toolbar.ts
    top-toolbar.html
    top-toolbar.scss
    toolbar-icon.ts
  active-editor-store.ts
```

`WorkspaceCommands` composes the existing `WorkspaceStore`,
`EditHistoryStore`, `DocumentTabsStore`, `SaveReviewStore`, and `HelpStore`.
The toolbar component renders command state and invokes commands; it must not
call the Wails gateway or coordinate save/edit workflows itself.

`ActiveEditorStore` is the small coordination boundary between the shell and
the focused map editor. It publishes the active map ID, selection, editability,
units, pending state, and the table-operation command. This is necessary
because a grid tab can contain multiple map panels and the global toolbar must
never guess which one receives an edit.

Only the focused map editor owns its visual selection. Changing focus to
another panel changes the active editor. Closing or removing the active panel
clears the context and disables the table tools.

## Keyboard behavior

Initial shortcuts:

| Command | Shortcut |
| --- | --- |
| Open workspace | Ctrl/Cmd+O |
| Save As | Ctrl/Cmd+Shift+S |
| Undo | Ctrl/Cmd+Z |
| Redo | Ctrl/Cmd+Y or Ctrl/Cmd+Shift+Z |
| New grid | Ctrl/Cmd+N |
| Open Changes | Ctrl/Cmd+Shift+C |
| Help | F1 |

The central dispatcher must ignore application shortcuts while a text input,
textarea, select, or editable element owns the keystroke, except for shortcuts
explicitly supported by that control. Browser and operating-system conventions
take precedence. Tooltips and menu items display the platform-appropriate
shortcut.

## Cell selection behavior

The current map editor has a single selected cell. Table operations require a
proper rectangular selection model:

- Click selects one cell.
- Shift+click extends from the anchor cell to the clicked cell.
- Shift+arrow extends the selection with the keyboard.
- Pointer drag selects a rectangular range.
- Ctrl/Cmd+A selects all editable body cells in the active map.
- An ordinary arrow key moves a collapsed selection.
- Starting direct editing with Enter or F2 edits the current selection. When
  more than one cell is selected, committing a typed value is equivalent to
  **Fill**.
- Axis headers are not editable and are not included in a body selection.

Selection is scoped to exactly one map. The UI shows the selected dimensions
and cell count, and charts continue to identify the anchor/current cell. The
selection model must work for scalar, one-dimensional, and matrix maps without
separate coordinate conventions.

## Initial table operations

The contextual control is:

```text
[Operation v] [Operand] [Apply]
```

The initial operation set is:

| Operation | Result for each selected calculated value `x` |
| --- | --- |
| Fill | `operand` |
| Offset | `x + operand` |
| Multiply | `x * operand` |
| Divide | `x / operand` |
| Scale by percent | `x * (1 + operand / 100)` |

Negative offsets and percentages are valid. Divide by zero is invalid. Blank,
non-numeric, NaN, and infinite operands are invalid. Apply is disabled until
the active selection and operand are valid.

The following operations are deliberately deferred:

- **Copy from compare** requires a comparison-binary model and compatibility
  rules that do not exist yet.
- **Smooth/interpolate** requires an explicit specification for horizontal,
  vertical, and two-dimensional selections before implementation.
- Raw-hex transformations are outside this feature; operations use decoded
  engineering values and the existing encoder.

## Shared Go contracts

Table-operation DTOs and enums belong in `pkg/types`. They must not be defined
inside the desktop service, calibration package, or Wails application wrapper.
The request needs:

```text
map ID
rectangular selection (start row/column, end row/column)
operation
operand
```

The result should include normal edit/history state plus enough aggregate
information for the UI to report:

```text
selected cell count
changed cell count
unchanged cell count
round-tripped minimum/maximum where useful
CanUndo / CanRedo
```

Avoid returning a full decoded map in the mutation response. After success the
existing revision signal should cause every open view of that map, its charts,
workspace summary, checksum state, and Changes panel to refresh consistently.

## Go responsibility and atomic edits

Add one batch-operation entry point at the desktop boundary, conceptually:

```text
ApplyCellOperation(request) -> BatchEditOutcome
```

The frontend must not calculate replacement values. Go owns:

1. Resolving and validating the map and selection bounds.
2. Decoding the current calculated values.
3. Applying the requested mathematical operation.
4. Encoding and round-tripping every proposed result through the existing
   calibration encoder.
5. Rejecting an out-of-range or otherwise invalid result.
6. Committing all changed bytes and one history entry only after every cell
   succeeds.

The operation is all-or-nothing. If cell 12 of 20 fails, none of the first 11
may remain changed. Work against a clone or preflight every encoded mutation,
then replace the working bytes only after successful validation.

One Apply action creates one undo transaction regardless of selected cell
count. Undo restores all affected bytes and Redo reapplies all of them. A
successful operation that produces no byte changes creates no dirty state or
history entry. A new batch edit clears the redo stack once.

Do not silently clamp out-of-range results. Return a useful error identifying
the first failing cell, requested calculated value, and supported encoded
range. Normal format-defined quantization is allowed, and the outcome should
make material rounding visible rather than implying the requested decimal was
stored exactly.

## Editing-history refactor

The current history is based on individual cell edits. Generalize it to an
edit transaction containing one or more cell/byte changes. Existing
`SetCell`/`SetIndex` calls should use the same transaction machinery with one
change instead of maintaining a separate undo path.

History owns before and after data needed for deterministic Undo and Redo; it
must not recompute mathematical operations during Redo. Diff reporting remains
based on the working image versus the original image so overlapping edits and
restores continue to report the actual current state.

## Delivery sequence

### Phase 1 - Command foundation and main toolbar

- Move existing shell actions and shortcuts behind the command contract.
- Implement grouped toolbar layout, document dirty marker, status, tooltips,
  focus states, and responsive overflow.
- Preserve all current open, save, undo/redo, grid, Changes, and Help behavior.
- Use application-owned inline SVG icons and existing design tokens.

Acceptance: buttons, keyboard shortcuts, and menu affordances invoke the same
commands; enabled state follows workspace state; no generated Wails function is
called directly by a toolbar component.

### Phase 2 - Active editor and range selection

- Add `ActiveEditorStore` and deterministic panel focus behavior.
- Add the rectangular selection model and pointer/keyboard interactions.
- Keep selection, cell inspector, and chart highlighting synchronized.
- Support multi-cell direct Fill by typing a value.

Acceptance: selection works for scalar, 1D, and matrix maps; operations cannot
target a background or removed panel; selection changes do not mutate data.

### Phase 3 - Atomic backend batch editing

- Add shared operation request/result types.
- Generalize edit history to transactions.
- Implement Fill, Offset, Multiply, Divide, and Scale by percent in Go.
- Expose the operation through the Wails boundary and frontend gateway.
- Refresh all map and workspace projections after one successful transaction.

Acceptance: a multi-cell operation is all-or-nothing, changes only expected
bytes, appears accurately in Changes, and requires exactly one Undo and one
Redo.

### Phase 4 - Contextual table-tool UI

- Add operation dropdown, numeric operand, Apply button, selection count, busy
  state, and inline validation/error feedback.
- Preserve focus and selection after Apply so repeated adjustments are fast.
- Disable the entire group for refused/read-only workspaces and unsupported
  map presentations.
- Add the operations to the Edit menu using the same command path where that
  improves keyboard access.

Acceptance: the complete selection/operation/operand/Apply workflow works from
the top toolbar and direct multi-cell Fill works from the grid.

### Phase 5 - Hardening

- Test toolbar overflow, narrow windows, high DPI, keyboard-only use, and screen
  reader names.
- Profile representative large selections and keep the UI responsive.
- Add operation details to user help.
- Add browser-level coverage for focus changes between multiple map panels.

Acceptance: toolbar controls remain accessible and stable at the supported
minimum window size, and a representative large table edit completes without
partial state or UI desynchronization.

## Test matrix

### Go

- Every operation on scalar, 1D, and matrix selections.
- Reversed/normalized range coordinates and out-of-bounds ranges.
- Negative offset and percentage, zero operand, divide by zero, NaN, and
  infinity.
- Quantization, signed values, endianness, overlapping storage, and encoded
  range failures.
- Atomic rollback when a later cell fails.
- No-op operation behavior.
- One-step batch Undo/Redo and deterministic byte/hash restoration.
- New edits after Undo clear Redo.
- Concurrent or stale workspace requests cannot mutate a replacement workspace.

### Angular

- Command enabled state and command execution.
- Shortcut dispatch and suppression inside editable controls.
- Active panel registration, focus changes, and cleanup.
- Mouse and keyboard range selection.
- Operation/operand validation and busy-state duplicate-submit prevention.
- Direct typed Fill for multi-cell selections.
- Successful and failed Apply behavior, retained selection, and error messages.
- Dirty filename marker, status, responsive overflow, ARIA names, and focus
  order.

### Integration

- Open a deterministic fixture, select a range, apply an operation, and verify
  the expected decoded values and bytes.
- Confirm grid, inspector, charts, diff, checksum state, and dirty marker all
  refresh after the same revision.
- Undo once to restore the original hash, then Redo once to restore the edited
  hash.
- Prove an invalid result leaves every selected byte and the history unchanged.

## Definition of done

This feature is complete when a user can focus any editable map panel, select a
rectangular range, choose a supported calculated-value operation, enter an
operand, and apply it safely from the toolbar. The edit must be performed by Go
as one validated transaction, reflected consistently in every open view, and
fully reversible with one Undo. All existing single-cell editing must use and
pass through the same history model without regression.
