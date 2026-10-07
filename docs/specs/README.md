# Roadmap specs

asanamate aims to be a full-featured, extensible Asana TUI. It should cover
the Asana work an average user does every day, run well in any terminal layout
(full screen, tmux pane, sidebar, or popup), and let external commands extend
it. The CLI should offer every write the TUI offers, so scripts and agents can
do what a person can.

Each file below is one self-contained task, numbered in the order to run
them. A task can start once the tasks in its "Depends on" column are merged
into `main`; tasks with no dependency between them (such as 02, 03, and 04)
can run in parallel, each on its own branch.

| Spec | Summary | Depends on | Breaking |
|---|---|---|---|
| [01-cli-parity](01-cli-parity.md) (done) | `assign`, `due`, and `project` subcommands; route TUI writes through `writeback` | none | no |
| [02-complete-tickets](02-complete-tickets.md) (done) | Mark tickets complete or reopen them, in the TUI and the CLI | 01 | no |
| [03-create-tickets](03-create-tickets.md) (done) | Create tickets and subtasks, in the TUI and the CLI | 01 | no |
| [04-layout-config](04-layout-config.md) (done) | `[layout]` table and a reader toggle key | none | no |
| [05-extension-protocols](05-extension-protocols.md) (done) | Document and test the external command protocols as the extension contract | none | no |
| [06-extract-hrvst](06-extract-hrvst.md) | Move the Harvest provider out of the main binary | 05 | yes |
| [07-extract-agent-adapter](07-extract-agent-adapter.md) | Move the ccmux, Claude, and Codex agent code into an adapter | 05 | yes |
| [08-readme-mission](08-readme-mission.md) | Rewrite the README intro around the project's goal and scope | 02, 03, 05 | no |

## Rules for every task

- **Read `AGENTS.md` first.** Its config key rules apply to every new key:
  struct field and doc comment, `Default()`, `validate()`, the template in
  `internal/setup/setup.go`, the README config table, and the config tests.
- **New key bindings** go in `internal/keymap/catalog.go`, so they are
  configurable, show in `?` help, and pass `TestKeyBindingsAreUniqueAndDocumented`.
  Each spec sets its default keys, and no two specs share one: `x` complete
  (02), `n` create and `N` create subtask (03), `z` toggle reader (04). Use
  them as given; if one is taken on `main` when you start, stop and ask the
  user.
- **New writes** go through `writeback.Service`, so the TUI and the CLI share
  one implementation (01 sets this up).
- **Shared files.** Several tasks add a case to the subcommand switch in
  `cmd/asanamate/main.go`, a line to its `usage` text, and rows to the README
  Keys table. Expect small merge conflicts there; keep each addition to its
  own lines to make them easy to resolve.
- **Tests.** The packages you touch have tests. Add tests next to the existing
  ones, and run `go test ./...` and `go vet ./...` before you finish. CI also
  runs staticcheck and the race detector.
- **Docs.** Update the README section for every user-visible change, and the
  "What it does" list when a feature is added or removed.
- **Breaking changes** (06, 07) need the user's approval before you start.
  Follow "Approved breaking changes" in `AGENTS.md`.
- **Done** means the acceptance list in the spec passes. Then mark the task
  done in the table above.

## Open questions

- **Action builder or schema.** The action builder (`A`) guides users through
  action files. A JSON Schema for action files would give completion and docs
  in any TOML editor with less code to maintain. No decision yet; no spec.
