# 06: Extension protocols

## Goal

External commands are asanamate's extension mechanism. Each protocol is
documented in one place, versioned, and covered by tests, so others can write
extensions without reading the source.

## Context

asanamate already runs external commands through `/bin/sh -c` in four
places:

| Extension point | Config | Code | Format today |
|---|---|---|---|
| Actions | `actions/*.toml` | `internal/action` | environment variables (`ASANAMATE_*`), stdin, exit code |
| Agent sources | `agents.command`, `agents.states` | `internal/agents` | TSV lines `<path>\t<status>[\t<target>[\t<title>]]` |
| Time tracking | `time_tracking.command` | `internal/timetracking/provider.go` | one JSON request on stdin, JSON reply |
| Repo sources | `repo_source.command` | `internal/repo` | one path per line |

The README documents each one inside its feature section.

Go's `plugin` package is not an option: plugins must match the host's Go
version and dependency versions exactly, which rules out shipping them
separately.

## Requirements

- Write `docs/extensions.md` with one section per extension point: when it
  runs, input, output, exit codes, timeouts, error handling, and a minimal
  example script.
- Give the JSON protocol (time tracking) a version field so it can change
  without breaking providers: requests carry `"version": 1`, and providers
  may ignore it. Document that unknown request fields must be ignored.
- Replace the protocol details in the README with a short summary and a link.
- Add protocol tests that run a small fixture script per extension point
  (under `testdata/`) through the real code path, so a format change fails a
  test.
- Link `docs/extensions.md` from the README intro (see 01).

## Out of scope

- A plugin registry, discovery, or installation command.
- New extension points.

## Acceptance

- `docs/extensions.md` describes all four extension points.
- Each extension point has a fixture-script test.
- The time tracking request includes `"version": 1`, and existing providers
  still work.
- `go test ./...` passes.
