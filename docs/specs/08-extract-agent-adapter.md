# 08: Move tool-specific agent code into an adapter

**Breaking. Get the user's approval before starting. Depends on 06.**

## Goal

The core knows only the generic agent source: a command that prints agents,
a state mapping, linking agents to tickets by branch and repo, and showing
them. Code for specific tools (ccmux, Claude Code session files, the Codex
hook) moves to an adapter command that prints the generic format.

## Context

- `internal/agents/agents.go`, `internal/agents/live.go`: agent sources,
  including the ccmux preset (`agents.preset = "ccmux"`) and the merge of
  ccmux state with self-reported state (README "Agent status").
- Claude Code state: read from `$CLAUDE_CONFIG_DIR/sessions/`.
- Codex state: `asanamate hook codex` writes per-session files under
  `$XDG_STATE_HOME/asanamate/agents/`; `internal/setup/hook.go` installs the
  hook into `$CODEX_HOME/hooks.json`; `asanamate setup hooks` and the
  `doctor` row `codex_hook` manage it.
- The generic path: `agents.command` printing
  `<path>\t<status>[\t<target>[\t<title>]]`, with `[agents.states]`.
- Released surface affected: `agents.preset`, `asanamate hook codex`,
  `asanamate setup hooks`, and the `codex_hook` doctor row.

## Requirements

- Add `cmd/asanamate-ccmux`, which runs `ccmux show --json`, merges Claude and
  Codex self-reported state the way the preset does today, and prints the
  generic TSV with statuses already mapped to asanamate's state names.
- Move `hook codex` and the hook installer to the adapter
  (`asanamate-ccmux hook codex`, `asanamate-ccmux setup`).
- The core keeps: the `agents.command` source, `[agents.states]`, linking,
  display, and the `doctor` agent listing.
- Migration: `Merge` rewrites `agents.preset = "ccmux"` to
  `agents.command = "asanamate-ccmux"`. Existing Codex hooks that run
  `asanamate hook codex` must keep working until the user reruns the adapter's
  setup: keep `asanamate hook codex` as a hidden alias that forwards to the
  adapter's code for one release, and have `doctor` warn about it.
- If the generic TSV cannot carry something the preset needs (such as "turn
  finished but not yet seen"), extend the format in a backward-compatible way
  and document it in `docs/extensions.md`.
- Docs: README Agents section, setup steps, troubleshooting, and the
  "Upgrading" section.
- Commit as `feat!:` with a `BREAKING CHANGE:` footer naming the removed key
  and commands. Tell the user the next tag needs a breaking version bump.

## Out of scope

- Adapters for other agent tools.
- Changing how agents link to tickets.

## Acceptance

- `internal/agents` contains no ccmux, Claude, or Codex specific code.
- The adapter's output, fed through `agents.command`, gives the same states
  as the preset did for the existing `agents_test.go` cases.
- `Merge` test for the preset rewrite.
- `go test ./...`, `go vet ./...` pass.
