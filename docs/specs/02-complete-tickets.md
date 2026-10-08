# 02: Complete and reopen tickets

**Depends on 01.**

## Goal

Users and agents can mark a ticket complete, or reopen a completed one, from
the TUI and the CLI.

## Context

- `internal/asana/api.go`: write methods such as `SetAssignee` and `SetDueOn`
  call `updateTask`. Completion is `PUT /tasks/{gid}` with
  `{"completed": true|false}`.
- `internal/asana/types.go:72`: `Task.Completed` is already loaded.
- `internal/writeback/writeback.go`: `Service` holds the CLI writes, with the
  confirmation prompt (`confirm`).
- `cmd/asanamate/main.go`, `writeCommand`: parses `comment`, `move`, and
  `field`.
- `internal/tui/edit.go`: the `e` menu (`editAssignee` and others) and the
  pickers behind it.
- `internal/keymap/catalog.go`: `[keys.main]` bindings.
- The default filter `is:open` hides completed tickets. The **Viewing**
  section already keeps an edited ticket visible when the view excludes it.

## Requirements

- **API:** add `Client.SetCompleted(ctx, taskGID string, done bool) error`.
- **Writeback:** add `Service.Complete(ctx, gid string, done bool) error`. It
  confirms like the other writes ("Mark <title> complete?" / "Reopen
  <title>?").
- **CLI:** `asanamate complete [--yes] [--reopen] <gid>`. Add it to the
  `usage` text.
- **TUI:**
  - A `complete` binding in `[keys.main]`, default key `x`, that toggles the
    selected ticket: complete when open, reopen when complete.
  - A matching item in the `e` menu, labelled "Complete" or "Reopen" to fit
    the ticket.
  - After the write, reload the ticket. When the view no longer shows it, keep
    it in **Viewing** like other edits, so the user can undo at once with
    the same key.
  - The status line confirms the change ("Completed <title>").
- **Docs:** README Keys table, the `e` row, "Writing back to Asana", and
  "What it does".

## Out of scope

- Completing several tickets at once.
- Approval or milestone task types.

## Acceptance

- Unit test for `SetCompleted` request body, like the existing `client_test.go`
  tests.
- Unit test for `Service.Complete`, including a refused confirmation.
- TUI test: pressing the binding on an open ticket calls the write with
  `done=true`, and the ticket shows in Viewing under `is:open`.
- `go test ./...`, `go vet ./...` pass.
