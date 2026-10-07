# 06: Move the Harvest provider out of the main binary

**Breaking. Get the user's approval before starting. Depends on 05.**

## Goal

The core binary has no Harvest code. Harvest support ships as a separate
provider that uses the documented time tracking protocol, like any
third-party provider would.

## Context

- `internal/timetracking/hrvst.go` (~350 lines): the Harvest provider.
- `cmd/asanamate/main.go`: `asanamate time-provider hrvst --task-id <id>`,
  which serves provider requests.
- Users configure it as
  `time_tracking.command = "asanamate time-provider hrvst --task-id ..."`.
  This command string is released, so removing the subcommand breaks
  existing configs.
- `internal/timetracking/provider.go`: the generic protocol client stays.

## Requirements

- Move the provider to a separate binary, `cmd/asanamate-hrvst`, in this
  repository. Ask the user whether it should later move to its own
  repository; do not create one.
- Build and release `asanamate-hrvst` with the main binary (check the release
  config in `.github/`).
- Migration: teach `Merge` (`internal/setup/update.go`) to rewrite a
  `time_tracking.command` that starts with `asanamate time-provider hrvst` to
  `asanamate-hrvst`, keeping the flags. `Load` names the old command and
  tells the user to run `asanamate config update`, like the v0.2.0 moves in
  `internal/config/moves.go`.
- Remove the `time-provider` subcommand and its `usage` line.
- Docs: README time tracking section and install instructions; the
  "Upgrading" section for the next version.
- Commit as `feat!:` with a `BREAKING CHANGE:` footer naming the old and new
  commands. Tell the user the next tag needs a breaking version bump.

## Out of scope

- Changing the time tracking protocol beyond 05.

## Acceptance

- No package under `internal/` imports Harvest code.
- `asanamate-hrvst` passes the time tracking protocol test from 05.
- `Merge` test: the old command is rewritten and other values are kept.
- `go test ./...`, `go vet ./...` pass.
