# 09: Background Activity scans with a disk cache

**Depends on nothing.**

## Goal

Activity is ready when the user opens it. Scans run in the background on a
timer, results persist across restarts, and the Views panel shows how many
new related events arrived since the user last looked.

## Context

- `internal/activity/activity.go`: `Fetch` scans tasks changed since `since`
  and streams sorted snapshots. `Classify` is the only reader of
  `Task.HTMLNotes`. The package doc says it does not persist activity.
- `internal/tui/activity.go`: `activityView` owns the scan (`seq`, `cancel`,
  `updates`). `openActivity` scans every time the view opens, and
  `closeActivity` cancels the scan. `openActivityPeriod` clears the report and
  rescans when the period changes.
- `internal/tui/nav.go:72`: the Activity row shows its key (`I`) in the right
  column, where other view rows show counts. `I` is already in `?` help
  through `internal/keymap/catalog.go:26`.
- `internal/tui/refresh.go`: the ticket auto-update tick chain
  (`refreshSeq`, `scheduleRefresh`). Activity is deliberately kept out of it.
- `internal/state/state.go`: `Save` writes atomically with owner-only
  permissions. Copy that pattern.
- `internal/config/config.go`: `StateDir` and `xdgDir`;
  `ParseRefreshInterval` validates `list.refresh_interval`.

## Requirements

### Config

- New `[activity]` table with `refresh_interval`, default `"2m"`. Same
  duration syntax as `list.refresh_interval` (at least `1s`), plus `"0"`,
  which turns background scans off. Errors name `activity.refresh_interval`.
- Follow the AGENTS.md key checklist: struct field and doc comment,
  `Default()`, `validate()`, the template in `internal/setup/setup.go`, the
  README config table, and the config tests.

### Cache

- `config.CacheDir()` returns `$XDG_CACHE_HOME/asanamate`, falling back to
  `~/.cache/asanamate`. `cmd/asanamate/main.go` passes it to the TUI in
  `Deps.CacheDir`.
- New `internal/activity/cache.go` holds a `Cache`: workspace GID, user GID,
  `Until` (the end of the last complete scan), `SeenAt`, and entries. It saves to
  `activity.json` atomically with mode 0600.
- Loading a missing, unreadable, or mismatched (workspace or user) file gives
  an empty cache. A corrupt cache never blocks startup.
- `Entry` gains `FoundAt`: when a scan first loaded it. Entries drop
  `Task.HTMLNotes` before they are stored.
- `Cache.Merge(report, foundAt)` adds a scan report's entries. It dedupes by
  story GID (the newer copy replaces the older one but keeps its first
  `FoundAt`), prunes entries older than 30 days, extends `Until` when the
  report is complete, and keeps entries sorted newest first.
- `Cache.Replace(report, foundAt)` does the same, but first drops cached
  entries inside the report's window. Manual reload uses it, so deleted
  stories disappear.
- The cache always covers the last 30 days. The period picker (`D`) filters
  cached entries by time and never triggers a scan.

### Scanning

- The model owns one scan at a time. Closing the view does not cancel it.
- **Launch:** load the cache, then start a scan.
- **Cold scan (no complete scan within 30 days):** 30 days. Its entries get a zero `FoundAt`, so
  they never count as new.
- **Incremental scan:** from `cache.Until − 15m`. The overlap covers search
  indexing lag; dedupe absorbs repeats.
- **Manual reload (`r`):** full rescan of the selected period, through
  `Replace`.
- **Timer:** its own tick chain at `activity.refresh_interval`, separate from
  the ticket refresh. A tick that finds a scan running is skipped.
- **`"0"`:** no launch scan and no timer. Opening the view starts an
  incremental scan, as it scans today; manual reload still works.
- Streamed snapshots merge into the cache, so an open view updates live. The
  file is written once per finished scan.
- **Errors:** HTTP 402 (no paid search API) stops background scans for the
  session. Other errors keep cached entries, show in the view banner, and
  retry on the next tick. Cache write errors show in the statusline once.

### New events

- An entry is new when it is not `Mine`, not `Automatic`, and its `FoundAt`
  is after `SeenAt`.
- The Activity row in the Views panel shows the new count in its right
  column, blank at zero. The row no longer shows `I`.
- Opening Activity marks the entries new at that moment, then sets `SeenAt`
  to now and saves. Marked rows show the `NEW` badge of changed My Tasks
  tickets (`colors.new`) until the view closes. Closing the view sets
  `SeenAt` to now again, so events that arrived while it was open do not
  count afterwards.

### Docs

- README Activity section: describe the background scan, the cache file and
  its location, the new count, and that `r` rescans the period. Remove "stores
  activity only in memory". Add the cache path next to the state path in the
  files list.
- `activityHelp`: replace "Activity stays in memory" with the cache note.
- `internal/activity` package doc: it now persists a cache.

## Out of scope

- Per-entry read state.
- Notifications outside the Views panel.
- Refreshing cached stories whose text changed after they were cached, except
  through manual reload.

## Acceptance

- `activity` tests: `Merge` dedupe keeps the first `FoundAt`, prune at 30
  days, `Replace` drops stale entries in its window, new-entry rule, cache
  round-trip, mismatched workspace or user gives an empty cache, corrupt file
  gives an empty cache.
- `config` tests: default, valid duration, `"0"`, invalid value names
  `activity.refresh_interval`.
- `tui` tests: nav row shows the new count and not `I`; opening Activity
  clears the count and marks rows; a tick during a scan is skipped; a 402
  stops the timer; changing the period does not scan.
- `go test ./...`, `go vet ./...` pass.
