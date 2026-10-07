# 04: CLI parity with the TUI

## Goal

Every write the TUI offers is also a CLI subcommand, and both use the same
code in `writeback.Service`.

## Context

- CLI writes today: `comment`, `move`, `field` (`cmd/asanamate/main.go`,
  `writeCommand`), implemented in `internal/writeback/writeback.go`.
- The TUI writes through the Asana client directly in
  `internal/tui/edit.go`: `SetAssignee` (line ~550), `SetDueOn` (~469),
  `SetCustomField` (~453, ~497), `SetMyTasksSection` (~268), plus project add
  and remove. These duplicate the resolution logic in `writeback`.
- TUI-only writes today: assign, set due date, add to project, remove from
  project, move to a My Tasks section.

## Requirements

- **Writeback methods**, each confirming like the existing ones:
  - `Assign(ctx, gid, user string)`: user name, `me`, or `""` to unassign.
    Reuse the name lookup from `people` fields, which refuses ambiguous names.
  - `SetDue(ctx, gid, date string)`: `YYYY-MM-DD` or `""` to clear. Reject
    other formats.
  - `AddProject(ctx, gid, projectGID string)` and
    `RemoveProject(ctx, gid, projectGID string)`.
  - Extend `Move` to accept My Tasks when the ticket is the user's (for
    example `--my-tasks`), matching the TUI.
- **CLI subcommands:**
  - `asanamate assign [--yes] <gid> <name|me|"">`
  - `asanamate due [--yes] <gid> <YYYY-MM-DD|"">`
  - `asanamate project add|remove [--yes] <gid> <project-gid>`
- **TUI:** call these `writeback.Service` methods instead of the client. The
  TUI passes no `Confirm` function, so it does not prompt. Keep the TUI's
  pickers; only the final write moves.
- **Rule for later work:** a new write is added to `writeback.Service` first,
  then exposed in both the TUI and the CLI. Add this rule to `AGENTS.md`.
- **Docs:** "Writing back to Asana", the `usage` text, and the environment
  variables passed to actions if any new ones are needed.

## Out of scope

- Completing and creating tickets (02, 03), which add their own methods.
- New TUI features.

## Acceptance

- `internal/tui/edit.go` has no direct `Client.Set*`, `AddToProject`, or
  `RemoveFromProject` calls.
- Tests for each new `Service` method, including invalid dates and ambiguous
  user names.
- Existing TUI edit tests pass unchanged, or with only their fakes updated.
- `go test ./...`, `go vet ./...` pass.
