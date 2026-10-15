# 14: Group the default view settings

## Goal

The settings that make up a project's starting view live in one table whose
name says what they are: the view a project starts with until the user
changes it.

Today these settings are spread over three places: `default_filter`,
`list.group_by`, and `list.sort`. Once a user changes the grouping, sort, or
filter in a project, the per-project view in state takes over. Nothing in
the config says so. The keys read as if they always apply.

## Decisions

These were agreed with the user on 2026-10-15.

- **State stays the home of per-project views.** XDG defines state as view
  and layout memory, so per-project views stay where they are. Only the
  config's naming changes.
- **These keys stay in `config.toml`.** The TUI never writes the defaults;
  it writes per-project views to state. So they don't belong in
  `settings.toml` (spec 13).
- **The table matches `state.View`.** `state.View` holds `GroupBy`, `Sort`,
  and `Filter`, and the new table holds the same three things.
- **`list.fields` stays where it is.** Per-project columns are separate from
  views in state ("Columns belong to the project, independently of named
  views", `internal/state/state.go`). Its doc comment and README row gain one
  sentence: projects whose columns were set in the TUI keep their own.
- **Breaking, approved.** This moves released keys, so it ships in the same
  minor release as spec 13. Follow "Approved breaking changes" in
  `AGENTS.md`.

## Depends on

Spec 12, so that old configs get the startup migration prompt. Run it after
spec 13 merges. Both edit `internal/config/moves.go`, the template, the
README config table, and `docs/migrations.md`.

## Keys that move

| Today | New |
|---|---|
| `default_filter` | `list.default_view.filter` |
| `list.group_by` | `list.default_view.group_by` |
| `list.sort.by` | `list.default_view.sort.by` |
| `list.sort.direction` | `list.default_view.sort.direction` |

```toml
[list.default_view]
# The view a project starts with, until you change its grouping (b), sort
# (B), or filter (/) there; asanamate then remembers that project's own view.
filter = "is:open"
group_by = ""

[list.default_view.sort]
by = ""
direction = "asc"
```

## Context

- `internal/config/config.go`: `DefaultFilter` (top level), `List.GroupBy`,
  and `List.Sort` with their doc comments; `Default()`; and `validate()`
  near line 530, which checks `group_by`, `sort.by`, and `sort.direction`.
- `internal/tui/model.go`, `restoreView`: builds the fallback `state.View`
  from these three settings.
- `internal/tui/nav.go`, `internal/tui/group.go`, `internal/tui/sort.go`:
  offer the configured group and sort as picker choices.
- `cmd/asanamate/main.go` (`runList`): `--filter` defaults to
  `default_filter`.
- `internal/config/moves.go`: `Moves`. These are plain key moves, so the
  existing mechanism and spec 12's prompt cover them.
- `internal/setup/setup.go`: the config template.

## Requirements

- **Config.** Add a `DefaultView` struct with `Filter`, `GroupBy`, and
  `Sort`, under `List.DefaultView`. Remove `DefaultFilter`, `List.GroupBy`,
  and `List.Sort`. Validation errors name the new keys, such as
  `list.default_view.sort.direction`.
- **Moves.** Register the four moves in the table above, so spec 12's prompt
  and `config update` migrate them.
  - Both `list.sort` keys move to one new table, which leaves `[list.sort]`
    empty. `deletePath` already drops empty tables.
  - The template no longer has a `[list.sort]` table.
- **Callers.** Update `restoreView`, the picker choices, and `runList` to
  read `List.DefaultView`. Behavior is unchanged.
- **Template.** Replace the old keys with the new table and the comment
  shown above.
- **Docs.**
  - README config table: one row per new key, and the `list.fields` sentence
    from Decisions.
  - The `list` subcommand's `--filter` help text and usage line name
    `list.default_view.filter`.
  - `docs/migrations.md`: the key table above, under `## Unreleased`.
- **Commit** as `feat!:` with a `BREAKING CHANGE:` footer naming each moved
  key.

## Out of scope

- Moving per-project views or columns out of state.
- Default columns as part of the view (`list.fields`); see Decisions.

## Acceptance

- Config tests:
  - defaults
  - the new keys load
  - errors name `list.default_view.*`
  - each old key gives `UpdateNeeded` with its move
- Setup tests: `Merge` moves all four keys and leaves no empty
  `[list.sort]` table.
- TUI tests: a project with no saved view starts with the configured filter,
  group, and sort. A project with a saved view keeps it.
- `asanamate list` without `--filter` uses `list.default_view.filter`.
- After migration, the local `~/.config/asanamate` passes `asanamate doctor`.
- `go test ./...` and `go vet ./...` pass.
