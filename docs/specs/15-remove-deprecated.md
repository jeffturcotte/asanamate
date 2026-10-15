# 15: Remove deprecated commands and flags

## Goal

v0.3.0 is a breaking release (specs 13 and 14), so it also removes the
compatibility code that was waiting for one.

## Decisions

These were agreed with the user on 2026-10-15.

- **Remove the hidden `asanamate hook` subcommand.** It forwards Codex hook
  events to the ccmux code for setups made before `asanamate-ccmux`
  existed, and it is marked "remove in v0.3.0".
- **Remove the `--no-preview` flag.** It is an alias for `--hide-preview`,
  and `docs/deprecated.md` says removing it needs a breaking release.
- **Not changed:**
  - CLI subcommand names, such as `edit-comment`. Regrouping them would
    break scripts and agents for a cosmetic gain.
  - Extension protocols. They carry a `version` and tell providers to ignore
    unknown fields, so they can change without a break.
  - `ASANAMATE_*` environment variables. They are consistent.

## Depends on

Nothing. It can run in parallel with specs 12 to 14, and must be released
with them.

## Context

- `cmd/asanamate/main.go`:
  - the `case "hook":` branch in `run`
  - the `--no-preview` flag in `runTUI`
  - the `--no-preview` line in `usage`
  - `checkLegacyCodexHook`, called from `runDoctor`
- `internal/ccmux/codexhook.go`: `LegacyCodexHook` has no other caller.
  `eachCodexHook` is also used by the hook setup code, so it stays.
- `internal/ccmux/live.go`: `RunHook` stays; `cmd/asanamate-ccmux/main.go`
  serves `asanamate-ccmux hook codex` with it.
- `README.md` lines ~529-530 mention `--no-preview` and link to
  `docs/deprecated.md`.
- `docs/deprecated.md` lists `--no-preview`.

## Requirements

- **`hook`.**
  - Delete the `hook` case in `run`, so `asanamate hook` prints usage and
    exits 2 like any unknown command.
  - Delete `checkLegacyCodexHook`, its call in `runDoctor`, and
    `ccmux.LegacyCodexHook` with its tests.
  - Keep `RunHook` and the rest of `internal/ccmux`.
- **`--no-preview`.** Delete the flag and its usage line. Passing it fails
  the way any unknown flag does.
- **Docs.**
  - README: drop the `--no-preview` sentence and the link to the deprecated
    list.
  - `docs/deprecated.md`: keep the file and its intro, and replace the table
    with "None." so later deprecations have a place.
  - `docs/migrations.md`, under `## Unreleased`:
    - `--no-preview` is gone; use `--hide-preview`.
    - If `asanamate doctor` on v0.2.x warned about `codex_hook`, run
      `asanamate-ccmux setup` before upgrading. Otherwise Codex keeps
      calling a command that no longer exists.
- **Commit** as `feat!:` with a `BREAKING CHANGE:` footer naming the removed
  subcommand and flag.

## Out of scope

- Changes to `asanamate-ccmux` itself.

## Acceptance

- `asanamate hook codex` exits 2 with usage.
- `asanamate --no-preview` fails with an unknown-flag error.
- `asanamate --hide-preview` behaves as before.
- `doctor` no longer prints a `codex_hook` row.
- `go test ./...`, `go vet ./...`, and staticcheck pass, with no unused code
  left behind.
