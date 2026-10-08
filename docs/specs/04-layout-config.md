# 04: Layout configuration

## Goal

Users can fit asanamate to any spot in their terminal workflow: a full
screen, a tmux sidebar showing only the list, a popup, or a split with a fixed
list width and the reader filling the rest. They set this in config and can
toggle the reader while running.

## Context

- `cmd/asanamate/main.go:310`: the `--no-preview` flag (list only). It is
  released, so it must keep working.
- `internal/tui/model.go`: `Deps.NoPreview`; `paneWidths` (line ~828) splits
  the list and the reader; `narrowWidth = 100` is the narrowest split.
- `internal/tui/nav.go`: `wideWidth = 160` shows the views panel.
- `internal/tui/listview.go`: `fitList` and the `=` binding size the list to
  its content; `m.listW` holds that width.
- `NoPreview` is also checked in `history.go`, `keys.go`, and `reader.go`.
- `AGENTS.md`: a concept that will grow more settings gets its own sub-table.
  Layout will grow (views panel, borders), so use `[layout]`.

## Requirements

- **Config `[layout]`:**
  - `reader` (bool, default `true`): start with the reading pane shown.
    `--no-preview` forces it off.
  - `list_width` (int, default `0`): a fixed list width in columns, borders
    included. `0` keeps today's sizing. The reader takes the rest of the
    width. When the reader would get fewer than its minimum width, the
    reader hides, as on narrow terminals today. Validation rejects negative
    numbers.
- **Runtime toggle:** a `toggle_reader` binding in `[keys.main]` (default
  `z`) shows or hides the reader. Hiding it moves focus to the list. The
  choice lasts for the session only; config sets the start state.
- **Code:** replace `Deps.NoPreview` checks with one method, such as
  `m.readerShown()`, that combines the config, the flag, the toggle, and the
  terminal width. `fitList` and `=` do nothing to the width when
  `list_width` is set.
- **Docs:** README Layout section, Keys table, config table, and the
  "List-only mode and scripting" section. Show one tmux sidebar example and
  one popup example.

## Out of scope

- Hiding or sizing the views panel. Note it as a follow-up in
  `docs/deferred-ideas.md` if you find a clean shape for it.
- Persisting the toggle across sessions.

## Acceptance

- Config tests: defaults, `list_width` below zero rejected with an error
  naming `layout.list_width`.
- TUI tests: `reader = false` starts list only; the toggle shows and hides
  the reader and moves focus; `list_width = 60` gives the list 60 columns at
  width 160; a fixed width that leaves too little room hides the reader.
- `--no-preview` behaves as before.
- `go test ./...`, `go vet ./...` pass.
