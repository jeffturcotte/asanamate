# 03: Create tickets and subtasks

**Depends on 01.**

## Goal

Users and agents can create a ticket in the current project or My Tasks, and
a subtask under a ticket, from the TUI and the CLI.

## Context

- Asana API: `POST /tasks` with `name`, `workspace`, and either `projects`
  (plus `memberships` for a section) or `assignee` for My Tasks.
  `POST /tasks/{gid}/subtasks` creates a subtask.
- `internal/asana/api.go`: client methods; `Subtasks` already reads subtasks.
- `internal/writeback/writeback.go`: `Service`, `Move` (section lookup by name),
  and `Comment` (Markdown to Asana HTML via `markdown.go`).
- `cmd/asanamate/main.go`, `writeCommand`.
- `internal/tui/inputbox.go`: the text input used for comments, with
  `ctrl+e` external editing.
- `internal/tui/listview.go`, `internal/tui/reader.go`: list selection and the
  reading pane.

## Requirements

- **API:** add `Client.CreateTask(ctx, NewTask) (Task, error)` and
  `Client.CreateSubtask(ctx, parentGID string, NewTask) (Task, error)`. Keep
  `NewTask` small: name, notes HTML, project, section, assignee, due date.
- **Writeback:** add `Service.Create(...)`, which resolves a section name and
  an assignee name like `Move` and the `people` field do, converts Markdown
  notes to HTML like `Comment`, and confirms before writing.
- **CLI:** `asanamate create [--yes] [--project <gid>] [--section <name>]
  [--parent <gid>] [--assignee <name|me>] [--due <YYYY-MM-DD>]
  [--notes <markdown|->] <title>`.
  - Without `--project` or `--parent`, the ticket goes to My Tasks, assigned
    to the user.
  - `--parent` makes a subtask and cannot be combined with `--project`.
  - It prints the new task's gid on stdout, so scripts can chain it.
- **TUI:**
  - A `create` binding (default `n`): asks for a title, then creates the
    ticket in the current project, in the section of the selected ticket when
    the list is grouped by section, or in My Tasks.
  - A `create_subtask` binding (default `N`): asks for a title and creates a
    subtask of the selected ticket.
  - After creating, reload and select the new ticket (or the parent, for a
    subtask) and show it in the reader. Keep it in **Viewing** if the filter
    hides it.
  - Other fields are set afterwards with the existing edit keys. No
    multi-field form.
- **Docs:** README Keys table, "Writing back to Asana", "What it does", and the
  `usage` text.

## Out of scope

- Templates, duplicating tickets, dependencies, and multi-field creation
  forms.
- Editing a ticket's title or description (a separate future task).

## Acceptance

- Client tests for both create calls' request bodies.
- `Service.Create` tests: My Tasks default, project plus section, subtask,
  `--parent` with `--project` rejected, unknown section rejected.
- CLI prints only the gid on success.
- TUI test: the binding opens the title input; submitting creates the ticket
  and selects it.
- `go test ./...`, `go vet ./...` pass.
