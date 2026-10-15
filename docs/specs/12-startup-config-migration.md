# 12: Offer config migration at startup

## Goal

When a user upgrades and their `config.toml` still uses settings an older
release moved, the TUI offers to migrate it at startup instead of stopping
with an error. One answer runs the existing `asanamate config update` and the
app starts.

## Context

- `internal/config/moves.go`: `Moves` and `CommandMoves` list released keys
  and commands that moved. `ApplyMoves` and `ApplyCommandMoves` rewrite a
  decoded config.
- `internal/config/agentspreset.go`: `agents.preset`, replaced by
  `agents.command`, with `MoveAgentsPreset`.
- `internal/config/config.go`, `Load`: decodes the file into a map and
  returns an error for the first pending move ("`<key> moved to <key> in
  v0.2.0; run asanamate config update`").
- `internal/setup/update.go`: `Update` runs `Merge`, which applies the same
  moves and rebuilds the file from the template. `Update` also saves a
  timestamped backup, asks before writing unless `yes`, and checks that the
  result loads. Comments are reset to the template's; the summary says so.
- `internal/prompt`: `Confirm` asks a yes/no question on the given reader and
  writer.
- `cmd/asanamate/main.go`: `runTUI` loads the config through `loadConfig`.
  `list`, `show`, `doctor`, and the write subcommands load it too.
- `docs/migrations.md` and the README "Upgrading" section tell users to run
  `asanamate config update` after upgrading.

## Decisions

- **Detect old configs by shape, not by a version number.** The move tables
  already say what an old config looks like. A `config_version` key would
  miss old snippets pasted into a current file, add a number to bump on every
  breaking release, and create a new failure mode when an older binary meets
  a newer config. Add a version only if a release ever changes what a value
  means without changing its key. AGENTS.md already rules that out ("add keys
  and values rather than changing them").
- **Ask only for files the user edits.** `config.toml` is the user's, so the
  app asks before rewriting it. Files only the app writes, such as
  `state.toml`, migrate silently when they load.
- **Ask only in the TUI on a terminal.** Scripts and agents call `list`,
  `show`, and the write subcommands, and must never block on a question.

## Requirements

- **One error type for pending changes.** `config.Load` returns a
  `*config.UpdateNeeded` when the file needs `config update`:

  ```go
  // UpdateNeeded reports a config that uses settings an older release moved.
  type UpdateNeeded struct {
      Path    string
      Changes []string // one line per pending change, such as "theme moved to theme.name"
  }
  ```

  It lists every pending change, not only the first. That covers all
  `Moves`, `CommandMoves`, and the `agents.preset` replacement. Its
  `Error()` keeps the existing guidance: the path, the changes, and "run
  `asanamate config update`". Callers check for it with `errors.As`.
- **One source for detection and migration.** Add a single function in
  `internal/config`, such as `PendingUpdates(raw map[string]any) []string`,
  that `Load` uses to build `Changes`. It must cover exactly what `Merge`
  applies, so every future move registered for `Merge` is detected without
  further work.
- **Room for new kinds of change.** Specs 13 to 15 rely on this function and
  add new kinds of change:
  - renamed values, such as the built-in theme `dark` becoming `ansi-dark`
  - keys that move out of `config.toml` into `settings.toml`

  Keep `PendingUpdates` and `Merge` built from one ordered list of change
  kinds, so a new kind is added in one place and covered by both.
- **Startup prompt in `runTUI`.** Move the load-and-migrate step into a
  testable function in `cmd/asanamate`, such as
  `loadOrMigrate(path string, in io.Reader, out io.Writer, interactive bool) (config.Config, error)`.
  When `Load` returns `UpdateNeeded`:
  - **Not interactive:** stdin or stdout is not a terminal. Return the
    error unchanged. Use `github.com/charmbracelet/x/term`, which the repo
    already depends on (`internal/kitty/kitty.go`).
  - **Interactive:** print the changes, then ask with `prompt.Confirm`. For
    example:

    ```
    Your config uses settings from an older asanamate:
      - theme moved to theme.name
      - accent_color moved to colors.accent
    Update ~/.config/asanamate/config.toml now? It keeps your values, resets
    comments to the current template, and saves the old file as
    config.toml.<time>.bak.
    ```

    On yes, call `setup.Update` with `yes = true` (the question was already
    asked), then load again and continue starting the TUI. On no, return
    the `UpdateNeeded` error so asanamate exits as it does today.
  - **Update fails:** for example, `ApplyMoves` finds both an old and a new
    key set. Return that error. The config stays unchanged, as `Update`
    guarantees.
- **Other commands are unchanged.** `list`, `show`, `doctor`, `config`, and
  the write subcommands report `UpdateNeeded` as an error, with its new
  message listing every change.
- **Docs.**
  - README "Upgrading": the TUI offers the update at startup, and
    `asanamate config update` still runs it by hand.
  - `docs/migrations.md`: add a note under `## Unreleased`.
  - `AGENTS.md`, "Approved breaking changes": a move must be registered so
    `PendingUpdates` reports it and `Merge` applies it, and then the startup
    prompt covers it.
  - `AGENTS.md`, "Local config migration": add the rule that files only the
    app writes migrate silently on load.

## Out of scope

- A `config_version` key; see Decisions.
- Keeping the user's own comments through `config update`. The backup keeps
  them, as today.
- Prompting from any command other than the TUI.

## Acceptance

- Config tests: a file with two moved keys and an `agents.preset` returns an
  `UpdateNeeded` that `errors.As` finds, with all three changes listed. A
  current config loads without it. The `PendingUpdates` entries match what
  `Merge` changes for the same input.
- `cmd/asanamate` tests for `loadOrMigrate`:
  - **Yes:** the file is rewritten, a backup exists, and the returned
    config loads.
  - **No:** returns `UpdateNeeded` and the file is byte-for-byte unchanged.
  - **Not interactive:** returns `UpdateNeeded` without reading input.
  - **Conflicting old and new keys:** returns the conflict error and leaves
    the file unchanged.
- Existing tests that match the old single-key message are updated.
- `go test ./...` and `go vet ./...` pass.
- Not breaking. It is a `feat`.
